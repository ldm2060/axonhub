package usage_monitor

import (
	"encoding/json"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

var clineWindowOrder = []string{"last5h", "last7d", "last30d"}

// ClineQuotaToParsedFields converts Cline quota data into the generic monitor
// field representation. The model scope is retained so routing can distinguish
// ClinePass-only channels from mixed and direct-credit channels.
func ClineQuotaToParsedFields(q provider_quota.QuotaData) []ParsedField {
	if q.RawData == nil {
		return nil
	}

	fields := make([]ParsedField, 0, len(clineWindowOrder)*2+3)
	if scope, ok := q.RawData["model_scope"].(string); ok {
		fields = append(fields, ParsedField{Key: "model_scope", Label: "Model Scope", Value: scope, Format: string(FieldFormatText)})
	}
	if statusBasis, ok := q.RawData["status_basis"].(string); ok {
		fields = append(fields, ParsedField{Key: "status_basis", Label: "Status Basis", Value: statusBasis, Format: string(FieldFormatText)})
	}

	windows, _ := q.RawData["windows"].(map[string]any)
	for _, key := range clineWindowOrder {
		window, ok := windows[key].(map[string]any)
		if !ok {
			continue
		}

		usagePercent, _ := toFloat(window["usage_percent"])
		label := labelForClineWindow(key)
		fields = append(fields, ParsedField{
			Key:     key + "_used_pct",
			Label:   label + " Usage %",
			Value:   usagePercent,
			Percent: usagePercent,
			Format:  string(FieldFormatPercentage),
			Group:   key,
		})

		var reset any
		if value, ok := window["next_reset_at"].(string); ok {
			reset = value
		}
		fields = append(fields, ParsedField{
			Key:    key + "_reset",
			Label:  label + " Reset At",
			Value:  reset,
			Format: string(FieldFormatDatetime),
			Group:  key,
		})
	}

	if balance, ok := q.RawData["balance"].(map[string]any); ok {
		if rawBalance, ok := balance["raw_balance"]; ok {
			fields = append(fields, ParsedField{Key: "balance", Label: "Balance", Value: rawBalance, Format: string(FieldFormatNumber)})
		}
	}

	return fields
}

func ClineQuotaRawJSON(q provider_quota.QuotaData) string {
	if q.RawData == nil {
		return "{}"
	}
	data, err := json.Marshal(q.RawData)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func labelForClineWindow(key string) string {
	switch key {
	case "last5h":
		return "5h"
	case "last7d":
		return "7d"
	case "last30d":
		return "30d"
	default:
		return key
	}
}
