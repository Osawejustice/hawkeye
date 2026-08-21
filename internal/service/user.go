package service

import (
	"context"
	"strings"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/auth"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/google/uuid"
)

type UpdateProfileInput struct {
	Name            *string
	Password        *string
	CurrentPassword *string
}

type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrNotFound.WithMessage("user not found")
		}
		return nil, apierr.ErrInternal.With(err)
	}
	if !user.IsActive {
		return nil, apierr.ErrInactiveUser
	}
	return user, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*models.User, error) {
	user, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		name, err := normalizeName(*in.Name)
		if err != nil {
			return nil, err
		}
		user.Name = name
	}

	if in.Password != nil {
		if in.CurrentPassword == nil || strings.TrimSpace(*in.CurrentPassword) == "" {
			return nil, apierr.ErrInvalidInput.WithMessage("current_password is required to change password")
		}
		if err := auth.CheckPassword(user.PasswordHash, *in.CurrentPassword); err != nil {
			return nil, apierr.ErrInvalidCredentials.WithMessage("current password is incorrect")
		}
		if err := validatePassword(*in.Password); err != nil {
			return nil, err
		}
		hash, err := auth.HashPassword(*in.Password)
		if err != nil {
			return nil, apierr.ErrInternal.With(err)
		}
		user.PasswordHash = hash
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	return user, nil
}
