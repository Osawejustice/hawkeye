package mediamtx

import "testing"

func TestBuildSourceURLInjectsCredentials(t *testing.T) {
	got := BuildSourceURL("rtsp://192.168.1.20:554/stream", "admin", "s3cret")
	want := "rtsp://admin:s3cret@192.168.1.20:554/stream"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRedactRTSPURL(t *testing.T) {
	got := RedactRTSPURL("rtsp://admin:s3cret@192.168.1.20:554/stream")
	if got != "rtsp://admin:****@192.168.1.20:554/stream" {
		t.Fatalf("got %q", got)
	}
	plain := "rtsp://192.168.1.20:554/stream"
	if RedactRTSPURL(plain) != plain {
		t.Fatal("plain URL should be unchanged")
	}
}

func TestPathConfigForCamera(t *testing.T) {
	onDemand := PathConfigForCamera("rtsp://cam/stream", false)
	if !onDemand.SourceOnDemand || onDemand.Record {
		t.Fatalf("unexpected on-demand conf: %+v", onDemand)
	}
	recording := PathConfigForCamera("rtsp://cam/stream", true)
	if recording.SourceOnDemand || !recording.Record {
		t.Fatalf("unexpected recording conf: %+v", recording)
	}
}
