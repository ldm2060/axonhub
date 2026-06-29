package usage_monitor

import (
	"encoding/json"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// opencodeGoWindowOrder is the stable order in which OpenCode Go usage windows
// are emitted as parsed fields, matching the deriver and the monitor template.
var opencodeGoWindowOrder = []string{"rolling", "weekly", "monthly"}

// OpenCodeGoQuotaToParsedFields converts an OpenCode Go QuotaData (produced by
// provider_quota.OpenCodeGoQuotaChecker) into ParsedFields consumable by the
// generic usage-monitor storage + display path and DeriveQuotaStatus.
//
// For each usage window (rolling/weekly/monthly) it emits two fields:
//   - <window>_used_pct  (percentage, 0-100)
//   - <window>_reset     (datetime, RFC3339)
//
// The checker stores per-window raw data under QuotaData.RawData["windows"].
func OpenCodeGoQuotaToParsedFields(q provider_quota.QuotaData) []ParsedField {
	if q.RawData == nil {
		return nil
	}

	windowsAny, ok := q.RawData["windows"]
	if !ok {
		return nil
	}

	windows, ok := windowsAny.(map[string]any)
	if !ok {
		return nil
	}

	fields := make([]ParsedField, 0, len(opencodeGoWindowOrder)*2)
	for _, key := range opencodeGoWindowOrder {
		wAny, ok := windows[key]
		if !ok {
			continue
		}
		w, ok := wAny.(map[string]any)
		if !ok {
			continue
		}

		usagePercent, _ := toFloat(w["usage_percent"])

		fields = append(fields, ParsedField{
			Key:     key + "_used_pct",
			Label:   labelForOpenCodeGoWindow(key) + " Usage %",
			Value:   usagePercent,
			Percent: usagePercent,
			Format:  string(FieldFormatPercentage),
			Group:   key,
		})

		var resetVal any
		if s, ok := w["reset_time"].(string); ok {
			resetVal = s
		}
		fields = append(fields, ParsedField{
			Key:    key + "_reset",
			Label:  labelForOpenCodeGoWindow(key) + " Reset At",
			Value:  resetVal,
			Format: string(FieldFormatDatetime),
			Group:  key,
		})
	}

	return fields
}

// OpenCodeGoQuotaRawJSON returns the checker's RawData as a JSON string for the
// PollData.Raw field (used by the degraded last_poll_data display path).
func OpenCodeGoQuotaRawJSON(q provider_quota.QuotaData) string {
	if q.RawData == nil {
		return "{}"
	}
	b, err := json.Marshal(q.RawData)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func labelForOpenCodeGoWindow(key string) string {
	switch key {
	case "rolling":
		return "Rolling"
	case "weekly":
		return "Weekly"
	case "monthly":
		return "Monthly"
	default:
		return key
	}
}
