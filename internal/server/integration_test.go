package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jadunawa/stoat-bridge/internal/handler"
	"github.com/jadunawa/stoat-bridge/internal/metrics"
	"github.com/jadunawa/stoat-bridge/internal/queue"
	"github.com/jadunawa/stoat-bridge/internal/sender"
	"github.com/jadunawa/stoat-bridge/internal/server"
)

// fakeStoat simulates the Stoat API for integration tests.
type fakeStoat struct {
	mu       sync.Mutex
	messages []fakeStoatMessage
	server   *httptest.Server
}

type fakeStoatMessage struct {
	ChannelID string
	Content   string
}

func newFakeStoat() *fakeStoat {
	fs := &fakeStoat{}
	fs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "channels" || parts[2] != "messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		channelID := parts[1]

		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(body, &payload)

		fs.mu.Lock()
		fs.messages = append(fs.messages, fakeStoatMessage{
			ChannelID: channelID,
			Content:   payload.Content,
		})
		fs.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	return fs
}

func (fs *fakeStoat) getMessages() []fakeStoatMessage {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	result := make([]fakeStoatMessage, len(fs.messages))
	copy(result, fs.messages)
	return result
}

func (fs *fakeStoat) close() {
	fs.server.Close()
}

func TestIntegration_GrafanaToStoat(t *testing.T) {
	stoat := newFakeStoat()
	defer stoat.close()

	met := metrics.New()
	stoatSender := sender.NewStoatSender(stoat.server.URL, "test-token", &http.Client{})
	q := queue.New(stoatSender, 10, 3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	reg := handler.NewRegistry()
	reg.Register(handler.NewGrafanaHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))

	srv := server.New(&server.Config{
		MaxBodySize: 1048576,
		RateLimit:   100,
	}, reg, q, met)
	srv.SetReady(true)

	body := `{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "HighMemory", "severity": "critical"},
			"annotations": {"description": "Memory above 95%"},
			"generatorURL": "https://grafana.example.com/alerting/rule/1"
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for Stoat delivery")
		case <-time.After(50 * time.Millisecond):
			msgs := stoat.getMessages()
			if len(msgs) > 0 {
				if msgs[0].ChannelID != "crit-ch" {
					t.Errorf("channel = %q, want %q", msgs[0].ChannelID, "crit-ch")
				}
				if !strings.Contains(msgs[0].Content, "HighMemory") {
					t.Error("message content missing alert name")
				}
				if !strings.Contains(msgs[0].Content, "Triggered") {
					t.Error("message content missing state")
				}
				return
			}
		}
	}
}

func TestIntegration_AlertmanagerToStoat(t *testing.T) {
	stoat := newFakeStoat()
	defer stoat.close()

	met := metrics.New()
	stoatSender := sender.NewStoatSender(stoat.server.URL, "test-token", &http.Client{})
	q := queue.New(stoatSender, 10, 3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	reg := handler.NewRegistry()
	reg.Register(handler.NewAlertmanagerHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))

	srv := server.New(&server.Config{
		MaxBodySize: 1048576,
		RateLimit:   100,
	}, reg, q, met)
	srv.SetReady(true)

	body := `{
		"version": "4",
		"status": "firing",
		"groupLabels": {"alertname": "HighErrorRate"},
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "HighErrorRate", "severity": "warning"},
			"annotations": {"description": "Error rate above 5%"},
			"generatorURL": "http://prometheus.example.com/graph"
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/alertmanager", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for Stoat delivery")
		case <-time.After(50 * time.Millisecond):
			msgs := stoat.getMessages()
			if len(msgs) > 0 {
				if msgs[0].ChannelID != "warn-ch" {
					t.Errorf("channel = %q, want %q", msgs[0].ChannelID, "warn-ch")
				}
				if !strings.Contains(msgs[0].Content, "HighErrorRate") {
					t.Error("message content missing alert name")
				}
				return
			}
		}
	}
}

func TestIntegration_GatusAutoDetect(t *testing.T) {
	stoat := newFakeStoat()
	defer stoat.close()

	met := metrics.New()
	stoatSender := sender.NewStoatSender(stoat.server.URL, "test-token", &http.Client{})
	q := queue.New(stoatSender, 10, 3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	reg := handler.NewRegistry()
	reg.Register(handler.NewGatusHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))

	srv := server.New(&server.Config{
		MaxBodySize: 1048576,
		RateLimit:   100,
	}, reg, q, met)
	srv.SetReady(true)

	body := `{
		"type": "alert-triggered",
		"endpoint": {
			"name": "Sonarr",
			"group": "Media",
			"conditions": ["[STATUS] == 200"]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for Stoat delivery")
		case <-time.After(50 * time.Millisecond):
			msgs := stoat.getMessages()
			if len(msgs) > 0 {
				if msgs[0].ChannelID != "crit-ch" {
					t.Errorf("channel = %q, want %q", msgs[0].ChannelID, "crit-ch")
				}
				if !strings.Contains(msgs[0].Content, "Sonarr") {
					t.Error("message content missing endpoint name")
				}
				return
			}
		}
	}
}

func TestIntegration_AuthBlocks(t *testing.T) {
	stoat := newFakeStoat()
	defer stoat.close()

	met := metrics.New()
	stoatSender := sender.NewStoatSender(stoat.server.URL, "test-token", &http.Client{})
	q := queue.New(stoatSender, 10, 3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)

	reg := handler.NewRegistry()
	reg.Register(handler.NewGrafanaHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))

	srv := server.New(&server.Config{
		MaxBodySize:   1048576,
		RateLimit:     100,
		WebhookSecret: "secret-123",
	}, reg, q, met)
	srv.SetReady(true)

	body := `{"alerts": [{"status": "firing", "labels": {"alertname": "Test", "severity": "warning"}, "annotations": {}, "generatorURL": ""}]}`

	// Without auth — rejected
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("without auth: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// With auth — accepted
	req = httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-123")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("with auth: status = %d, want %d", w.Code, http.StatusAccepted)
	}
}
