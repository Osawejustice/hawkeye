package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/cohi-hq/cohi-api/internal/database"
	"github.com/cohi-hq/cohi-api/internal/media/mediamtx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db      *gorm.DB
	mtx     mediamtx.Client
	started time.Time
	version string
}

func NewHealthHandler(db *gorm.DB, mtx mediamtx.Client, version string) *HealthHandler {
	return &HealthHandler{db: db, mtx: mtx, started: time.Now().UTC(), version: version}
}

func (h *HealthHandler) Live(c *gin.Context) {
	JSON(c, http.StatusOK, gin.H{
		"status":  "ok",
		"service": "cohi-api",
		"version": h.version,
		"uptime":  time.Since(h.started).String(),
		"time":    time.Now().UTC(),
	})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := gin.H{}
	ready := true

	if err := database.Ping(ctx, h.db); err != nil {
		ready = false
		checks["postgres"] = gin.H{"ok": false, "error": err.Error()}
	} else {
		checks["postgres"] = gin.H{"ok": true}
	}

	if h.mtx.Enabled() {
		if err := h.mtx.Health(ctx); err != nil {
			checks["mediamtx"] = gin.H{"ok": false, "error": err.Error()}
		} else {
			checks["mediamtx"] = gin.H{"ok": true}
		}
	} else {
		checks["mediamtx"] = gin.H{"ok": true, "enabled": false}
	}

	status := http.StatusOK
	state := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "degraded"
	}
	JSON(c, status, gin.H{
		"status":  state,
		"service": "cohi-api",
		"checks":  checks,
	})
}
