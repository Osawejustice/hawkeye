package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cohi-hq/cohi-api/internal/logger"
	"github.com/cohi-hq/cohi-api/internal/worker"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := worker.LoadConfig()
	if err != nil {
		slog.Error("load config", "err", err)
		return 1
	}

	log := logger.New(cfg.LogLevel, cfg.LogFormat, os.Stdout)
	slog.SetDefault(log)

	w, err := worker.New(cfg, log)
	if err != nil {
		log.Error("bootstrap failed", "err", err)
		return 1
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"status":"ok","service":"cohi-worker"}`))
	})
	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("cohi-worker health listening", "addr", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("health server", "err", err)
		}
	}()

	log.Info("cohi-worker started",
		"api", cfg.APIURL,
		"poll", cfg.PollInterval.String(),
		"mediamtx", cfg.MediaMTX.APIURL,
	)

	w.Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	return 0
}
