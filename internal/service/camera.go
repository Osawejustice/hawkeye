package service

import (
	"context"
	"log/slog"
	"net/url"
	"strings"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/config"
	"github.com/cohi-hq/cohi-api/internal/media/mediamtx"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/google/uuid"
)

type CreateCameraInput struct {
	Name             string
	Description      string
	Location         string
	RTSPURL          string
	RTSPUsername     string
	RTSPPassword     string
	Enabled          *bool
	RecordingEnabled *bool
}

type UpdateCameraInput struct {
	Name             *string
	Description      *string
	Location         *string
	RTSPURL          *string
	RTSPUsername     *string
	RTSPPassword     *string
	Enabled          *bool
	RecordingEnabled *bool
}

type ListCamerasInput struct {
	Query   string
	Enabled *bool
	Page    int
	PerPage int
}

type CameraListResult struct {
	Items   []models.Camera
	Total   int64
	Page    int
	PerPage int
}

type StreamURLs struct {
	Path   string `json:"path"`
	RTSP   string `json:"rtsp"`
	HLS    string `json:"hls"`
	WebRTC string `json:"webrtc"`
	WHEP   string `json:"whep"`
}

type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   string
}

func (a Actor) IsAdmin() bool {
	return a.Role == models.RoleAdmin
}

type CameraService struct {
	cameras *repository.CameraRepository
	mtx     mediamtx.Client
	mtxCfg  config.MediaMTXConfig
	log     *slog.Logger
}

func NewCameraService(
	cameras *repository.CameraRepository,
	mtx mediamtx.Client,
	mtxCfg config.MediaMTXConfig,
	log *slog.Logger,
) *CameraService {
	return &CameraService{cameras: cameras, mtx: mtx, mtxCfg: mtxCfg, log: log}
}

func (s *CameraService) Create(ctx context.Context, actor Actor, in CreateCameraInput) (*models.Camera, error) {
	name, err := normalizeCameraName(in.Name)
	if err != nil {
		return nil, err
	}
	rtspURL, err := normalizeRTSPURL(in.RTSPURL)
	if err != nil {
		return nil, err
	}

	taken, err := s.cameras.NameTaken(ctx, actor.OrgID, name, nil)
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	if taken {
		return nil, apierr.ErrConflict.WithMessage("a camera with this name already exists")
	}

	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	recording := false
	if in.RecordingEnabled != nil {
		recording = *in.RecordingEnabled
	}

	camera := &models.Camera{
		OrganizationID:   actor.OrgID,
		OwnerID:          actor.UserID,
		Name:             name,
		Description:      strings.TrimSpace(in.Description),
		Location:         strings.TrimSpace(in.Location),
		RTSPURL:          rtspURL,
		RTSPUsername:     strings.TrimSpace(in.RTSPUsername),
		RTSPPassword:     in.RTSPPassword,
		Enabled:          enabled,
		RecordingEnabled: recording,
		MTXSyncStatus:    models.MTXSyncPending,
	}

	if err := s.cameras.Create(ctx, camera); err != nil {
		if isUniqueViolation(err) {
			return nil, apierr.ErrConflict.WithMessage("a camera with this name already exists")
		}
		return nil, apierr.ErrInternal.With(err)
	}

	s.syncPath(ctx, camera)
	s.log.Info("camera created", "camera_id", camera.ID, "org_id", actor.OrgID, "mtx_path", camera.MTXPath)
	return camera, nil
}

func (s *CameraService) Get(ctx context.Context, actor Actor, id uuid.UUID) (*models.Camera, error) {
	camera, err := s.loadOwned(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	return camera, nil
}

func (s *CameraService) List(ctx context.Context, actor Actor, in ListCamerasInput) (*CameraListResult, error) {
	page, per := normalizePage(in.Page, in.PerPage)
	items, total, err := s.cameras.List(ctx, repository.CameraListFilter{
		OrganizationID: actor.OrgID,
		Query:          in.Query,
		Enabled:        in.Enabled,
		Offset:         (page - 1) * per,
		Limit:          per,
	})
	if err != nil {
		return nil, apierr.ErrInternal.With(err)
	}
	return &CameraListResult{Items: items, Total: total, Page: page, PerPage: per}, nil
}

func (s *CameraService) Update(ctx context.Context, actor Actor, id uuid.UUID, in UpdateCameraInput) (*models.Camera, error) {
	camera, err := s.loadOwned(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !actor.IsAdmin() && camera.OwnerID != actor.UserID {
		return nil, apierr.ErrForbidden.WithMessage("only the owner or an admin can update this camera")
	}

	resync := false

	if in.Name != nil {
		name, err := normalizeCameraName(*in.Name)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(name, camera.Name) {
			taken, err := s.cameras.NameTaken(ctx, actor.OrgID, name, &camera.ID)
			if err != nil {
				return nil, apierr.ErrInternal.With(err)
			}
			if taken {
				return nil, apierr.ErrConflict.WithMessage("a camera with this name already exists")
			}
		}
		camera.Name = name
	}
	if in.Description != nil {
		camera.Description = strings.TrimSpace(*in.Description)
	}
	if in.Location != nil {
		camera.Location = strings.TrimSpace(*in.Location)
	}
	if in.RTSPURL != nil {
		rtspURL, err := normalizeRTSPURL(*in.RTSPURL)
		if err != nil {
			return nil, err
		}
		if rtspURL != camera.RTSPURL {
			camera.RTSPURL = rtspURL
			resync = true
		}
	}
	if in.RTSPUsername != nil {
		camera.RTSPUsername = strings.TrimSpace(*in.RTSPUsername)
		resync = true
	}
	if in.RTSPPassword != nil {
		camera.RTSPPassword = *in.RTSPPassword
		resync = true
	}
	if in.Enabled != nil && *in.Enabled != camera.Enabled {
		camera.Enabled = *in.Enabled
		resync = true
	}
	if in.RecordingEnabled != nil && *in.RecordingEnabled != camera.RecordingEnabled {
		camera.RecordingEnabled = *in.RecordingEnabled
		resync = true
	}

	if err := s.cameras.Save(ctx, camera); err != nil {
		if isUniqueViolation(err) {
			return nil, apierr.ErrConflict.WithMessage("a camera with this name already exists")
		}
		return nil, apierr.ErrInternal.With(err)
	}

	if resync {
		s.syncPath(ctx, camera)
	}
	return camera, nil
}

func (s *CameraService) Delete(ctx context.Context, actor Actor, id uuid.UUID) error {
	camera, err := s.loadOwned(ctx, actor, id)
	if err != nil {
		return err
	}
	if !actor.IsAdmin() && camera.OwnerID != actor.UserID {
		return apierr.ErrForbidden.WithMessage("only the owner or an admin can delete this camera")
	}

	if err := s.mtx.DeletePath(ctx, camera.MTXPath); err != nil {
		s.log.Warn("failed to delete mediamtx path", "camera_id", camera.ID, "path", camera.MTXPath, "err", err)
	}
	if err := s.cameras.Delete(ctx, camera); err != nil {
		return apierr.ErrInternal.With(err)
	}
	s.log.Info("camera deleted", "camera_id", camera.ID)
	return nil
}

func (s *CameraService) Sync(ctx context.Context, actor Actor, id uuid.UUID) (*models.Camera, error) {
	camera, err := s.loadOwned(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	s.syncPath(ctx, camera)
	return camera, nil
}

func (s *CameraService) Status(ctx context.Context, actor Actor, id uuid.UUID) (*models.Camera, *mediamtx.PathStatus, error) {
	camera, err := s.loadOwned(ctx, actor, id)
	if err != nil {
		return nil, nil, err
	}
	if !s.mtx.Enabled() || !camera.Enabled {
		return camera, nil, nil
	}
	st, err := s.mtx.GetPathStatus(ctx, camera.MTXPath)
	if err != nil {
		return camera, nil, err
	}
	return camera, st, nil
}

func (s *CameraService) StreamURLs(camera *models.Camera) StreamURLs {
	path := camera.MTXPath
	return StreamURLs{
		Path:   path,
		RTSP:   strings.TrimRight(s.mtxCfg.RTSPURL, "/") + "/" + path,
		HLS:    strings.TrimRight(s.mtxCfg.HLSURL, "/") + "/" + path + "/index.m3u8",
		WebRTC: strings.TrimRight(s.mtxCfg.WebRTCURL, "/") + "/" + path,
		WHEP:   strings.TrimRight(s.mtxCfg.WebRTCURL, "/") + "/" + path + "/whep",
	}
}

func (s *CameraService) loadOwned(ctx context.Context, actor Actor, id uuid.UUID) (*models.Camera, error) {
	camera, err := s.cameras.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apierr.ErrNotFound.WithMessage("camera not found")
		}
		return nil, apierr.ErrInternal.With(err)
	}
	if camera.OrganizationID != actor.OrgID {
		return nil, apierr.ErrNotFound.WithMessage("camera not found")
	}
	return camera, nil
}

func (s *CameraService) syncPath(ctx context.Context, camera *models.Camera) {
	if !s.mtx.Enabled() {
		camera.MTXSyncStatus = models.MTXSyncSkipped
		camera.MTXSyncError = ""
		_ = s.cameras.UpdateSync(ctx, camera.ID, camera.MTXSyncStatus, camera.MTXSyncError)
		return
	}

	if !camera.Enabled {
		if err := s.mtx.DeletePath(ctx, camera.MTXPath); err != nil {
			s.setSyncError(ctx, camera, err)
			return
		}
		camera.MTXSyncStatus = models.MTXSyncSynced
		camera.MTXSyncError = ""
		_ = s.cameras.UpdateSync(ctx, camera.ID, camera.MTXSyncStatus, camera.MTXSyncError)
		return
	}

	source := mediamtx.BuildSourceURL(camera.RTSPURL, camera.RTSPUsername, camera.RTSPPassword)
	conf := mediamtx.PathConfigForCamera(source, camera.RecordingEnabled)

	err := s.mtx.AddPath(ctx, camera.MTXPath, conf)
	if err != nil {
		// Path likely already exists (re-sync / retry) — replace it.
		err = s.mtx.ReplacePath(ctx, camera.MTXPath, conf)
	}
	if err != nil {
		s.setSyncError(ctx, camera, err)
		return
	}
	camera.MTXSyncStatus = models.MTXSyncSynced
	camera.MTXSyncError = ""
	_ = s.cameras.UpdateSync(ctx, camera.ID, camera.MTXSyncStatus, camera.MTXSyncError)
}

func (s *CameraService) setSyncError(ctx context.Context, camera *models.Camera, err error) {
	s.log.Warn("mediamtx sync failed", "camera_id", camera.ID, "path", camera.MTXPath, "err", err)
	camera.MTXSyncStatus = models.MTXSyncError
	camera.MTXSyncError = err.Error()
	_ = s.cameras.UpdateSync(ctx, camera.ID, camera.MTXSyncStatus, camera.MTXSyncError)
}

func normalizeCameraName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apierr.ErrInvalidInput.WithMessage("name is required")
	}
	if len(name) > 120 {
		return "", apierr.ErrInvalidInput.WithMessage("name is too long")
	}
	return name, nil
}

func normalizeRTSPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apierr.ErrInvalidInput.WithMessage("rtsp_url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", apierr.ErrInvalidInput.WithMessage("rtsp_url is invalid")
	}
	switch strings.ToLower(u.Scheme) {
	case "rtsp", "rtsps":
	default:
		return "", apierr.ErrInvalidInput.WithMessage("rtsp_url must use rtsp:// or rtsps://")
	}
	return raw, nil
}

func normalizePage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}
