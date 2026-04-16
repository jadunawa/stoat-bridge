package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jadunawa/stoat-bridge/internal/message"
)

type gatusPayload struct {
	Type     string        `json:"type"`
	Endpoint gatusEndpoint `json:"endpoint"`
}

type gatusEndpoint struct {
	Name       string   `json:"name"`
	Group      string   `json:"group"`
	URL        string   `json:"url"`
	Conditions []string `json:"conditions"`
}

type GatusHandler struct {
	opts     Options
	template string
}

func NewGatusHandler(opts Options) *GatusHandler {
	tmpl := opts.Template
	if tmpl == "" {
		tmpl = message.DefaultTemplate
	}
	return &GatusHandler{opts: opts, template: tmpl}
}

func (h *GatusHandler) Name() string { return "gatus" }

func (h *GatusHandler) Parse(r *http.Request) ([]message.Message, error) {
	var payload gatusPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var state string
	var status string
	switch payload.Type {
	case "alert-triggered":
		state = "Triggered"
		status = "firing"
	case "alert-resolved":
		state = "Resolved"
		status = "resolved"
	default:
		return nil, nil // Unknown alert type, ignore
	}

	severity := "critical"

	var conditionsStr string
	if len(payload.Endpoint.Conditions) > 0 {
		conditionsStr = strings.Join(payload.Endpoint.Conditions, ", ")
	}

	data := message.AlertData{
		Emoji:           message.ResolveEmoji(status, severity),
		Name:            payload.Endpoint.Name,
		State:           state,
		Severity:        severity,
		SeverityDisplay: message.ResolveSeverityDisplay(severity),
		Group:           payload.Endpoint.Group,
		ConditionsStr:   conditionsStr,
		Source:          "gatus",
	}

	content, err := message.Render(h.template, data, h.opts.MaxMessageLength)
	if err != nil {
		return nil, fmt.Errorf("template render error: %w", err)
	}

	return []message.Message{{ChannelID: h.opts.CriticalChannelID, Content: content}}, nil
}
