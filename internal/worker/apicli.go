package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type apiClient struct {
	base  string
	token string
	http  *http.Client
}

type cameraRow struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	Name             string     `json:"name"`
	MTXPath          string     `json:"mtx_path"`
	Enabled          bool       `json:"enabled"`
	RecordingEnabled bool       `json:"recording_enabled"`
	IsOnline         bool       `json:"is_online"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newAPIClient(base, token string, timeout time.Duration) *apiClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &apiClient{
		base:  strings.TrimRight(base, "/"),
		token: token,
		http:  &http.Client{Timeout: timeout},
	}
}

func (c *apiClient) ListCameras(ctx context.Context) ([]cameraRow, error) {
	var payload struct {
		Items []cameraRow `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/internal/v1/cameras", nil, &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (c *apiClient) Heartbeat(ctx context.Context, id uuid.UUID, online bool, seenAt *time.Time) error {
	body := map[string]any{"online": online}
	if seenAt != nil {
		body["seen_at"] = seenAt.UTC()
	}
	return c.do(ctx, http.MethodPost, "/internal/v1/cameras/"+id.String()+"/heartbeat", body, nil)
}

func (c *apiClient) SyncCamera(ctx context.Context, id uuid.UUID) error {
	return c.do(ctx, http.MethodPost, "/internal/v1/cameras/"+id.String()+"/sync", map[string]any{}, nil)
}

func (c *apiClient) UpsertRecording(ctx context.Context, cameraID uuid.UUID, started time.Time, durationMS int64, format string) error {
	body := map[string]any{
		"camera_id":   cameraID,
		"started_at":  started.UTC(),
		"duration_ms": durationMS,
		"trigger":     "continuous",
		"format":      format,
	}
	return c.do(ctx, http.MethodPost, "/internal/v1/recordings/upsert", body, nil)
}

func (c *apiClient) do(ctx context.Context, method, path string, payload any, dest any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cohi-worker")
	req.Header.Set("X-Service-Token", c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("api decode %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode >= 300 || !env.Success {
		msg := strings.TrimSpace(string(raw))
		if env.Error != nil && env.Error.Message != "" {
			msg = env.Error.Message
		}
		return fmt.Errorf("api %s %s: %d %s", method, path, resp.StatusCode, msg)
	}
	if dest != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, dest)
	}
	return nil
}
