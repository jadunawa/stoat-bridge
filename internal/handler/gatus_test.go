package handler

import (
	"strings"
	"testing"
)

func gatusOpts() Options {
	return Options{
		CriticalChannelID: "crit-ch",
		WarningChannelID:  "warn-ch",
		DefaultChannelID:  "default-ch",
		MaxMessageLength:  1900,
	}
}

func TestGatusHandler_Name(t *testing.T) {
	h := NewGatusHandler(gatusOpts())
	if h.Name() != "gatus" {
		t.Errorf("Name() = %q, want %q", h.Name(), "gatus")
	}
}

func TestGatusHandler_AlertTriggered(t *testing.T) {
	body := `{
		"type": "alert-triggered",
		"endpoint": {
			"name": "Sonarr",
			"group": "Media",
			"url": "http://sonarr:8989",
			"conditions": ["[STATUS] == 200", "[RESPONSE_TIME] < 1000"]
		}
	}`

	h := NewGatusHandler(gatusOpts())
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
	if !strings.Contains(msgs[0].Content, "Sonarr") {
		t.Error("content missing endpoint name")
	}
	if !strings.Contains(msgs[0].Content, "Triggered") {
		t.Error("content missing Triggered state")
	}
	if !strings.Contains(msgs[0].Content, "Group: Media") {
		t.Error("content missing group")
	}
	if !strings.Contains(msgs[0].Content, "[STATUS] == 200") {
		t.Error("content missing conditions")
	}
}

func TestGatusHandler_AlertResolved(t *testing.T) {
	body := `{
		"type": "alert-resolved",
		"endpoint": {
			"name": "Sonarr",
			"group": "Media",
			"url": "http://sonarr:8989",
			"conditions": ["[STATUS] == 200"]
		}
	}`

	h := NewGatusHandler(gatusOpts())
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

func TestGatusHandler_NoGroup(t *testing.T) {
	body := `{
		"type": "alert-triggered",
		"endpoint": {
			"name": "External API",
			"conditions": ["[STATUS] == 200"]
		}
	}`

	h := NewGatusHandler(gatusOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(msgs[0].Content, "Group:") {
		t.Error("content should not contain Group when empty")
	}
}

func TestGatusHandler_EmptyConditions(t *testing.T) {
	body := `{
		"type": "alert-triggered",
		"endpoint": {
			"name": "Test",
			"conditions": []
		}
	}`

	h := NewGatusHandler(gatusOpts())
	msgs, err := h.Parse(newRequest(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(msgs[0].Content, "Conditions:") {
		t.Error("content should not contain Conditions when empty")
	}
}

func TestGatusHandler_UnknownAlertType(t *testing.T) {
	body := `{
		"type": "unknown-type",
		"endpoint": {"name": "Test"}
	}`

	h := NewGatusHandler(gatusOpts())
	_, err := h.Parse(newRequest(body))
	if err == nil {
		t.Fatal("expected error for unknown alert type")
	}
}

func TestGatusHandler_InvalidJSON(t *testing.T) {
	h := NewGatusHandler(gatusOpts())
	_, err := h.Parse(newRequest("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
