package repository

import (
	"context"
	"time"

	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecordingListFilter struct {
	OrganizationID uuid.UUID
	CameraID       *uuid.UUID
	From           *time.Time
	To             *time.Time
	Offset         int
	Limit          int
}

type RecordingRepository struct {
	db *gorm.DB
}

func NewRecordingRepository(db *gorm.DB) *RecordingRepository {
	return &RecordingRepository{db: db}
}

func (r *RecordingRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RecordingSegment, error) {
	var seg models.RecordingSegment
	err := r.db.WithContext(ctx).Preload("Camera").First(&seg, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &seg, nil
}

func (r *RecordingRepository) GetByCameraStart(ctx context.Context, cameraID uuid.UUID, startedAt time.Time) (*models.RecordingSegment, error) {
	var seg models.RecordingSegment
	err := r.db.WithContext(ctx).
		Where("camera_id = ? AND started_at = ?", cameraID, startedAt.UTC()).
		First(&seg).Error
	if err != nil {
		return nil, err
	}
	return &seg, nil
}

func (r *RecordingRepository) Create(ctx context.Context, seg *models.RecordingSegment) error {
	return r.db.WithContext(ctx).Create(seg).Error
}

func (r *RecordingRepository) Save(ctx context.Context, seg *models.RecordingSegment) error {
	return r.db.WithContext(ctx).Save(seg).Error
}

func (r *RecordingRepository) List(ctx context.Context, f RecordingListFilter) ([]models.RecordingSegment, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.RecordingSegment{}).Where("organization_id = ?", f.OrganizationID)
	if f.CameraID != nil {
		q = q.Where("camera_id = ?", *f.CameraID)
	}
	if f.From != nil {
		q = q.Where("started_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("started_at <= ?", *f.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.RecordingSegment
	err := q.Preload("Camera").
		Order("started_at DESC").
		Offset(f.Offset).
		Limit(f.Limit).
		Find(&items).Error
	return items, total, err
}
