package handler

import (
	"net/http"
	"strings"
	"testing"
)

func newRequest(body string) *http.Request {
	r, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func grafanaOpts() Options {
	return Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}
}

func TestGrafanaHandler_Name(t *testing.T) {
	h := NewGrafanaHandler(grafanaOpts())
	if h.Name() != "grafana" {
		t.Errorf("Name() = %q, want %q", h.Name(), "grafana")
	}
}

func TestGrafanaHandler_FiringCritical(t *testing.T) {
	body := `{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "HighMemory", "severity": "critical"},
			"annotations": {"description": "Memory above 95%"},
			"generatorURL": "https://grafana.example.com/alerting/rule/1"
		}]
	}`

	h := NewGrafanaHandler(grafanaOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}

	if msgs[0].ChannelID != "crit-ch" {
		t.Errorf("ChannelID = %q, want %q", msgs[0].ChannelID, "crit-ch")
	}
	if !strings.Contains(msgs[0].Content, "HighMemory") {
		t.Error("content missing alert name")
	}
	if !strings.Contains(msgs[0].Content, "Triggered") {
		t.Error("content missing Triggered state")
	}
	if !strings.Contains(msgs[0].Content, "High") {
		t.Error("content missing severity display")
	}
	if !strings.Contains(msgs[0].Content, "https://grafana.example.com") {
		t.Error("content missing generatorURL")
	}
}

func TestGrafanaHandler_FiringWarning(t *testing.T) {
	body := `{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "SlowQuery", "severity": "warning"},
			"annotations": {"description": "Query latency high"},
			"generatorURL": "https://grafana.example.com/alerting/rule/2"
		}]
	}`

	h := NewGrafanaHandler(grafanaOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msgs[0].ChannelID != "warn-ch" {
		t.Errorf("ChannelID = %q, want %q", msgs[0].ChannelID, "warn-ch")
	}
}

func TestGrafanaHandler_Resolved(t *testing.T) {
	body := `{
		"alerts": [{
			"status": "resolved",
			"labels": {"alertname": "HighMemory", "severity": "critical"},
			"annotations": {"description": "Memory back to normal"},
			"generatorURL": "https://grafana.example.com/alerting/rule/1"
		}]
	}`

	h := NewGrafanaHandler(grafanaOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(msgs[0].Content, "Resolved") {
		t.Error("content missing Resolved state")
	}
	if msgs[0].ChannelID != "crit-ch" {
		t.Errorf("ChannelID = %q, want %q", msgs[0].ChannelID, "crit-ch")
	}
}

func TestGrafanaHandler_MultiplAlerts(t *testing.T) {
	body := `{
		"alerts": [
			{"status": "firing", "labels": {"alertname": "Alert1", "severity": "warning"}, "annotations": {}, "generatorURL": ""},
			{"status": "firing", "labels": {"alertname": "Alert2", "severity": "critical"}, "annotations": {}, "generatorURL": ""}
		]
	}`

	h := NewGrafanaHandler(grafanaOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
}

func TestGrafanaHandler_MissingSeverity(t *testing.T) {
	body := `{
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "NoSeverity"},
			"annotations": {},
			"generatorURL": ""
		}]
	}`

	h := NewGrafanaHandler(grafanaOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msgs[0].ChannelID != "warn-ch" {
		t.Errorf("ChannelID = %q, want %q (default to warning)", msgs[0].ChannelID, "warn-ch")
	}
}

func TestGrafanaHandler_EmptyAlerts(t *testing.T) {
	body := `{"alerts": []}`

	h := NewGrafanaHandler(grafanaOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("got %d messages, want 0", len(msgs))
	}
}

func TestGrafanaHandler_InvalidJSON(t *testing.T) {
	h := NewGrafanaHandler(grafanaOpts())
	_, err := h.Parse(newRequest("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
