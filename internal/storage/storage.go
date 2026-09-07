package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

const (
	BackendLocal = "local"
	BackendMTX   = "mediamtx"
	BackendR2    = "r2"
)

// Object is a recorded media blob the control plane can open for playback.
type Object struct {
	Backend     string
	Path        string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

// Backend reads (and later writes) recording bytes.
// Local MVP: MediaMTX playback. Later: Cloudflare R2.
type Backend interface {
	Name() string
	OpenPlayback(ctx context.Context, mtxPath string, start time.Time, duration time.Duration) (*Object, error)
}

// Key builds a stable storage_path for a MediaMTX-backed segment.
func MTXKey(mtxPath string, start time.Time) string {
	return fmt.Sprintf("mtx://%s/%s", mtxPath, start.UTC().Format(time.RFC3339Nano))
}
