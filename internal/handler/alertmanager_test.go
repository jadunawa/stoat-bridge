package handler

import (
	"strings"
	"testing"
)

func amOpts() Options {
	return Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}
}

func TestAlertmanagerHandler_Name(t *testing.T) {
	h := NewAlertmanagerHandler(amOpts())
	if h.Name() != "alertmanager" {
		t.Errorf("Name() = %q, want %q", h.Name(), "alertmanager")
	}
}

func TestAlertmanagerHandler_FiringCritical(t *testing.T) {
	body := `{
		"version": "4",
		"status": "firing",
		"groupLabels": {"alertname": "HighErrorRate"},
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "HighErrorRate", "severity": "critical"},
			"annotations": {"summary": "Error rate above 5%", "description": "Current error rate: 5.3%"},
			"startsAt": "2026-04-15T10:30:00Z",
			"endsAt": "0001-01-01T00:00:00Z",
			"generatorURL": "http://prometheus.example.com/graph?g0.expr=rate",
			"fingerprint": "abc123"
		}]
	}`

	h := NewAlertmanagerHandler(amOpts())
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
	if !strings.Contains(msgs[0].Content, "HighErrorRate") {
		t.Error("content missing alert name")
	}
	if !strings.Contains(msgs[0].Content, "Triggered") {
		t.Error("content missing Triggered state")
	}
}

func TestAlertmanagerHandler_Resolved(t *testing.T) {
	body := `{
		"version": "4",
		"status": "resolved",
		"groupLabels": {"alertname": "HighErrorRate"},
		"alerts": [{
			"status": "resolved",
			"labels": {"alertname": "HighErrorRate", "severity": "critical"},
			"annotations": {"summary": "Resolved"},
			"startsAt": "2026-04-15T10:30:00Z",
			"endsAt": "2026-04-15T11:00:00Z",
			"generatorURL": "http://prometheus.example.com/graph",
			"fingerprint": "abc123"
		}]
	}`

	h := NewAlertmanagerHandler(amOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(msgs[0].Content, "Resolved") {
		t.Error("content missing Resolved state")
	}
}

func TestAlertmanagerHandler_UsesDescriptionOverSummary(t *testing.T) {
	body := `{
		"version": "4",
		"status": "firing",
		"groupLabels": {},
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "Test", "severity": "warning"},
			"annotations": {"summary": "Summary text", "description": "Description text"},
			"generatorURL": ""
		}]
	}`

	h := NewAlertmanagerHandler(amOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(msgs[0].Content, "Description text") {
		t.Error("content should prefer description annotation")
	}
}

func TestAlertmanagerHandler_FallsBackToSummary(t *testing.T) {
	body := `{
		"version": "4",
		"status": "firing",
		"groupLabels": {},
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "Test", "severity": "warning"},
			"annotations": {"summary": "Summary text"},
			"generatorURL": ""
		}]
	}`

	h := NewAlertmanagerHandler(amOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(msgs[0].Content, "Summary text") {
		t.Error("content should fall back to summary when description is empty")
	}
}

func TestAlertmanagerHandler_InvalidJSON(t *testing.T) {
	h := NewAlertmanagerHandler(amOpts())
	_, err := h.Parse(newRequest("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAlertmanagerHandler_MultipleAlerts(t *testing.T) {
	body := `{
		"version": "4",
		"status": "firing",
		"groupLabels": {},
		"alerts": [
			{"status": "firing", "labels": {"alertname": "A", "severity": "warning"}, "annotations": {}, "generatorURL": ""},
			{"status": "resolved", "labels": {"alertname": "B", "severity": "critical"}, "annotations": {}, "generatorURL": ""}
		]
	}`

	h := NewAlertmanagerHandler(amOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
}
