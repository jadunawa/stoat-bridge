package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

type grafanaPayload struct {
	Alerts []grafanaAlert `json:"alerts"`
}

type grafanaAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	GeneratorURL string            `json:"generatorURL"`
}

type GrafanaHandler struct {
	opts     Options
	template string
}

func NewGrafanaHandler(opts Options) *GrafanaHandler {
	tmpl := opts.Template
	if tmpl == "" {
		tmpl = message.DefaultTemplate
	}
	return &GrafanaHandler{opts: opts, template: tmpl}
}

func (h *GrafanaHandler) Name() string { return "grafana" }

func (h *GrafanaHandler) Parse(r *http.Request) ([]message.Message, error) {
	var payload grafanaPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var msgs []message.Message
	for _, alert := range payload.Alerts {
		severity := alert.Labels["severity"]
		if severity == "" {
			severity = "warning"
		}

		state := "Triggered"
		if alert.Status == "resolved" {
			state = "Resolved"
		}

		data := message.AlertData{
			Emoji:           message.ResolveEmoji(alert.Status, severity),
			Name:            alert.Labels["alertname"],
			State:           state,
			Severity:        severity,
			SeverityDisplay: message.ResolveSeverityDisplay(severity),
			Description:     alert.Annotations["description"],
			URL:             alert.GeneratorURL,
			Source:          "grafana",
		}

		content, err := message.Render(h.template, data, h.opts.MaxMessageLength)
		if err != nil {
			return nil, fmt.Errorf("template render error: %w", err)
		}

		channelID := message.ResolveChannelID(severity, h.opts.CriticalChannelID, h.opts.WarningChannelID, h.opts.DefaultChannelID)
		msgs = append(msgs, message.Message{ChannelID: channelID, Content: content})
	}

	return msgs, nil
}
