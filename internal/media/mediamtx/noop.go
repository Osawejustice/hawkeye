package mediamtx

import "context"

// NoopClient is used when MediaMTX integration is disabled.
// Path mutations succeed so the control plane remains the source of truth.
type NoopClient struct{}

func (n *NoopClient) Enabled() bool { return false }

func (n *NoopClient) Health(ctx context.Context) error { return nil }

func (n *NoopClient) AddPath(ctx context.Context, name string, conf PathConfig) error {
	return nil
}

func (n *NoopClient) PatchPath(ctx context.Context, name string, conf PathConfig) error {
	return nil
}

func (n *NoopClient) ReplacePath(ctx context.Context, name string, conf PathConfig) error {
	return nil
}

func (n *NoopClient) DeletePath(ctx context.Context, name string) error { return nil }

func (n *NoopClient) GetPathConfig(ctx context.Context, name string) (*PathConfig, error) {
	return &PathConfig{}, nil
}

func (n *NoopClient) GetPathStatus(ctx context.Context, name string) (*PathStatus, error) {
	return &PathStatus{Name: name}, nil
}

func (n *NoopClient) ListPathStatuses(ctx context.Context) ([]PathStatus, error) {
	return nil, nil
}

func (n *NoopClient) ListPlayback(ctx context.Context, path string) ([]PlaybackSegment, error) {
	return nil, nil
}
