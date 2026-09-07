package httpx

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/http/middleware"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RecordingHandler struct {
	recordings *service.RecordingService
}

func NewRecordingHandler(recordings *service.RecordingService) *RecordingHandler {
	return &RecordingHandler{recordings: recordings}
}

func (h *RecordingHandler) ListForCamera(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	h.list(c, &id)
}

func (h *RecordingHandler) List(c *gin.Context) {
	var cameraID *uuid.UUID
	if raw := c.Query("camera_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			Error(c, apierr.ErrInvalidInput.WithMessage("invalid camera_id"))
			return
		}
		cameraID = &id
	}
	h.list(c, cameraID)
}

func (h *RecordingHandler) list(c *gin.Context, cameraID *uuid.UUID) {
	from, err := parseOptionalTime(c.Query("from"))
	if err != nil {
		Error(c, apierr.ErrInvalidInput.WithMessage("from must be RFC3339"))
		return
	}
	to, err := parseOptionalTime(c.Query("to"))
	if err != nil {
		Error(c, apierr.ErrInvalidInput.WithMessage("to must be RFC3339"))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.recordings.List(c.Request.Context(), middleware.Actor(c), service.ListRecordingsInput{
		CameraID: cameraID,
		From:     from,
		To:       to,
		Page:     page,
		PerPage:  perPage,
	})
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]RecordingResponse, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, NewRecordingResponse(&result.Items[i]))
	}
	JSON(c, http.StatusOK, gin.H{
		"items":    items,
		"page":     result.Page,
		"per_page": result.PerPage,
		"total":    result.Total,
	})
}

func (h *RecordingHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	seg, err := h.recordings.Get(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewRecordingResponse(seg))
}

func (h *RecordingHandler) Video(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	obj, err := h.recordings.OpenPlayback(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		Error(c, err)
		return
	}
	defer obj.Body.Close()

	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Disposition", "inline")
	c.Header("Cache-Control", "private, max-age=60")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, obj.Body)
}

func parseOptionalTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, err
		}
	}
	utc := t.UTC()
	return &utc, nil
}
