package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User is an authenticated operator of the control plane.
type User struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index"`
	Email          string         `gorm:"size:255;not null"`
	PasswordHash   string         `gorm:"size:255;not null"`
	Name           string         `gorm:"size:255;not null"`
	Role           string         `gorm:"size:32;not null;default:admin"`
	IsActive       bool           `gorm:"not null;default:true"`
	LastLoginAt    *time.Time     `gorm:"column:last_login_at"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	Organization *Organization `gorm:"foreignKey:OrganizationID"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.Role == "" {
		u.Role = RoleAdmin
	}
	return nil
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
