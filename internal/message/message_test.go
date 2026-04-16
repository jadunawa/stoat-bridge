package message

import (
	"strings"
	"testing"
)

func TestResolveEmoji(t *testing.T) {
	tests := []struct {
		status   string
		severity string
		want     string
	}{
		{"firing", "critical", "\U0001f534"},
		{"firing", "warning", "\U0001f7e0"},
		{"resolved", "critical", "\U0001f7e2"},
		{"resolved", "warning", "\U0001f7e2"},
		{"firing", "unknown", "\U0001f7e0"},
		{"firing", "", "\U0001f7e0"},
	}
	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.severity, func(t *testing.T) {
			got := ResolveEmoji(tt.status, tt.severity)
			if got != tt.want {
				t.Errorf("ResolveEmoji(%q, %q) = %q, want %q", tt.status, tt.severity, got, tt.want)
			}
		})
	}
}

func TestResolveSeverityDisplay(t *testing.T) {
	tests := []struct {
		severity string
		want     string
	}{
		{"critical", "High"},
		{"warning", "Warning"},
		{"unknown", "Unknown"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			got := ResolveSeverityDisplay(tt.severity)
			if got != tt.want {
				t.Errorf("ResolveSeverityDisplay(%q) = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

func TestResolveChannelID(t *testing.T) {
	tests := []struct {
		name       string
		severity   string
		criticalCh string
		warningCh  string
		defaultCh  string
		want       string
	}{
		{"critical routes to critical channel", "critical", "crit-ch", "warn-ch", "default-ch", "crit-ch"},
		{"warning routes to warning channel", "warning", "crit-ch", "warn-ch", "default-ch", "warn-ch"},
		{"unknown routes to default channel", "unknown", "crit-ch", "warn-ch", "default-ch", "default-ch"},
		{"empty routes to default channel", "", "crit-ch", "warn-ch", "default-ch", "default-ch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveChannelID(tt.severity, tt.criticalCh, tt.warningCh, tt.defaultCh)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRender_DefaultTemplate(t *testing.T) {
	data := AlertData{
		Emoji:           "\U0001f534",
		Name:            "HighMemoryUsage",
		State:           "Triggered",
		SeverityDisplay: "High",
		URL:             "https://grafana.example.com/alerting/rule/1",
		Description:     "Pod memory usage is above 95%",
	}

	result, err := Render(DefaultTemplate, data, 1900)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "HighMemoryUsage") {
		t.Error("result missing alert name")
	}
	if !strings.Contains(result, "Triggered") {
		t.Error("result missing state")
	}
	if !strings.Contains(result, "High") {
		t.Error("result missing severity display")
	}
	if !strings.Contains(result, "https://grafana.example.com") {
		t.Error("result missing URL")
	}
	if !strings.Contains(result, "Pod memory usage") {
		t.Error("result missing description")
	}
}

func TestRender_WithGroupAndConditions(t *testing.T) {
	data := AlertData{
		Emoji:           "\U0001f534",
		Name:            "Sonarr",
		State:           "Triggered",
		SeverityDisplay: "High",
		Group:           "Media",
		ConditionsStr:   "[STATUS] == 200",
	}

	result, err := Render(DefaultTemplate, data, 1900)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Group: Media") {
		t.Error("result missing group")
	}
	if !strings.Contains(result, "Conditions: [STATUS] == 200") {
		t.Error("result missing conditions")
	}
}

func TestRender_OmitsEmptyFields(t *testing.T) {
	data := AlertData{
		Emoji:           "\U0001f7e2",
		Name:            "TestAlert",
		State:           "Resolved",
		SeverityDisplay: "Warning",
	}

	result, err := Render(DefaultTemplate, data, 1900)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result, "Group:") {
		t.Error("result should not contain Group when empty")
	}
	if strings.Contains(result, "Conditions:") {
		t.Error("result should not contain Conditions when empty")
	}
}

func TestRender_CustomTemplate(t *testing.T) {
	customTmpl := `ALERT: {{.Name}} is {{.State}}`
	data := AlertData{Name: "TestAlert", State: "Triggered"}

	result, err := Render(customTmpl, data, 1900)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "ALERT: TestAlert is Triggered" {
		t.Errorf("got %q, want %q", result, "ALERT: TestAlert is Triggered")
	}
}

func TestRender_Truncation(t *testing.T) {
	data := AlertData{
		Emoji:           "\U0001f534",
		Name:            "TestAlert",
		State:           "Triggered",
		SeverityDisplay: "High",
		Description:     strings.Repeat("x", 2000),
	}

	result, err := Render(DefaultTemplate, data, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(result, "...(truncated)") {
		t.Error("truncated result should end with ...(truncated)")
	}
	withoutSuffix := strings.TrimSuffix(result, "\n...(truncated)")
	if len([]rune(withoutSuffix)) != 100 {
		t.Errorf("truncated to %d runes, want 100", len([]rune(withoutSuffix)))
	}
}

func TestRender_InvalidTemplate(t *testing.T) {
	_, err := Render("{{.Invalid", AlertData{}, 1900)
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestRender_Truncation_MultiByte(t *testing.T) {
	emoji := "\U0001f534" // red circle: 4 bytes, 1 rune
	data := AlertData{
		Name:  strings.Repeat(emoji, 50), // 50 runes, 200 bytes
		State: "Triggered",
	}

	result, err := Render("{{.Name}}", data, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(result, "...(truncated)") {
		t.Errorf("result should be truncated, got %q", result)
	}

	// Verify the truncated content is exactly 10 runes (not 10 bytes)
	withoutSuffix := strings.TrimSuffix(result, "\n...(truncated)")
	if len([]rune(withoutSuffix)) != 10 {
		t.Errorf("truncated to %d runes, want 10", len([]rune(withoutSuffix)))
	}
}
