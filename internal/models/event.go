package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	EventCameraOnline     = "camera.online"
	EventCameraOffline    = "camera.offline"
	EventCameraSyncError  = "camera.sync_error"
	EventRecordingSegment = "recording.segment"
)

// Event is an organization-scoped activity record.
type Event struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID       `gorm:"type:uuid;not null;index"`
	CameraID       *uuid.UUID      `gorm:"type:uuid;index"`
	Type           string          `gorm:"size:64;not null"`
	Message        string          `gorm:"type:text;not null"`
	Metadata       json.RawMessage `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time       `gorm:"not null"`

	Camera *Camera `gorm:"foreignKey:CameraID"`
}

func (Event) TableName() string { return "events" }

func (e *Event) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if len(e.Metadata) == 0 {
		e.Metadata = json.RawMessage(`{}`)
	}
	return nil
}
