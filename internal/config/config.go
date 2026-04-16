package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             int
	LogLevel         string
	QueueSize        int
	MaxRetries       int
	ShutdownTimeout  time.Duration
	MaxBodySize      int64
	MaxMessageLength int
	RateLimit        int

	StoatAPIURL       string
	StoatBotToken     string
	StoatChannelID    string
	CriticalChannelID string
	WarningChannelID  string

	WebhookSecret string

	GrafanaTemplate      string
	AlertmanagerTemplate string
	GatusTemplate        string
}

func Load() (*Config, error) {
	botToken := os.Getenv("STOAT_BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("required environment variable STOAT_BOT_TOKEN is not set")
	}

	channelID := os.Getenv("STOAT_CHANNEL_ID")
	if channelID == "" {
		return nil, fmt.Errorf("required environment variable STOAT_CHANNEL_ID is not set")
	}

	port, err := getEnvInt("PORT", 8080)
	if err != nil { return nil, fmt.Errorf("invalid PORT: %w", err) }

	queueSize, err := getEnvInt("QUEUE_SIZE", 100)
	if err != nil { return nil, fmt.Errorf("invalid QUEUE_SIZE: %w", err) }
	if queueSize <= 0 { return nil, fmt.Errorf("QUEUE_SIZE must be positive, got %d", queueSize) }

	maxRetries, err := getEnvInt("MAX_RETRIES", 3)
	if err != nil { return nil, fmt.Errorf("invalid MAX_RETRIES: %w", err) }

	maxBodySize, err := getEnvInt64("MAX_BODY_SIZE", 1048576)
	if err != nil { return nil, fmt.Errorf("invalid MAX_BODY_SIZE: %w", err) }
	if maxBodySize > 1048576 { slog.Warn("MAX_BODY_SIZE is set above 1MB", "value", maxBodySize) }

	maxMessageLength, err := getEnvInt("MAX_MESSAGE_LENGTH", 1900)
	if err != nil { return nil, fmt.Errorf("invalid MAX_MESSAGE_LENGTH: %w", err) }

	rateLimit, err := getEnvInt("RATE_LIMIT", 100)
	if err != nil { return nil, fmt.Errorf("invalid RATE_LIMIT: %w", err) }

	shutdownTimeout, err := getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil { return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err) }

	criticalCh := os.Getenv("STOAT_CRITICAL_CHANNEL_ID")
	if criticalCh == "" { criticalCh = channelID }
	warningCh := os.Getenv("STOAT_WARNING_CHANNEL_ID")
	if warningCh == "" { warningCh = channelID }

	return &Config{
		Port: port, LogLevel: getEnvDefault("LOG_LEVEL", "info"),
		QueueSize: queueSize, MaxRetries: maxRetries,
		ShutdownTimeout: shutdownTimeout, MaxBodySize: maxBodySize,
		MaxMessageLength: maxMessageLength, RateLimit: rateLimit,
		StoatAPIURL: getEnvDefault("STOAT_API_URL", "https://api.stoat.chat"),
		StoatBotToken: botToken, StoatChannelID: channelID,
		CriticalChannelID: criticalCh, WarningChannelID: warningCh,
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		GrafanaTemplate: os.Getenv("GRAFANA_TEMPLATE"),
		AlertmanagerTemplate: os.Getenv("ALERTMANAGER_TEMPLATE"),
		GatusTemplate: os.Getenv("GATUS_TEMPLATE"),
	}, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" { return fallback, nil }
	return strconv.Atoi(v)
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" { return fallback, nil }
	return strconv.ParseInt(v, 10, 64)
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" { return fallback, nil }
	return time.ParseDuration(v)
}
