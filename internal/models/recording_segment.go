package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StorageLocal = "local"
	StorageMTX   = "mediamtx"
	StorageR2    = "r2"
	FormatFMP4   = "fmp4"
	FormatMP4    = "mp4"
	FormatMPEGTS = "mpegts"

	TriggerContinuous = "continuous"
	TriggerMotion     = "motion"
	TriggerManual     = "manual"
)

// RecordingSegment is control-plane metadata for a recorded clip.
// The bytes live in the storage layer (MediaMTX playback now, R2 later).
type RecordingSegment struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index"`
	CameraID       uuid.UUID      `gorm:"type:uuid;not null;index"`
	StartedAt      time.Time      `gorm:"not null"`
	EndedAt        *time.Time     `gorm:"column:ended_at"`
	DurationMS     *int64         `gorm:"column:duration_ms"`
	StorageBackend string         `gorm:"size:32;not null;default:mediamtx"`
	StoragePath    string         `gorm:"type:text;not null"`
	SizeBytes      *int64         `gorm:"column:size_bytes"`
	Format         string         `gorm:"size:32;not null;default:fmp4"`
	Trigger        string         `gorm:"size:32;not null;default:continuous"`
	MTXPath        string         `gorm:"column:mtx_path;size:255;not null"`
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
		r.StorageBackend = StorageMTX
	}
	if r.Format == "" {
		r.Format = FormatFMP4
	}
	if r.Trigger == "" {
		r.Trigger = TriggerContinuous
	}
	return nil
}
