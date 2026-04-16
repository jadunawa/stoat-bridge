package message

import (
	"bytes"
	"strings"
	"text/template"
)

// Message is the normalized type that flows through the pipeline.
type Message struct {
	ChannelID string
	Content   string
}

// AlertData holds all fields available to message templates.
type AlertData struct {
	Emoji           string
	Name            string
	State           string // "Triggered" or "Resolved"
	Severity        string // raw: "critical", "warning"
	SeverityDisplay string // display: "High", "Warning"
	Description     string
	URL             string
	Source          string // "grafana", "alertmanager", "gatus"
	Group           string
	ConditionsStr   string // pre-joined conditions for template use
}

const DefaultTemplate = `{{.Emoji}} Alert {{.Name}} {{.State}}
Severity: {{.SeverityDisplay}}
{{- if .Group}}
Group: {{.Group}}
{{- end}}
{{- if .ConditionsStr}}
Conditions: {{.ConditionsStr}}
{{- end}}
{{- if .URL}}

{{.URL}}
{{- end}}
{{- if .Description}}

{{.Description}}
{{- end}}`

func Render(tmpl string, data AlertData, maxLength int) (string, error) {
	t, err := template.New("msg").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	result := buf.String()
	runes := []rune(result)
	if maxLength > 0 && len(runes) > maxLength {
		result = string(runes[:maxLength]) + "\n...(truncated)"
	}
	return result, nil
}

func ResolveEmoji(status, severity string) string {
	if status == "resolved" {
		return "\U0001f7e2" // green circle
	}
	if severity == "critical" {
		return "\U0001f534" // red circle
	}
	return "\U0001f7e0" // orange circle
}

func ResolveSeverityDisplay(severity string) string {
	switch severity {
	case "critical":
		return "High"
	case "warning":
		return "Warning"
	case "":
		return ""
	default:
		return strings.ToUpper(severity[:1]) + severity[1:]
	}
}

func ResolveChannelID(severity, criticalCh, warningCh, defaultCh string) string {
	switch severity {
	case "critical":
		return criticalCh
	case "warning":
		return warningCh
	default:
		return defaultCh
	}
}
