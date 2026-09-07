package mediamtx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cohi-hq/cohi-api/internal/config"
)

// Client talks to the MediaMTX Control API (v3).
//
// The control plane owns camera records; this client is how we project
// those records onto MediaMTX paths (RTSP ingest, HLS/WebRTC egress, recording).
type Client interface {
	Enabled() bool
	Health(ctx context.Context) error
	AddPath(ctx context.Context, name string, conf PathConfig) error
	PatchPath(ctx context.Context, name string, conf PathConfig) error
	ReplacePath(ctx context.Context, name string, conf PathConfig) error
	DeletePath(ctx context.Context, name string) error
	GetPathConfig(ctx context.Context, name string) (*PathConfig, error)
	GetPathStatus(ctx context.Context, name string) (*PathStatus, error)
	ListPathStatuses(ctx context.Context) ([]PathStatus, error)
	ListPlayback(ctx context.Context, path string) ([]PlaybackSegment, error)
}

// PathConfig is the subset of MediaMTX PathConf we manage today.
type PathConfig struct {
	Source         string `json:"source,omitempty"`
	SourceOnDemand bool   `json:"sourceOnDemand"`
	Record         bool   `json:"record"`
	RecordPath     string `json:"recordPath,omitempty"`
}

// PathStatus is the runtime state of a MediaMTX path.
type PathStatus struct {
	Name      string `json:"name"`
	Ready     bool   `json:"ready"`
	Online    bool   `json:"online"`
	Available bool   `json:"available"`
	ConfName  string `json:"confName"`
	Tracks    int    `json:"tracks"`
	Readers   int    `json:"readers"`
}

// PlaybackSegment is one recorded time span from the Playback API.
type PlaybackSegment struct {
	Start    time.Time     `json:"start"`
	Duration time.Duration `json:"duration"`
	URL      string        `json:"url"`
}

type playbackListEntry struct {
	Start    string  `json:"start"`
	Duration float64 `json:"duration"`
	URL      string  `json:"url"`
}

type listEnvelope[T any] struct {
	ItemCount int64 `json:"itemCount"`
	PageCount int64 `json:"pageCount"`
	Items     []T   `json:"items"`
}

type apiError struct {
	Error  string `json:"error"`
	Status string `json:"status"`
}

type runtimePath struct {
	Name      string          `json:"name"`
	Ready     bool            `json:"ready"`
	Online    bool            `json:"online"`
	Available bool            `json:"available"`
	ConfName  string          `json:"confName"`
	Tracks2   []any           `json:"tracks2"`
	Readers   []any           `json:"readers"`
	Source    json.RawMessage `json:"source"`
}

type HTTPClient struct {
	enabled     bool
	baseURL     string
	playbackURL string
	user        string
	pass        string
	http        *http.Client
}

func New(cfg config.MediaMTXConfig) Client {
	if !cfg.Enabled {
		return &NoopClient{}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &HTTPClient{
		enabled:     true,
		baseURL:     strings.TrimRight(cfg.APIURL, "/"),
		playbackURL: strings.TrimRight(cfg.PlaybackURL, "/"),
		user:        cfg.APIUser,
		pass:        cfg.APIPass,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        16,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
			},
		},
	}
}

func (c *HTTPClient) Enabled() bool { return c.enabled }

func (c *HTTPClient) Health(ctx context.Context) error {
	// Listing path configs is a cheap authenticated Control API call.
	_, err := c.do(ctx, http.MethodGet, "/v3/config/paths/list?page=0&itemsPerPage=1", nil)
	return err
}

func (c *HTTPClient) AddPath(ctx context.Context, name string, conf PathConfig) error {
	_, err := c.do(ctx, http.MethodPost, "/v3/config/paths/add/"+url.PathEscape(name), conf)
	return err
}

func (c *HTTPClient) PatchPath(ctx context.Context, name string, conf PathConfig) error {
	_, err := c.do(ctx, http.MethodPatch, "/v3/config/paths/patch/"+url.PathEscape(name), conf)
	return err
}

func (c *HTTPClient) ReplacePath(ctx context.Context, name string, conf PathConfig) error {
	_, err := c.do(ctx, http.MethodPost, "/v3/config/paths/replace/"+url.PathEscape(name), conf)
	return err
}

func (c *HTTPClient) DeletePath(ctx context.Context, name string) error {
	_, err := c.do(ctx, http.MethodDelete, "/v3/config/paths/delete/"+url.PathEscape(name), nil)
	if err != nil && isNotFound(err) {
		return nil
	}
	return err
}

func (c *HTTPClient) GetPathConfig(ctx context.Context, name string) (*PathConfig, error) {
	body, err := c.do(ctx, http.MethodGet, "/v3/config/paths/get/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	var conf PathConfig
	if err := json.Unmarshal(body, &conf); err != nil {
		return nil, fmt.Errorf("decode path config: %w", err)
	}
	return &conf, nil
}

func (c *HTTPClient) GetPathStatus(ctx context.Context, name string) (*PathStatus, error) {
	body, err := c.do(ctx, http.MethodGet, "/v3/paths/get/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	var raw runtimePath
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode path status: %w", err)
	}
	return pathStatusFromRuntime(raw), nil
}

func (c *HTTPClient) ListPathStatuses(ctx context.Context) ([]PathStatus, error) {
	var out []PathStatus
	page := 0
	for {
		body, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v3/paths/list?page=%d&itemsPerPage=100", page), nil)
		if err != nil {
			return nil, err
		}
		var env listEnvelope[runtimePath]
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode path list: %w", err)
		}
		for _, raw := range env.Items {
			out = append(out, *pathStatusFromRuntime(raw))
		}
		if page+1 >= int(env.PageCount) || len(env.Items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (c *HTTPClient) ListPlayback(ctx context.Context, path string) ([]PlaybackSegment, error) {
	if c.playbackURL == "" {
		return nil, nil
	}
	u, err := url.Parse(c.playbackURL + "/list")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("path", path)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cohi-api")
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediamtx playback list: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.StatusCode, Message: strings.TrimSpace(string(body))}
	}

	var entries []playbackListEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode playback list: %w", err)
	}
	out := make([]PlaybackSegment, 0, len(entries))
	for _, e := range entries {
		start, err := time.Parse(time.RFC3339Nano, e.Start)
		if err != nil {
			start, err = time.Parse(time.RFC3339, e.Start)
			if err != nil {
				continue
			}
		}
		out = append(out, PlaybackSegment{
			Start:    start.UTC(),
			Duration: time.Duration(e.Duration * float64(time.Second)),
			URL:      e.URL,
		})
	}
	return out, nil
}

func pathStatusFromRuntime(raw runtimePath) *PathStatus {
	return &PathStatus{
		Name:      raw.Name,
		Ready:     raw.Ready || raw.Online,
		Online:    raw.Online || raw.Ready,
		Available: raw.Available,
		ConfName:  raw.ConfName,
		Tracks:    len(raw.Tracks2),
		Readers:   len(raw.Readers),
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cohi-api")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediamtx request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read mediamtx response: %w", err)
	}

	if resp.StatusCode >= 300 {
		var apiErr apiError
		_ = json.Unmarshal(respBody, &apiErr)
		msg := strings.TrimSpace(apiErr.Error)
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, &HTTPError{Status: resp.StatusCode, Message: msg}
	}
	return respBody, nil
}

// HTTPError is a non-2xx Control API response.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("mediamtx api %d: %s", e.Status, e.Message)
}

func isNotFound(err error) bool {
	var httpErr *HTTPError
	ok := errorAs(err, &httpErr)
	return ok && httpErr.Status == http.StatusNotFound
}

func errorAs(err error, target **HTTPError) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*HTTPError)
	if ok {
		*target = e
		return true
	}
	return false
}

// PathConfigForCamera builds the MediaMTX path configuration for a camera.
func PathConfigForCamera(sourceURL string, recordingEnabled bool) PathConfig {
	conf := PathConfig{
		Source:         sourceURL,
		SourceOnDemand: !recordingEnabled,
		Record:         recordingEnabled,
	}
	if recordingEnabled {
		conf.RecordPath = "/recordings/%path/%Y-%m-%d_%H-%M-%S-%f"
	}
	return conf
}

// BuildSourceURL injects optional credentials into an RTSP URL.
func BuildSourceURL(rtspURL, username, password string) string {
	u, err := url.Parse(rtspURL)
	if err != nil {
		return rtspURL
	}
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}
	return u.String()
}

// RedactRTSPURL strips passwords from an RTSP URL for API responses / logs.
func RedactRTSPURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, hasPass := u.User.Password(); !hasPass {
		return raw
	}
	userinfo := url.User(u.User.Username()).String()
	u.User = nil
	rest := strings.TrimPrefix(u.String(), u.Scheme+"://")
	return u.Scheme + "://" + userinfo + ":****@" + rest
}
