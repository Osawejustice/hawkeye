package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MediaMTXPlayback fetches recorded time spans from the MediaMTX Playback API.
type MediaMTXPlayback struct {
	BaseURL  string
	User     string
	Pass     string
	Timeout  time.Duration
	HTTP     *http.Client
}

func NewMediaMTXPlayback(baseURL, user, pass string, timeout time.Duration) *MediaMTXPlayback {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MediaMTXPlayback{
		BaseURL: strings.TrimRight(baseURL, "/"),
		User:    user,
		Pass:    pass,
		Timeout: timeout,
		HTTP: &http.Client{
			Timeout: 0, // caller owns the stream lifetime
			Transport: &http.Transport{
				MaxIdleConns:        8,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
				ResponseHeaderTimeout: 20 * time.Second,
			},
		},
	}
}

func (m *MediaMTXPlayback) Name() string { return BackendMTX }

func (m *MediaMTXPlayback) OpenPlayback(ctx context.Context, mtxPath string, start time.Time, duration time.Duration) (*Object, error) {
	if m == nil || m.BaseURL == "" {
		return nil, fmt.Errorf("mediamtx playback is not configured")
	}
	if duration <= 0 {
		duration = time.Second
	}

	u, err := url.Parse(m.BaseURL + "/get")
	if err != nil {
		return nil, fmt.Errorf("playback url: %w", err)
	}
	q := u.Query()
	q.Set("path", mtxPath)
	q.Set("start", start.UTC().Format(time.RFC3339Nano))
	q.Set("duration", fmt.Sprintf("%.3f", duration.Seconds()))
	q.Set("format", "mp4")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cohi-api")
	if m.User != "" {
		req.SetBasicAuth(m.User, m.Pass)
	}

	resp, err := m.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediamtx playback: %w", err)
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("mediamtx playback %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	ctype := resp.Header.Get("Content-Type")
	if ctype == "" {
		ctype = "video/mp4"
	}
	return &Object{
		Backend:     BackendMTX,
		Path:        u.String(),
		ContentType: ctype,
		Size:        resp.ContentLength,
		Body:        resp.Body,
	}, nil
}
