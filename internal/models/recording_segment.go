package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StorageLocal = "local"
	StorageR2    = "r2"
	FormatFMP4   = "fmp4"
	FormatMPEGTS = "mpegts"
)

// RecordingSegment is control-plane metadata for a recorded clip.
// The bytes live in the storage layer (local FS now, R2 later).
type RecordingSegment struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index"`
	CameraID       uuid.UUID      `gorm:"type:uuid;not null;index"`
	StartedAt      time.Time      `gorm:"not null"`
	EndedAt        *time.Time     `gorm:"column:ended_at"`
	DurationMS     *int64         `gorm:"column:duration_ms"`
	StorageBackend string         `gorm:"size:32;not null;default:local"`
	StoragePath    string         `gorm:"type:text;not null"`
	SizeBytes      *int64         `gorm:"column:size_bytes"`
	Format         string         `gorm:"size:32;not null;default:fmp4"`
	CreatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	Camera *Camera `gorm:"foreignKey:CameraID"`
}

func (RecordingSegment) TableName() string { return "recording_segments" }

func (r *RecordingSegment) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.StorageBackend == "" {
		r.StorageBackend = StorageLocal
	}
	if r.Format == "" {
		r.Format = FormatFMP4
	}
	return nil
}
