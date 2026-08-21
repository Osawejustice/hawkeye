package httpx

import (
	"time"

	"github.com/cohi-hq/cohi-api/internal/media/mediamtx"
	"github.com/cohi-hq/cohi-api/internal/models"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/google/uuid"
)

type UserResponse struct {
	ID             uuid.UUID            `json:"id"`
	OrganizationID uuid.UUID            `json:"organization_id"`
	Email          string               `json:"email"`
	Name           string               `json:"name"`
	Role           string               `json:"role"`
	IsActive       bool                 `json:"is_active"`
	LastLoginAt    *time.Time           `json:"last_login_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	Organization   *OrganizationSummary `json:"organization,omitempty"`
}

type OrganizationSummary struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type UserSummary struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type AuthResponse struct {
	User   UserResponse  `json:"user"`
	Tokens TokenResponse `json:"tokens"`
}

type CameraResponse struct {
	ID               uuid.UUID          `json:"id"`
	OrganizationID   uuid.UUID          `json:"organization_id"`
	OwnerID          uuid.UUID          `json:"owner_id"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Location         string             `json:"location"`
	RTSPURL          string             `json:"rtsp_url"`
	RTSPUsername     string             `json:"rtsp_username,omitempty"`
	HasRTSPPassword  bool               `json:"has_rtsp_password"`
	Enabled          bool               `json:"enabled"`
	RecordingEnabled bool               `json:"recording_enabled"`
	MTXPath          string             `json:"mtx_path"`
	MTXSyncStatus    string             `json:"mtx_sync_status"`
	MTXSyncError     string             `json:"mtx_sync_error,omitempty"`
	LastSeenAt       *time.Time         `json:"last_seen_at,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Owner            *UserSummary       `json:"owner,omitempty"`
	Stream           service.StreamURLs `json:"stream"`
}

type CameraStatusResponse struct {
	Camera CameraResponse      `json:"camera"`
	Live   *mediamtx.PathStatus `json:"live,omitempty"`
}

func NewUserResponse(u *models.User) UserResponse {
	out := UserResponse{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		Email:          u.Email,
		Name:           u.Name,
		Role:           u.Role,
		IsActive:       u.IsActive,
		LastLoginAt:    u.LastLoginAt,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
	if u.Organization != nil {
		out.Organization = &OrganizationSummary{
			ID:   u.Organization.ID,
			Name: u.Organization.Name,
			Slug: u.Organization.Slug,
		}
	}
	return out
}

func NewAuthResponse(res *service.AuthResult) AuthResponse {
	return AuthResponse{
		User: NewUserResponse(res.User),
		Tokens: TokenResponse{
			AccessToken:  res.AccessToken,
			RefreshToken: res.RefreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(res.AccessExp).Seconds()),
			ExpiresAt:    res.AccessExp.UTC(),
		},
	}
}

func NewCameraResponse(cam *models.Camera, urls service.StreamURLs) CameraResponse {
	out := CameraResponse{
		ID:               cam.ID,
		OrganizationID:   cam.OrganizationID,
		OwnerID:          cam.OwnerID,
		Name:             cam.Name,
		Description:      cam.Description,
		Location:         cam.Location,
		RTSPURL:          mediamtx.RedactRTSPURL(cam.RTSPURL),
		RTSPUsername:     cam.RTSPUsername,
		HasRTSPPassword:  cam.HasRTSPPassword(),
		Enabled:          cam.Enabled,
		RecordingEnabled: cam.RecordingEnabled,
		MTXPath:          cam.MTXPath,
		MTXSyncStatus:    cam.MTXSyncStatus,
		MTXSyncError:     cam.MTXSyncError,
		LastSeenAt:       cam.LastSeenAt,
		CreatedAt:        cam.CreatedAt,
		UpdatedAt:        cam.UpdatedAt,
		Stream:           urls,
	}
	if cam.Owner != nil {
		out.Owner = &UserSummary{
			ID:    cam.Owner.ID,
			Email: cam.Owner.Email,
			Name:  cam.Owner.Name,
		}
	}
	return out
}
