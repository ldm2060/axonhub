package usage_monitor

import (
	"encoding/json"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// xaiSubscriptionWindowOrder is the stable order in which xAI subscription billing
// windows are emitted as parsed fields, matching the deriver and the monitor template.
var xaiSubscriptionWindowOrder = []string{"weekly", "monthly"}

// XaiSubscriptionQuotaToParsedFields converts an xAI subscription QuotaData into
// ParsedFields consumable by the generic usage-monitor storage + display path
// and DeriveQuotaStatus.
//
// For each billing window (weekly/monthly) it emits two fields:
//   - <window>_used_pct  (percentage, 0-100)
//   - <window>_reset     (datetime, RFC3339)
//
// The checker stores per-window raw data under QuotaData.RawData["billing"].
func XaiSubscriptionQuotaToParsedFields(q provider_quota.QuotaData) []ParsedField {
	if q.RawData == nil {
		return nil
	}

	billingAny, ok := q.RawData["billing"]
	if !ok {
		return nil
	}

	billing, ok := billingAny.(map[string]any)
	if !ok {
		return nil
	}

	fields := make([]ParsedField, 0, len(xaiSubscriptionWindowOrder)*2)
	for _, key := range xaiSubscriptionWindowOrder {
		wAny, ok := billing[key]
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
			Label:   labelForXaiSubscriptionWindow(key) + " Usage %",
			Value:   usagePercent,
			Percent: usagePercent,
			Format:  string(FieldFormatPercentage),
			Group:   key,
		})

		var resetVal any
		if s, ok := w["reset_at"].(string); ok && s != "" {
			resetVal = s
		}
		fields = append(fields, ParsedField{
			Key:    key + "_reset",
			Label:  labelForXaiSubscriptionWindow(key) + " Reset At",
			Value:  resetVal,
			Format: string(FieldFormatDatetime),
			Group:  key,
		})
	}

	if plan, ok := q.RawData["plan_type"].(string); ok && plan != "" {
		fields = append(fields, ParsedField{
			Key:    "plan_type",
			Label:  "Plan",
			Value:  plan,
			Format: string(FieldFormatText),
		})
	}

	return fields
}

// XaiSubscriptionQuotaRawJSON returns the checker's RawData as a JSON string for
// the PollData.Raw field (used by the degraded last_poll_data display path).
func XaiSubscriptionQuotaRawJSON(q provider_quota.QuotaData) string {
	if q.RawData == nil {
		return "{}"
	}
	b, err := json.Marshal(q.RawData)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func labelForXaiSubscriptionWindow(key string) string {
	switch key {
	case "weekly":
		return "Weekly"
	case "monthly":
		return "Monthly"
	default:
		return key
	}
}
