package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jadunawa/stoat-bridge/internal/config"
	"github.com/jadunawa/stoat-bridge/internal/handler"
	"github.com/jadunawa/stoat-bridge/internal/metrics"
	"github.com/jadunawa/stoat-bridge/internal/queue"
	"github.com/jadunawa/stoat-bridge/internal/sender"
	"github.com/jadunawa/stoat-bridge/internal/server"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	logger.Info("starting stoat-bridge",
		"version", version,
		"commit", commit,
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"queue_size", cfg.QueueSize,
		"max_retries", cfg.MaxRetries,
	)

	met := metrics.New()

	httpClient := &http.Client{Timeout: 10 * time.Second}
	stoatSender := sender.NewStoatSender(cfg.StoatAPIURL, cfg.StoatBotToken, httpClient)

	q := queue.New(stoatSender, cfg.QueueSize, cfg.MaxRetries, logger, met)

	reg := handler.NewRegistry()

	grafanaOpts := handler.Options{
		CriticalChannelID: cfg.CriticalChannelID,
		WarningChannelID:  cfg.WarningChannelID,
		DefaultChannelID:  cfg.StoatChannelID,
		Template:          cfg.GrafanaTemplate,
		MaxMessageLength:  cfg.MaxMessageLength,
	}
	reg.Register(handler.NewGrafanaHandler(grafanaOpts))

	amOpts := handler.Options{
		CriticalChannelID: cfg.CriticalChannelID,
		WarningChannelID:  cfg.WarningChannelID,
		DefaultChannelID:  cfg.StoatChannelID,
		Template:          cfg.AlertmanagerTemplate,
		MaxMessageLength:  cfg.MaxMessageLength,
	}
	reg.Register(handler.NewAlertmanagerHandler(amOpts))

	gatusOpts := handler.Options{
		CriticalChannelID: cfg.CriticalChannelID,
		WarningChannelID:  cfg.WarningChannelID,
		DefaultChannelID:  cfg.StoatChannelID,
		Template:          cfg.GatusTemplate,
		MaxMessageLength:  cfg.MaxMessageLength,
	}
	reg.Register(handler.NewGatusHandler(gatusOpts))

	srv := server.New(&server.Config{
		MaxBodySize:   cfg.MaxBodySize,
		RateLimit:     cfg.RateLimit,
		WebhookSecret: cfg.WebhookSecret,
	}, reg, q, met)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	srv.SetReady(true)
	logger.Info("stoat-bridge is ready", "addr", httpServer.Addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := <-sigCh
	logger.Info("shutdown signal received", "signal", sig)

	srv.SetReady(false)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}

	q.Shutdown(cfg.ShutdownTimeout)
	cancel()

	logger.Info("stoat-bridge stopped")
}
