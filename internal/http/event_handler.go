package httpx

import (
	"net/http"
	"strconv"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/http/middleware"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EventHandler struct {
	events *service.EventService
}

func NewEventHandler(events *service.EventService) *EventHandler {
	return &EventHandler{events: events}
}

func (h *EventHandler) List(c *gin.Context) {
	var cameraID *uuid.UUID
	if raw := c.Query("camera_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			Error(c, apierr.ErrInvalidInput.WithMessage("invalid camera_id"))
			return
		}
		cameraID = &id
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	result, err := h.events.List(c.Request.Context(), middleware.Actor(c), service.ListEventsInput{
		CameraID: cameraID,
		Type:     c.Query("type"),
		Page:     page,
		PerPage:  perPage,
	})
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]EventResponse, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, NewEventResponse(&result.Items[i]))
	}
	JSON(c, http.StatusOK, gin.H{
		"items":    items,
		"page":     result.Page,
		"per_page": result.PerPage,
		"total":    result.Total,
	})
}
