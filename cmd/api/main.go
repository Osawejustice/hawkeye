package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cohi-hq/cohi-api/internal/app"
	"github.com/cohi-hq/cohi-api/internal/config"
	httpx "github.com/cohi-hq/cohi-api/internal/http"
	"github.com/cohi-hq/cohi-api/internal/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		return 1
	}

	log := logger.New(cfg.LogLevel, cfg.LogFormat, os.Stdout)
	slog.SetDefault(log)

	if cfg.IsDevelopment() && (os.Getenv("JWT_ACCESS_SECRET") == "" || os.Getenv("JWT_REFRESH_SECRET") == "") {
		log.Warn("using development JWT secrets; set JWT_ACCESS_SECRET and JWT_REFRESH_SECRET before deploying")
	}

	application, err := app.New(cfg, log)
	if err != nil {
		log.Error("bootstrap failed", "err", err)
		return 1
	}
	defer func() {
		if err := application.Close(); err != nil {
			log.Error("close resources", "err", err)
		}
	}()

	srv := httpx.NewServer(cfg.Addr(), application.Handler, log)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	log.Info("cohi-api started",
		"env", cfg.AppEnv,
		"addr", cfg.Addr(),
		"version", app.Version,
		"mediamtx", cfg.MediaMTX.Enabled,
	)

	go resyncMediaMTX(application)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			log.Error("server error", "err", err)
			return 1
		}
	case sig := <-stop:
		log.Info("shutdown signal received", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
		return 1
	}
	return 0
}

func resyncMediaMTX(application *app.App) {
	if application.Cameras == nil || !application.MTX.Enabled() {
		return
	}
	delays := []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second}
	for i, wait := range delays {
		time.Sleep(wait)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := application.MTX.Health(ctx)
		if err != nil {
			cancel()
			application.Log.Warn("mediamtx not ready for resync", "attempt", i+1, "err", err)
			continue
		}
		application.Cameras.ResyncAll(ctx)
		cancel()
		return
	}
	application.Log.Warn("mediamtx resync skipped; control API never became healthy")
}
