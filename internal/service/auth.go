package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/auth"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt limit
	maxNameLen     = 120
)

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

type LoginInput struct {
	Email    string
	Password string
}

type SessionMeta struct {
	UserAgent string
	IPAddress string
}

type AuthResult struct {
	User         *models.User
	AccessToken  string
	RefreshToken string
	AccessExp    time.Time
	RefreshExp   time.Time
}

type AuthService struct {
	db         *gorm.DB
	users      *repository.UserRepository
	tokens     *repository.TokenRepository
	jwt        *auth.JWTManager
	refreshTTL time.Duration
	pepper     string
	log        *slog.Logger
}

func NewAuthService(
	db *gorm.DB,
	users *repository.UserRepository,
	tokens *repository.TokenRepository,
	jwtMgr *auth.JWTManager,
	refreshTTL time.Duration,
	pepper string,
	log *slog.Logger,
) *AuthService {
	return &AuthService{
		db:         db,
		users:      users,
		tokens:     tokens,
		jwt:        jwtMgr,
		refreshTTL: refreshTTL,
		pepper:     pepper,
		log:        log,
	}
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput, meta SessionMeta) (*AuthResult, error) {
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}

	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return nil, apierr.ErrConflict.WithMessage("an account with this email already exists")
	} else if !repository.IsNotFound(err) {
		return nil, apierr.ErrInternal.With(err)
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}

	org := &models.Organization{
		Name: name + "'s Organization",
		Slug: "",
	}

	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Role:         models.RoleAdmin,
		IsActive:     true,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		orgRepo := repository.NewOrganizationRepository(tx)
		userRepo := repository.NewUserRepository(tx)

		slug, err := uniqueSlug(ctx, orgRepo, name)
		if err != nil {
			return err
		}
		org.Slug = slug
		if err := orgRepo.Create(ctx, org); err != nil {
			return err
		}
		user.OrganizationID = org.ID
		return userRepo.Create(ctx, user)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apierr.ErrConflict.WithMessage("an account with this email already exists")
		}
		return nil, apierr.ErrInternal.With(err)
	}

	user.Organization = org
	s.log.Info("user registered", "user_id", user.ID, "org_id", org.ID)

	return s.issueSession(ctx, user, meta)
}

func (s *AuthService) Login(ctx context.Context, in LoginInput, meta SessionMeta) (*AuthResult, error) {
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return nil, apierr.ErrInvalidCredentials
	}
	if in.Password == "" {
		return nil, apierr.ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrInvalidCredentials
		}
		return nil, apierr.ErrInternal.With(err)
	}
	if !user.IsActive {
		return nil, apierr.ErrInactiveUser
	}
	if err := auth.CheckPassword(user.PasswordHash, in.Password); err != nil {
		return nil, apierr.ErrInvalidCredentials
	}

	now := time.Now().UTC()
	_ = s.users.TouchLastLogin(ctx, user.ID, now)
	user.LastLoginAt = &now

	s.log.Info("user logged in", "user_id", user.ID)
	return s.issueSession(ctx, user, meta)
}

func (s *AuthService) Refresh(ctx context.Context, rawToken string, meta SessionMeta) (*AuthResult, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, apierr.ErrTokenInvalid.WithMessage("refresh token is required")
	}

	stored, err := s.tokens.GetByHash(ctx, auth.HashRefreshToken(rawToken, s.pepper))
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrTokenInvalid
		}
		return nil, apierr.ErrInternal.With(err)
	}

	now := time.Now().UTC()
	if stored.IsRevoked() {
		// Reuse of a rotated/revoked token: revoke the whole family.
		if err := s.tokens.RevokeAllForUser(ctx, stored.UserID, now); err != nil {
			s.log.Error("failed to revoke token family after reuse", "err", err, "user_id", stored.UserID)
		}
		s.log.Warn("refresh token reuse detected", "user_id", stored.UserID, "token_id", stored.ID)
		return nil, apierr.ErrTokenReused
	}
	if stored.IsExpired(now) {
		return nil, apierr.ErrTokenExpired
	}

	user := stored.User
	if user == nil {
		user, err = s.users.GetByID(ctx, stored.UserID)
		if err != nil {
			return nil, apierr.ErrInternal.With(err)
		}
	}
	if !user.IsActive {
		return nil, apierr.ErrInactiveUser
	}

	access, accessExp, err := s.jwt.IssueAccess(user.ID.String(), user.OrganizationID.String(), user.Email, user.Role)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}

	raw, hash, err := auth.NewRefreshToken(s.pepper)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	next := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		UserAgent: meta.UserAgent,
		IPAddress: meta.IPAddress,
		ExpiresAt: now.Add(s.refreshTTL),
	}
	if err := s.tokens.Rotate(ctx, stored, next, now); err != nil {
		return nil, apierr.ErrInternal.With(err)
	}

	return &AuthResult{
		User:         user,
		AccessToken:  access,
		RefreshToken: raw,
		AccessExp:    accessExp,
		RefreshExp:   next.ExpiresAt,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return apierr.ErrInvalidInput.WithMessage("refresh token is required")
	}
	stored, err := s.tokens.GetByHash(ctx, auth.HashRefreshToken(rawToken, s.pepper))
	if err != nil {
		if repository.IsNotFound(err) {
			return nil
		}
		return apierr.ErrInternal.With(err)
	}
	if stored.IsRevoked() {
		return nil
	}
	return s.tokens.Revoke(ctx, stored, time.Now().UTC())
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.tokens.RevokeAllForUser(ctx, userID, time.Now().UTC())
}

func (s *AuthService) ParseAccess(token string) (*auth.AccessClaims, error) {
	claims, err := s.jwt.ParseAccess(token)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "expired") {
			return nil, apierr.ErrTokenExpired
		}
		return nil, apierr.ErrTokenInvalid
	}
	return claims, nil
}

func (s *AuthService) issueSession(ctx context.Context, user *models.User, meta SessionMeta) (*AuthResult, error) {
	access, accessExp, err := s.jwt.IssueAccess(user.ID.String(), user.OrganizationID.String(), user.Email, user.Role)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	raw, hash, err := auth.NewRefreshToken(s.pepper)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}

	now := time.Now().UTC()
	rt := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		UserAgent: meta.UserAgent,
		IPAddress: meta.IPAddress,
		ExpiresAt: now.Add(s.refreshTTL),
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, apierr.ErrInternal.With(err)
	}

	return &AuthResult{
		User:         user,
		AccessToken:  access,
		RefreshToken: raw,
		AccessExp:    accessExp,
		RefreshExp:   rt.ExpiresAt,
	}, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", apierr.ErrInvalidInput.WithMessage("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", apierr.ErrInvalidInput.WithMessage("email is invalid")
	}
	return email, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apierr.ErrInvalidInput.WithMessage("name is required")
	}
	if len(name) > maxNameLen {
		return "", apierr.ErrInvalidInput.WithMessage("name is too long")
	}
	return name, nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return apierr.ErrInvalidInput.WithMessage(fmt.Sprintf("password must be at least %d characters", minPasswordLen))
	}
	if len(password) > maxPasswordLen {
		return apierr.ErrInvalidInput.WithMessage(fmt.Sprintf("password must be at most %d characters", maxPasswordLen))
	}
	return nil
}

func uniqueSlug(ctx context.Context, orgs *repository.OrganizationRepository, name string) (string, error) {
	base := slugify(name)
	if base == "" {
		base = "org"
	}
	for i := 0; i < 8; i++ {
		suffix, err := randomHex(3)
		if err != nil {
			return "", err
		}
		candidate := base + "-" + suffix
		exists, err := orgs.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique organization slug")
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
		if b.Len() >= 40 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
