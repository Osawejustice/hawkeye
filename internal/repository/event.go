package repository

import (
	"context"

	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EventListFilter struct {
	OrganizationID uuid.UUID
	CameraID       *uuid.UUID
	Type           string
	Offset         int
	Limit          int
}

type EventRepository struct {
	db *gorm.DB
}

func NewEventRepository(db *gorm.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Create(ctx context.Context, event *models.Event) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *EventRepository) List(ctx context.Context, f EventListFilter) ([]models.Event, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Event{}).Where("organization_id = ?", f.OrganizationID)
	if f.CameraID != nil {
		q = q.Where("camera_id = ?", *f.CameraID)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.Event
	err := q.Preload("Camera").
		Order("created_at DESC").
		Offset(f.Offset).
		Limit(f.Limit).
		Find(&items).Error
	return items, total, err
}
