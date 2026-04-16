package handler

import (
	"encoding/json"
	"fmt"
)

// Detect inspects a raw JSON payload and returns the handler name.
// Detection order: Gatus (most distinct) → Alertmanager (version "4") → Grafana (fallback with alerts array).
func Detect(body []byte) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Gatus: has "type" and "endpoint" fields
	if _, hasType := raw["type"]; hasType {
		if _, hasEndpoint := raw["endpoint"]; hasEndpoint {
			return "gatus", nil
		}
	}

	// Alertmanager vs Grafana: both can have "groupLabels" and "alerts".
	// Alertmanager uses version "4", Grafana uses version "1".
	if versionRaw, hasVersion := raw["version"]; hasVersion {
		var version string
		if err := json.Unmarshal(versionRaw, &version); err == nil {
			if version == "4" {
				return "alertmanager", nil
			}
		}
	}

	// Fallback: has "alerts" array → assume Grafana
	if _, hasAlerts := raw["alerts"]; hasAlerts {
		return "grafana", nil
	}

	return "", fmt.Errorf("unable to detect webhook source — use /grafana, /gatus, or /alertmanager instead")
}
