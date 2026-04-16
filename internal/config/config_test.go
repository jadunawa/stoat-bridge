package config

import (
	"testing"
)

func TestLoad_MissingRequiredVars(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "")
	t.Setenv("STOAT_CHANNEL_ID", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when required vars are missing")
	}
}

func TestLoad_MissingBotToken(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "")
	t.Setenv("STOAT_CHANNEL_ID", "test-channel")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when STOAT_BOT_TOKEN is missing")
	}
}

func TestLoad_MissingChannelID(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when STOAT_CHANNEL_ID is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "default-ch")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 { t.Errorf("Port = %d, want 8080", cfg.Port) }
	if cfg.LogLevel != "info" { t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info") }
	if cfg.QueueSize != 100 { t.Errorf("QueueSize = %d, want 100", cfg.QueueSize) }
	if cfg.MaxRetries != 3 { t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries) }
	if cfg.MaxBodySize != 1048576 { t.Errorf("MaxBodySize = %d, want 1048576", cfg.MaxBodySize) }
	if cfg.MaxMessageLength != 1900 { t.Errorf("MaxMessageLength = %d, want 1900", cfg.MaxMessageLength) }
	if cfg.RateLimit != 100 { t.Errorf("RateLimit = %d, want 100", cfg.RateLimit) }
	if cfg.StoatAPIURL != "https://api.stoat.chat" { t.Errorf("StoatAPIURL = %q, want default", cfg.StoatAPIURL) }
	if cfg.ShutdownTimeout.Seconds() != 10 { t.Errorf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout) }
}

func TestLoad_ChannelIDFallbacks(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "default-ch")
	t.Setenv("STOAT_CRITICAL_CHANNEL_ID", "")
	t.Setenv("STOAT_WARNING_CHANNEL_ID", "")
	cfg, err := Load()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if cfg.CriticalChannelID != "default-ch" { t.Errorf("CriticalChannelID = %q, want %q", cfg.CriticalChannelID, "default-ch") }
	if cfg.WarningChannelID != "default-ch" { t.Errorf("WarningChannelID = %q, want %q", cfg.WarningChannelID, "default-ch") }
}

func TestLoad_ChannelIDOverrides(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "default-ch")
	t.Setenv("STOAT_CRITICAL_CHANNEL_ID", "critical-ch")
	t.Setenv("STOAT_WARNING_CHANNEL_ID", "warning-ch")
	cfg, err := Load()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if cfg.CriticalChannelID != "critical-ch" { t.Errorf("CriticalChannelID = %q, want %q", cfg.CriticalChannelID, "critical-ch") }
	if cfg.WarningChannelID != "warning-ch" { t.Errorf("WarningChannelID = %q, want %q", cfg.WarningChannelID, "warning-ch") }
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "default-ch")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("QUEUE_SIZE", "50")
	t.Setenv("MAX_RETRIES", "5")
	t.Setenv("RATE_LIMIT", "200")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")
	cfg, err := Load()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if cfg.Port != 9090 { t.Errorf("Port = %d, want 9090", cfg.Port) }
	if cfg.LogLevel != "debug" { t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug") }
	if cfg.QueueSize != 50 { t.Errorf("QueueSize = %d, want 50", cfg.QueueSize) }
	if cfg.MaxRetries != 5 { t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries) }
	if cfg.RateLimit != 200 { t.Errorf("RateLimit = %d, want 200", cfg.RateLimit) }
	if cfg.ShutdownTimeout.Seconds() != 30 { t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout) }
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "default-ch")
	t.Setenv("PORT", "not-a-number")
	_, err := Load()
	if err == nil { t.Fatal("expected error for invalid PORT") }
}

func TestLoad_InvalidQueueSize(t *testing.T) {
	t.Setenv("STOAT_BOT_TOKEN", "test-token")
	t.Setenv("STOAT_CHANNEL_ID", "default-ch")
	t.Setenv("QUEUE_SIZE", "0")
	_, err := Load()
	if err == nil { t.Fatal("expected error for zero QUEUE_SIZE") }
}
