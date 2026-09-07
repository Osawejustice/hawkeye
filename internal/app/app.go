package app

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/cohi-hq/cohi-api/internal/auth"
	"github.com/cohi-hq/cohi-api/internal/config"
	"github.com/cohi-hq/cohi-api/internal/database"
	httpx "github.com/cohi-hq/cohi-api/internal/http"
	"github.com/cohi-hq/cohi-api/internal/media/mediamtx"
	"github.com/cohi-hq/cohi-api/internal/repository"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/cohi-hq/cohi-api/internal/storage"
	"gorm.io/gorm"
)

var Version = "0.2.0"

type App struct {
	Config  *config.Config
	Log     *slog.Logger
	DB      *gorm.DB
	Handler http.Handler
	MTX     mediamtx.Client
	Cameras *service.CameraService
}

func New(cfg *config.Config, log *slog.Logger) (*App, error) {
	if cfg.AutoMigrate {
		log.Info("running database migrations", "path", cfg.MigrationsPath)
		if err := database.RunMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
			return nil, fmt.Errorf("migrations: %w", err)
		}
	}

	db, err := database.New(cfg, log)
	if err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	cameraRepo := repository.NewCameraRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	eventRepo := repository.NewEventRepository(db)

	jwtMgr := auth.NewJWTManager(cfg.JWTAccessSecret, cfg.JWTAccessTTL, cfg.JWTIssuer)
	mtx := mediamtx.New(cfg.MediaMTX)
	store := storage.NewMediaMTXPlayback(
		cfg.MediaMTX.PlaybackURL,
		cfg.MediaMTX.APIUser,
		cfg.MediaMTX.APIPass,
		cfg.MediaMTX.Timeout,
	)

	authSvc := service.NewAuthService(db, userRepo, tokenRepo, jwtMgr, cfg.JWTRefreshTTL, cfg.JWTRefreshSecret, log)
	userSvc := service.NewUserService(userRepo)
	eventSvc := service.NewEventService(eventRepo, log)
	cameraSvc := service.NewCameraService(cameraRepo, mtx, cfg.MediaMTX, eventSvc, log)
	recordingSvc := service.NewRecordingService(recordingRepo, cameraRepo, eventSvc, store, log)
	internalSvc := service.NewInternalService(cameraRepo, eventSvc, log)

	router := httpx.NewRouter(httpx.Dependencies{
		Config:     cfg,
		Log:        log,
		Auth:       authSvc,
		Users:      userSvc,
		Cameras:    cameraSvc,
		Recordings: recordingSvc,
		Events:     eventSvc,
		Internal:   internalSvc,
		Health:     httpx.NewHealthHandler(db, mtx, Version),
	})

	return &App{
		Config:  cfg,
		Log:     log,
		DB:      db,
		Handler: router,
		MTX:     mtx,
		Cameras: cameraSvc,
	}, nil
}

func (a *App) Close() error {
	return database.Close(a.DB)
}
