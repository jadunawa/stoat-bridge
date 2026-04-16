package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jadunawa/stoat-bridge/internal/handler"
	"github.com/jadunawa/stoat-bridge/internal/message"
	"github.com/jadunawa/stoat-bridge/internal/metrics"
)

// mockQueue implements the Enqueuer interface for tests.
type mockQueue struct {
	messages []message.Message
	full     bool
}

func (m *mockQueue) Enqueue(msg message.Message) bool {
	if m.full {
		return false
	}
	m.messages = append(m.messages, msg)
	return true
}

func newTestServer(opts ...func(*Config)) *Server {
	cfg := &Config{
		MaxBodySize: 1048576,
		RateLimit:   100,
	}
	for _, o := range opts {
		o(cfg)
	}

	reg := handler.NewRegistry()
	reg.Register(handler.NewGrafanaHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))
	reg.Register(handler.NewGatusHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))
	reg.Register(handler.NewAlertmanagerHandler(handler.Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}))

	mq := &mockQueue{}
	met := metrics.New()

	s := New(cfg, reg, mq, met)
	return s
}

func TestHealthz(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReadyz_Ready(t *testing.T) {
	s := newTestServer()
	s.SetReady(true)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReadyz_NotReady(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestMetrics(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "stoatbridge_messages_queued_total") {
		t.Error("metrics response missing stoatbridge_messages_queued_total")
	}
}

func TestNamedEndpoint_Grafana(t *testing.T) {
	s := newTestServer()
	s.SetReady(true)

	body := `{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "TestAlert", "severity": "warning"},
			"annotations": {"description": "test"},
			"generatorURL": "https://grafana.example.com"
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestWebhookEndpoint_AutoDetect(t *testing.T) {
	s := newTestServer()
	s.SetReady(true)

	body := `{"type": "alert-triggered", "endpoint": {"name": "Test", "conditions": []}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestWebhookEndpoint_UnknownPayload(t *testing.T) {
	s := newTestServer()
	s.SetReady(true)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"foo": "bar"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuth_Required(t *testing.T) {
	s := newTestServer(func(cfg *Config) {
		cfg.WebhookSecret = "my-secret"
	})
	s.SetReady(true)

	body := `{"alerts": [{"status": "firing", "labels": {"alertname": "Test"}, "annotations": {}, "generatorURL": ""}]}`
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_ValidSecret(t *testing.T) {
	s := newTestServer(func(cfg *Config) {
		cfg.WebhookSecret = "my-secret"
	})
	s.SetReady(true)

	body := `{"alerts": [{"status": "firing", "labels": {"alertname": "Test", "severity": "warning"}, "annotations": {}, "generatorURL": ""}]}`
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer my-secret")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestAuth_InvalidSecret(t *testing.T) {
	s := newTestServer(func(cfg *Config) {
		cfg.WebhookSecret = "my-secret"
	})
	s.SetReady(true)

	body := `{"alerts": [{"status": "firing", "labels": {"alertname": "Test"}, "annotations": {}, "generatorURL": ""}]}`
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-secret")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBodySizeLimit(t *testing.T) {
	s := newTestServer(func(cfg *Config) {
		cfg.MaxBodySize = 100
	})
	s.SetReady(true)

	largeBody := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusAccepted {
		t.Error("expected non-202 for oversized body")
	}
}

func TestWrongMethod(t *testing.T) {
	s := newTestServer()
	s.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/grafana", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestQueueFull_Returns202(t *testing.T) {
	s := newTestServer()
	s.SetReady(true)
	mq, ok := s.queue.(*mockQueue)
	if !ok {
		t.Fatal("queue is not *mockQueue")
	}
	mq.full = true

	body := `{"alerts": [{"status": "firing", "labels": {"alertname": "Test", "severity": "warning"}, "annotations": {}, "generatorURL": ""}]}`
	req := httptest.NewRequest(http.MethodPost, "/grafana", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}
