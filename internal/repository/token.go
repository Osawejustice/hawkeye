package repository

import (
	"context"
	"time"

	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TokenRepository struct {
	db *gorm.DB
}

func NewTokenRepository(db *gorm.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

func (r *TokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *TokenRepository) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("User.Organization").
		Where("token_hash = ?", hash).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *TokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.WithContext(ctx).First(&token, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *TokenRepository) Save(ctx context.Context, token *models.RefreshToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

func (r *TokenRepository) Revoke(ctx context.Context, token *models.RefreshToken, now time.Time) error {
	token.RevokedAt = &now
	return r.db.WithContext(ctx).
		Model(token).
		Update("revoked_at", now).
		Error
}

func (r *TokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).
		Error
}

// Rotate atomically revokes the current token and inserts its replacement.
func (r *TokenRepository) Rotate(ctx context.Context, current *models.RefreshToken, next *models.RefreshToken, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(next).Error; err != nil {
			return err
		}
		current.RevokedAt = &now
		current.ReplacedBy = &next.ID
		return tx.Model(current).Updates(map[string]any{
			"revoked_at":  now,
			"replaced_by": next.ID,
		}).Error
	})
}
