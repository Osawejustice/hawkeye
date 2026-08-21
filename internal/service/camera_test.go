package service

import "testing"

func TestNormalizeRTSPURL(t *testing.T) {
	got, err := normalizeRTSPURL("  rtsp://192.168.0.10:554/live  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rtsp://192.168.0.10:554/live" {
		t.Fatalf("got %q", got)
	}

	if _, err := normalizeRTSPURL("http://example.com/stream"); err == nil {
		t.Fatal("expected error for non-rtsp scheme")
	}
	if _, err := normalizeRTSPURL(""); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestNormalizeCameraName(t *testing.T) {
	got, err := normalizeCameraName("  Front Gate  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Front Gate" {
		t.Fatalf("got %q", got)
	}
	if _, err := normalizeCameraName("   "); err == nil {
		t.Fatal("expected error for blank name")
	}
}

func TestNormalizePage(t *testing.T) {
	page, per := normalizePage(0, 0)
	if page != 1 || per != 20 {
		t.Fatalf("defaults: page=%d per=%d", page, per)
	}
	page, per = normalizePage(2, 500)
	if page != 2 || per != 100 {
		t.Fatalf("cap: page=%d per=%d", page, per)
	}
}
