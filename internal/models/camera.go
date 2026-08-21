package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	MTXSyncPending = "pending"
	MTXSyncSynced  = "synced"
	MTXSyncError   = "error"
	MTXSyncSkipped = "skipped"
)

// Camera is a physical or virtual RTSP source owned by an organization.
type Camera struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrganizationID   uuid.UUID      `gorm:"type:uuid;not null;index"`
	OwnerID          uuid.UUID      `gorm:"type:uuid;not null;index"`
	Name             string         `gorm:"size:255;not null"`
	Description      string         `gorm:"type:text"`
	Location         string         `gorm:"size:255"`
	RTSPURL          string         `gorm:"column:rtsp_url;type:text;not null"`
	RTSPUsername     string         `gorm:"column:rtsp_username;size:255"`
	RTSPPassword     string         `gorm:"column:rtsp_password;type:text"`
	Enabled          bool           `gorm:"not null;default:true"`
	RecordingEnabled bool           `gorm:"not null;default:false"`
	MTXPath          string         `gorm:"column:mtx_path;size:255;not null"`
	MTXSyncStatus    string         `gorm:"column:mtx_sync_status;size:32;not null;default:pending"`
	MTXSyncError     string         `gorm:"column:mtx_sync_error;type:text"`
	LastSeenAt       *time.Time     `gorm:"column:last_seen_at"`
	CreatedAt        time.Time      `gorm:"not null"`
	UpdatedAt        time.Time      `gorm:"not null"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`

	Owner        *User         `gorm:"foreignKey:OwnerID"`
	Organization *Organization `gorm:"foreignKey:OrganizationID"`
}

func (Camera) TableName() string { return "cameras" }

func (c *Camera) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.MTXPath == "" {
		c.MTXPath = MTXPathFor(c.ID)
	}
	if c.MTXSyncStatus == "" {
		c.MTXSyncStatus = MTXSyncPending
	}
	return nil
}

// MTXPathFor returns the MediaMTX path name owned by this camera.
func MTXPathFor(id uuid.UUID) string {
	return "cam-" + id.String()
}

func (c *Camera) HasRTSPPassword() bool {
	return c.RTSPPassword != ""
}
