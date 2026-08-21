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

type CameraHandler struct {
	cameras *service.CameraService
}

func NewCameraHandler(cameras *service.CameraService) *CameraHandler {
	return &CameraHandler{cameras: cameras}
}

type createCameraRequest struct {
	Name             string `json:"name" binding:"required,min=1,max=120"`
	Description      string `json:"description"`
	Location         string `json:"location"`
	RTSPURL          string `json:"rtsp_url" binding:"required"`
	RTSPUsername     string `json:"rtsp_username"`
	RTSPPassword     string `json:"rtsp_password"`
	Enabled          *bool  `json:"enabled"`
	RecordingEnabled *bool  `json:"recording_enabled"`
}

type updateCameraRequest struct {
	Name             *string `json:"name"`
	Description      *string `json:"description"`
	Location         *string `json:"location"`
	RTSPURL          *string `json:"rtsp_url"`
	RTSPUsername     *string `json:"rtsp_username"`
	RTSPPassword     *string `json:"rtsp_password"`
	Enabled          *bool   `json:"enabled"`
	RecordingEnabled *bool   `json:"recording_enabled"`
}

func (h *CameraHandler) Create(c *gin.Context) {
	var req createCameraRequest
	if !BindJSON(c, &req) {
		return
	}
	cam, err := h.cameras.Create(c.Request.Context(), middleware.Actor(c), service.CreateCameraInput{
		Name:             req.Name,
		Description:      req.Description,
		Location:         req.Location,
		RTSPURL:          req.RTSPURL,
		RTSPUsername:     req.RTSPUsername,
		RTSPPassword:     req.RTSPPassword,
		Enabled:          req.Enabled,
		RecordingEnabled: req.RecordingEnabled,
	})
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusCreated, NewCameraResponse(cam, h.cameras.StreamURLs(cam)))
}

func (h *CameraHandler) List(c *gin.Context) {
	var enabled *bool
	if raw, ok := c.GetQuery("enabled"); ok {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			Error(c, apierr.ErrInvalidInput.WithMessage("enabled must be a boolean"))
			return
		}
		enabled = &v
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.cameras.List(c.Request.Context(), middleware.Actor(c), service.ListCamerasInput{
		Query:   c.Query("q"),
		Enabled: enabled,
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		Error(c, err)
		return
	}

	items := make([]CameraResponse, 0, len(result.Items))
	for i := range result.Items {
		cam := result.Items[i]
		items = append(items, NewCameraResponse(&cam, h.cameras.StreamURLs(&cam)))
	}

	JSON(c, http.StatusOK, gin.H{
		"items":    items,
		"page":     result.Page,
		"per_page": result.PerPage,
		"total":    result.Total,
	})
}

func (h *CameraHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cam, err := h.cameras.Get(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewCameraResponse(cam, h.cameras.StreamURLs(cam)))
}

func (h *CameraHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req updateCameraRequest
	if !BindJSON(c, &req) {
		return
	}
	cam, err := h.cameras.Update(c.Request.Context(), middleware.Actor(c), id, service.UpdateCameraInput{
		Name:             req.Name,
		Description:      req.Description,
		Location:         req.Location,
		RTSPURL:          req.RTSPURL,
		RTSPUsername:     req.RTSPUsername,
		RTSPPassword:     req.RTSPPassword,
		Enabled:          req.Enabled,
		RecordingEnabled: req.RecordingEnabled,
	})
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewCameraResponse(cam, h.cameras.StreamURLs(cam)))
}

func (h *CameraHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.cameras.Delete(c.Request.Context(), middleware.Actor(c), id); err != nil {
		Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CameraHandler) Sync(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cam, err := h.cameras.Sync(c.Request.Context(), middleware.Actor(c), id)
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewCameraResponse(cam, h.cameras.StreamURLs(cam)))
}

func (h *CameraHandler) Status(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cam, live, err := h.cameras.Status(c.Request.Context(), middleware.Actor(c), id)
	if err != nil && cam == nil {
		Error(c, err)
		return
	}
	resp := CameraStatusResponse{
		Camera: NewCameraResponse(cam, h.cameras.StreamURLs(cam)),
		Live:   live,
	}
	JSON(c, http.StatusOK, resp)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, apierr.ErrInvalidInput.WithMessage("invalid id"))
		return uuid.Nil, false
	}
	return id, true
}
