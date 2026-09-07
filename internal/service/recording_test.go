package service

import (
	"testing"
	"time"

	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/storage"
	"github.com/google/uuid"
)

func TestSegmentDuration(t *testing.T) {
	ms := int64(1500)
	seg := &models.RecordingSegment{DurationMS: &ms}
	if got := segmentDuration(seg); got != 1500*time.Millisecond {
		t.Fatalf("got %s", got)
	}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Second)
	seg = &models.RecordingSegment{StartedAt: start, EndedAt: &end}
	if got := segmentDuration(seg); got != 3*time.Second {
		t.Fatalf("from timestamps: %s", got)
	}
}

func TestMTXKey(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	start := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	got := storage.MTXKey("cam-"+id.String(), start)
	want := "mtx://cam-11111111-1111-1111-1111-111111111111/2026-09-07T12:00:00Z"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
