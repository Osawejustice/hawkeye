package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshToken stores a hashed, rotatable refresh token.
// The raw token is never persisted.
type RefreshToken struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	TokenHash  string     `gorm:"size:64;not null;uniqueIndex"`
	UserAgent  string     `gorm:"type:text"`
	IPAddress  string     `gorm:"size:64"`
	ExpiresAt  time.Time  `gorm:"not null;index"`
	RevokedAt  *time.Time `gorm:"index"`
	ReplacedBy *uuid.UUID `gorm:"type:uuid"`
	CreatedAt  time.Time  `gorm:"not null"`

	User *User `gorm:"foreignKey:UserID"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

func (t *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

func (t *RefreshToken) IsExpired(now time.Time) bool {
	return !t.ExpiresAt.After(now)
}

func (t *RefreshToken) IsUsable(now time.Time) bool {
	return !t.IsRevoked() && !t.IsExpired(now)
}
