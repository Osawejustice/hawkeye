package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/google/uuid"
)

type ListEventsInput struct {
	CameraID *uuid.UUID
	Type     string
	Page     int
	PerPage  int
}

type EventListResult struct {
	Items   []models.Event
	Total   int64
	Page    int
	PerPage int
}

type EventService struct {
	events *repository.EventRepository
	log    *slog.Logger
}

func NewEventService(events *repository.EventRepository, log *slog.Logger) *EventService {
	return &EventService{events: events, log: log}
}

func (s *EventService) List(ctx context.Context, actor Actor, in ListEventsInput) (*EventListResult, error) {
	page, per := normalizePage(in.Page, in.PerPage)
	items, total, err := s.events.List(ctx, repository.EventListFilter{
		OrganizationID: actor.OrgID,
		CameraID:       in.CameraID,
		Type:           in.Type,
		Offset:         (page - 1) * per,
		Limit:          per,
	})
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	return &EventListResult{Items: items, Total: total, Page: page, PerPage: per}, nil
}

func (s *EventService) Emit(ctx context.Context, orgID uuid.UUID, cameraID *uuid.UUID, typ, message string, meta any) {
	raw := json.RawMessage(`{}`)
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			raw = b
		}
	}
	ev := &models.Event{
		OrganizationID: orgID,
		CameraID:       cameraID,
		Type:           typ,
		Message:        message,
		Metadata:       raw,
	}
	if err := s.events.Create(ctx, ev); err != nil {
		s.log.Warn("failed to persist event", "type", typ, "err", err)
	}
}
