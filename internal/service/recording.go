package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/cohi-hq/cohi-api/internal/storage"
	"github.com/google/uuid"
)

type ListRecordingsInput struct {
	CameraID *uuid.UUID
	From     *time.Time
	To       *time.Time
	Page     int
	PerPage  int
}

type RecordingListResult struct {
	Items   []models.RecordingSegment
	Total   int64
	Page    int
	PerPage int
}

type UpsertRecordingInput struct {
	CameraID  uuid.UUID
	StartedAt time.Time
	Duration  time.Duration
	Trigger   string
	Format    string
	SizeBytes *int64
}

type UpsertRecordingResult struct {
	Segment *models.RecordingSegment
	Created bool
}

type RecordingService struct {
	recordings *repository.RecordingRepository
	cameras    *repository.CameraRepository
	events     *EventService
	store      storage.Backend
	log        *slog.Logger
}

func NewRecordingService(
	recordings *repository.RecordingRepository,
	cameras *repository.CameraRepository,
	events *EventService,
	store storage.Backend,
	log *slog.Logger,
) *RecordingService {
	return &RecordingService{
		recordings: recordings,
		cameras:    cameras,
		events:     events,
		store:      store,
		log:        log,
	}
}

func (s *RecordingService) List(ctx context.Context, actor Actor, in ListRecordingsInput) (*RecordingListResult, error) {
	if in.CameraID != nil {
		if _, err := s.loadOwnedCamera(ctx, actor, *in.CameraID); err != nil {
			return nil, err
		}
	}
	page, per := normalizePage(in.Page, in.PerPage)
	items, total, err := s.recordings.List(ctx, repository.RecordingListFilter{
		OrganizationID: actor.OrgID,
		CameraID:       in.CameraID,
		From:           in.From,
		To:             in.To,
		Offset:         (page - 1) * per,
		Limit:          per,
	})
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	return &RecordingListResult{Items: items, Total: total, Page: page, PerPage: per}, nil
}

func (s *RecordingService) Get(ctx context.Context, actor Actor, id uuid.UUID) (*models.RecordingSegment, error) {
	seg, err := s.recordings.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrNotFound.WithMessage("recording not found")
		}
		return nil, apierr.ErrInternal.With(err)
	}
	if seg.OrganizationID != actor.OrgID {
		return nil, apierr.ErrNotFound.WithMessage("recording not found")
	}
	return seg, nil
}

func (s *RecordingService) OpenPlayback(ctx context.Context, actor Actor, id uuid.UUID) (*storage.Object, error) {
	seg, err := s.Get(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, apierr.ErrUnavailable.WithMessage("playback is not configured")
	}
	dur := segmentDuration(seg)
	obj, err := s.store.OpenPlayback(ctx, seg.MTXPath, seg.StartedAt, dur)
	if err != nil {
		s.log.Warn("playback open failed", "recording_id", seg.ID, "err", err)
		return nil, apierr.ErrBadGateway.WithMessage("could not open recording playback")
	}
	return obj, nil
}

func (s *RecordingService) UpsertFromWorker(ctx context.Context, in UpsertRecordingInput) (*UpsertRecordingResult, error) {
	if in.CameraID == uuid.Nil || in.StartedAt.IsZero() {
		return nil, apierr.ErrInvalidInput.WithMessage("camera_id and started_at are required")
	}
	camera, err := s.cameras.GetByID(ctx, in.CameraID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrNotFound.WithMessage("camera not found")
		}
		return nil, apierr.ErrInternal.With(err)
	}

	started := in.StartedAt.UTC()
	dur := in.Duration
	if dur < 0 {
		dur = 0
	}
	ended := started.Add(dur)
	ms := dur.Milliseconds()
	trigger := in.Trigger
	if trigger == "" {
		trigger = models.TriggerContinuous
	}
	format := in.Format
	if format == "" {
		format = models.FormatFMP4
	}

	existing, err := s.recordings.GetByCameraStart(ctx, camera.ID, started)
	if err != nil && !repository.IsNotFound(err) {
		return nil, apierr.ErrInternal.With(err)
	}
	if existing != nil {
		existing.EndedAt = &ended
		existing.DurationMS = &ms
		existing.SizeBytes = in.SizeBytes
		existing.Trigger = trigger
		existing.Format = format
		existing.MTXPath = camera.MTXPath
		existing.StorageBackend = models.StorageMTX
		existing.StoragePath = storage.MTXKey(camera.MTXPath, started)
		if err := s.recordings.Save(ctx, existing); err != nil {
			return nil, apierr.ErrInternal.With(err)
		}
		return &UpsertRecordingResult{Segment: existing, Created: false}, nil
	}

	seg := &models.RecordingSegment{
		OrganizationID: camera.OrganizationID,
		CameraID:       camera.ID,
		StartedAt:      started,
		EndedAt:        &ended,
		DurationMS:     &ms,
		StorageBackend: models.StorageMTX,
		StoragePath:    storage.MTXKey(camera.MTXPath, started),
		SizeBytes:      in.SizeBytes,
		Format:         format,
		Trigger:        trigger,
		MTXPath:        camera.MTXPath,
	}
	if err := s.recordings.Create(ctx, seg); err != nil {
		if isUniqueViolation(err) {
			existing, err := s.recordings.GetByCameraStart(ctx, camera.ID, started)
			if err != nil {
				return nil, apierr.ErrInternal.With(err)
			}
			return &UpsertRecordingResult{Segment: existing, Created: false}, nil
		}
		return nil, apierr.ErrInternal.With(err)
	}

	camID := camera.ID
	s.events.Emit(ctx, camera.OrganizationID, &camID, models.EventRecordingSegment,
		"New recording segment for "+camera.Name,
		map[string]any{
			"recording_id": seg.ID,
			"started_at":   started,
			"duration_ms":  ms,
		})

	return &UpsertRecordingResult{Segment: seg, Created: true}, nil
}

func (s *RecordingService) loadOwnedCamera(ctx context.Context, actor Actor, id uuid.UUID) (*models.Camera, error) {
	camera, err := s.cameras.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrNotFound.WithMessage("camera not found")
		}
		return nil, apierr.ErrInternal.With(err)
	}
	if camera.OrganizationID != actor.OrgID {
		return nil, apierr.ErrNotFound.WithMessage("camera not found")
	}
	return camera, nil
}

func segmentDuration(seg *models.RecordingSegment) time.Duration {
	if seg.DurationMS != nil && *seg.DurationMS > 0 {
		return time.Duration(*seg.DurationMS) * time.Millisecond
	}
	if seg.EndedAt != nil && seg.EndedAt.After(seg.StartedAt) {
		return seg.EndedAt.Sub(seg.StartedAt)
	}
	d := time.Since(seg.StartedAt)
	if d < time.Second {
		return time.Second
	}
	return d
}
