package httpx

import (
	"net/http"
	"time"

	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InternalHandler struct {
	internal   *service.InternalService
	recordings *service.RecordingService
	cameras    *service.CameraService
}

func NewInternalHandler(internal *service.InternalService, recordings *service.RecordingService, cameras *service.CameraService) *InternalHandler {
	return &InternalHandler{internal: internal, recordings: recordings, cameras: cameras}
}

func (h *InternalHandler) ListCameras(c *gin.Context) {
	items, err := h.internal.ListCameras(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	out := make([]InternalCameraResponse, 0, len(items))
	for i := range items {
		out = append(out, NewInternalCameraResponse(&items[i]))
	}
	JSON(c, http.StatusOK, gin.H{"items": out, "total": len(out)})
}

type heartbeatRequest struct {
	Online bool       `json:"online"`
	SeenAt *time.Time `json:"seen_at"`
}

func (h *InternalHandler) SyncCamera(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cam, err := h.cameras.SyncByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewInternalCameraResponse(cam))
}

func (h *InternalHandler) Heartbeat(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req heartbeatRequest
	if !BindJSON(c, &req) {
		return
	}
	cam, err := h.internal.Heartbeat(c.Request.Context(), id, service.HeartbeatInput{
		Online: req.Online,
		SeenAt: req.SeenAt,
	})
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewInternalCameraResponse(cam))
}

type upsertRecordingRequest struct {
	CameraID    uuid.UUID  `json:"camera_id" binding:"required"`
	StartedAt   time.Time  `json:"started_at" binding:"required"`
	DurationMS  *int64     `json:"duration_ms"`
	Trigger     string     `json:"trigger"`
	Format      string     `json:"format"`
	SizeBytes   *int64     `json:"size_bytes"`
}

func (h *InternalHandler) UpsertRecording(c *gin.Context) {
	var req upsertRecordingRequest
	if !BindJSON(c, &req) {
		return
	}
	var dur time.Duration
	if req.DurationMS != nil && *req.DurationMS > 0 {
		dur = time.Duration(*req.DurationMS) * time.Millisecond
	}
	res, err := h.recordings.UpsertFromWorker(c.Request.Context(), service.UpsertRecordingInput{
		CameraID:  req.CameraID,
		StartedAt: req.StartedAt,
		Duration:  dur,
		Trigger:   req.Trigger,
		Format:    req.Format,
		SizeBytes: req.SizeBytes,
	})
	if err != nil {
		Error(c, err)
		return
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	JSON(c, status, gin.H{
		"created":   res.Created,
		"recording": NewRecordingResponse(res.Segment),
	})
}
