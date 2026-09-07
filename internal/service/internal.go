package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/google/uuid"
)

type HeartbeatInput struct {
	Online bool
	SeenAt *time.Time
}

type InternalService struct {
	cameras *repository.CameraRepository
	events  *EventService
	log     *slog.Logger
}

func NewInternalService(cameras *repository.CameraRepository, events *EventService, log *slog.Logger) *InternalService {
	return &InternalService{cameras: cameras, events: events, log: log}
}

func (s *InternalService) ListCameras(ctx context.Context) ([]models.Camera, error) {
	items, err := s.cameras.ListAll(ctx)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	return items, nil
}

func (s *InternalService) Heartbeat(ctx context.Context, cameraID uuid.UUID, in HeartbeatInput) (*models.Camera, error) {
	camera, err := s.cameras.GetByID(ctx, cameraID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrNotFound.WithMessage("camera not found")
		}
		return nil, apierr.ErrInternal.With(err)
	}

	seen := in.SeenAt
	if in.Online {
		now := time.Now().UTC()
		if seen == nil {
			seen = &now
		}
	}

	changed := camera.IsOnline != in.Online
	if err := s.cameras.UpdateHeartbeat(ctx, camera.ID, in.Online, seen); err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	camera.IsOnline = in.Online
	if seen != nil {
		camera.LastSeenAt = seen
	}

	if changed {
		camID := camera.ID
		if in.Online {
			s.events.Emit(ctx, camera.OrganizationID, &camID, models.EventCameraOnline,
				camera.Name+" is online", nil)
		} else {
			s.events.Emit(ctx, camera.OrganizationID, &camID, models.EventCameraOffline,
				camera.Name+" is offline", nil)
		}
	}
	return camera, nil
}
