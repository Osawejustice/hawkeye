package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/cohi-hq/cohi-api/internal/media/mediamtx"
)

type Worker struct {
	cfg *Config
	log *slog.Logger
	api *apiClient
	mtx mediamtx.Client
}

func New(cfg *Config, log *slog.Logger) (*Worker, error) {
	return &Worker{
		cfg: cfg,
		log: log,
		api: newAPIClient(cfg.APIURL, cfg.ServiceToken, 12*time.Second),
		mtx: mediamtx.New(cfg.MediaMTX),
	}, nil
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			w.log.Info("worker stopping")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	tickCtx, cancel := context.WithTimeout(ctx, w.cfg.PollInterval)
	defer cancel()

	cameras, err := w.api.ListCameras(tickCtx)
	if err != nil {
		w.log.Warn("list cameras failed", "err", err)
		return
	}

	live := map[string]mediamtx.PathStatus{}
	listed := false
	if w.mtx.Enabled() {
		statuses, err := w.mtx.ListPathStatuses(tickCtx)
		if err != nil {
			w.log.Warn("list mediamtx paths failed", "err", err)
		} else {
			listed = true
			for _, st := range statuses {
				live[st.Name] = st
			}
		}
	}

	now := time.Now().UTC()
	for _, cam := range cameras {
		if !cam.Enabled {
			if cam.IsOnline {
				if err := w.api.Heartbeat(tickCtx, cam.ID, false, cam.LastSeenAt); err != nil {
					w.log.Warn("heartbeat failed", "camera_id", cam.ID, "err", err)
				}
			}
			continue
		}

		st, ok := live[cam.MTXPath]
		if listed && cam.Enabled && cam.RecordingEnabled && !ok {
			if err := w.api.SyncCamera(tickCtx, cam.ID); err != nil {
				w.log.Warn("resync missing path failed", "camera_id", cam.ID, "path", cam.MTXPath, "err", err)
			} else {
				w.log.Info("reprojected missing mediamtx path", "camera_id", cam.ID, "path", cam.MTXPath)
			}
		}
		online := ok && (st.Ready || st.Online)
		var seen *time.Time
		if online {
			seen = &now
		}
		if err := w.api.Heartbeat(tickCtx, cam.ID, online, seen); err != nil {
			w.log.Warn("heartbeat failed", "camera_id", cam.ID, "err", err)
		}

		if !cam.RecordingEnabled || !w.mtx.Enabled() {
			continue
		}
		segments, err := w.mtx.ListPlayback(tickCtx, cam.MTXPath)
		if err != nil {
			w.log.Warn("list playback failed", "camera_id", cam.ID, "path", cam.MTXPath, "err", err)
			continue
		}
		for _, seg := range segments {
			ms := seg.Duration.Milliseconds()
			if ms <= 0 {
				ms = 1000
			}
			if err := w.api.UpsertRecording(tickCtx, cam.ID, seg.Start, ms, "fmp4"); err != nil {
				w.log.Warn("upsert recording failed", "camera_id", cam.ID, "start", seg.Start, "err", err)
			}
		}
	}
}
