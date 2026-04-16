package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

type alertmanagerPayload struct {
	Version     string              `json:"version"`
	Status      string              `json:"status"`
	GroupLabels map[string]string   `json:"groupLabels"`
	Alerts      []alertmanagerAlert `json:"alerts"`
}

type alertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type AlertmanagerHandler struct {
	opts     Options
	template string
}

func NewAlertmanagerHandler(opts Options) *AlertmanagerHandler {
	tmpl := opts.Template
	if tmpl == "" {
		tmpl = message.DefaultTemplate
	}
	return &AlertmanagerHandler{opts: opts, template: tmpl}
}

func (h *AlertmanagerHandler) Name() string { return "alertmanager" }

func (h *AlertmanagerHandler) Parse(r *http.Request) ([]message.Message, error) {
	var payload alertmanagerPayload
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

		description := alert.Annotations["description"]
		if description == "" {
			description = alert.Annotations["summary"]
		}

		data := message.AlertData{
			Emoji:           message.ResolveEmoji(alert.Status, severity),
			Name:            alert.Labels["alertname"],
			State:           state,
			Severity:        severity,
			SeverityDisplay: message.ResolveSeverityDisplay(severity),
			Description:     description,
			URL:             alert.GeneratorURL,
			Source:          "alertmanager",
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
