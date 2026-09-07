package repository

import (
	"context"
	"strings"

	"time"

	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CameraListFilter struct {
	OrganizationID uuid.UUID
	Query          string
	Enabled        *bool
	Offset         int
	Limit          int
}

type CameraRepository struct {
	db *gorm.DB
}

func NewCameraRepository(db *gorm.DB) *CameraRepository {
	return &CameraRepository{db: db}
}

func (r *CameraRepository) Create(ctx context.Context, camera *models.Camera) error {
	return r.db.WithContext(ctx).Create(camera).Error
}

func (r *CameraRepository) Save(ctx context.Context, camera *models.Camera) error {
	return r.db.WithContext(ctx).Save(camera).Error
}

func (r *CameraRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Camera, error) {
	var camera models.Camera
	err := r.db.WithContext(ctx).
		Preload("Owner").
		First(&camera, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &camera, nil
}

func (r *CameraRepository) List(ctx context.Context, f CameraListFilter) ([]models.Camera, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Camera{}).Where("organization_id = ?", f.OrganizationID)

	if trimmed := strings.TrimSpace(f.Query); trimmed != "" {
		like := "%" + strings.ToLower(trimmed) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(location) LIKE ?", like, like)
	}
	if f.Enabled != nil {
		q = q.Where("enabled = ?", *f.Enabled)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var cameras []models.Camera
	err := q.Preload("Owner").
		Order("created_at DESC").
		Offset(f.Offset).
		Limit(f.Limit).
		Find(&cameras).Error
	return cameras, total, err
}

func (r *CameraRepository) Delete(ctx context.Context, camera *models.Camera) error {
	return r.db.WithContext(ctx).Delete(camera).Error
}

func (r *CameraRepository) NameTaken(ctx context.Context, orgID uuid.UUID, name string, excludeID *uuid.UUID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&models.Camera{}).
		Where("organization_id = ? AND LOWER(name) = ?", orgID, strings.ToLower(strings.TrimSpace(name)))
	if excludeID != nil {
		q = q.Where("id <> ?", *excludeID)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *CameraRepository) UpdateSync(ctx context.Context, id uuid.UUID, status, syncErr string) error {
	return r.db.WithContext(ctx).
		Model(&models.Camera{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"mtx_sync_status": status,
			"mtx_sync_error":  syncErr,
		}).Error
}

func (r *CameraRepository) ListAll(ctx context.Context) ([]models.Camera, error) {
	var cameras []models.Camera
	err := r.db.WithContext(ctx).
		Select("id", "organization_id", "name", "mtx_path", "enabled", "recording_enabled", "is_online", "last_seen_at", "mtx_sync_status").
		Order("created_at ASC").
		Find(&cameras).Error
	return cameras, err
}

func (r *CameraRepository) ListForSync(ctx context.Context) ([]models.Camera, error) {
	var cameras []models.Camera
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&cameras).Error
	return cameras, err
}

func (r *CameraRepository) GetByMTXPath(ctx context.Context, path string) (*models.Camera, error) {
	var camera models.Camera
	err := r.db.WithContext(ctx).First(&camera, "mtx_path = ?", path).Error
	if err != nil {
		return nil, err
	}
	return &camera, nil
}

func (r *CameraRepository) UpdateHeartbeat(ctx context.Context, id uuid.UUID, online bool, seenAt *time.Time) error {
	updates := map[string]any{
		"is_online":  online,
		"updated_at": time.Now().UTC(),
	}
	if seenAt != nil {
		updates["last_seen_at"] = seenAt
	}
	return r.db.WithContext(ctx).
		Model(&models.Camera{}).
		Where("id = ?", id).
		Updates(updates).Error
}
