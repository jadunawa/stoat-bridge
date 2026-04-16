package handler

import (
	"strings"
	"testing"
)

func TestDetect_Gatus(t *testing.T) {
	body := `{"type": "alert-triggered", "endpoint": {"name": "Test"}}`
	name, err := Detect([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "gatus" {
		t.Errorf("got %q, want %q", name, "gatus")
	}
}

func TestDetect_GatusResolved(t *testing.T) {
	body := `{"type": "alert-resolved", "endpoint": {"name": "Test"}}`
	name, err := Detect([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "gatus" {
		t.Errorf("got %q, want %q", name, "gatus")
	}
}

func TestDetect_Alertmanager(t *testing.T) {
	body := `{
		"version": "4",
		"groupLabels": {"alertname": "Test"},
		"alerts": [{"status": "firing", "labels": {}}]
	}`
	name, err := Detect([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "alertmanager" {
		t.Errorf("got %q, want %q", name, "alertmanager")
	}
}

func TestDetect_Grafana(t *testing.T) {
	body := `{
		"version": "1",
		"groupLabels": {"alertname": "Test"},
		"alerts": [{"status": "firing", "labels": {}, "generatorURL": "https://grafana.example.com"}]
	}`
	name, err := Detect([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "grafana" {
		t.Errorf("got %q, want %q", name, "grafana")
	}
}

func TestDetect_GrafanaNoVersion(t *testing.T) {
	body := `{"alerts": [{"status": "firing", "labels": {}}]}`
	name, err := Detect([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "grafana" {
		t.Errorf("got %q, want %q", name, "grafana")
	}
}

func TestDetect_Unknown(t *testing.T) {
	body := `{"foo": "bar"}`
	_, err := Detect([]byte(body))
	if err == nil {
		t.Fatal("expected error for unknown payload")
	}
	if !strings.Contains(err.Error(), "unable to detect") {
		t.Errorf("error should mention detection failure, got: %v", err)
	}
}

func TestDetect_InvalidJSON(t *testing.T) {
	_, err := Detect([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDetect_EmptyObject(t *testing.T) {
	_, err := Detect([]byte("{}"))
	if err == nil {
		t.Fatal("expected error for empty object")
	}
}
