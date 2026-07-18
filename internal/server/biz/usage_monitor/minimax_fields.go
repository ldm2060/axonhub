package usage_monitor

import (
	"encoding/json"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func MinimaxQuotaToParsedFields(q provider_quota.QuotaData) []ParsedField {
	if q.RawData == nil {
		return nil
	}

	rows, ok := q.RawData["rows"]
	if !ok {
		return nil
	}

	data, err := json.Marshal(rows)
	if err != nil {
		return nil
	}

	var normalized []map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil || len(normalized) == 0 {
		return nil
	}

	row := normalized[0]
	fields := []ParsedField{
		{Key: "interval_used_pct", Label: "Interval Usage %", Value: row["intervalPercent"], Percent: numberValue(row["intervalPercent"]), Format: string(FieldFormatPercentage), Group: "interval"},
		{Key: "interval_reset", Label: "Interval Reset At", Value: row["intervalResetAt"], Format: string(FieldFormatDatetime), Group: "interval"},
	}

	if status, _ := row["weeklyStatus"].(string); status != "" {
		fields = append(fields,
			ParsedField{Key: "weekly_used_pct", Label: "Weekly Usage %", Value: row["weeklyPercent"], Percent: numberValue(row["weeklyPercent"]), Format: string(FieldFormatPercentage), Group: "weekly"},
			ParsedField{Key: "weekly_reset", Label: "Weekly Reset At", Value: row["weeklyResetAt"], Format: string(FieldFormatDatetime), Group: "weekly"},
		)
	}

	return fields
}

func MinimaxQuotaRawJSON(q provider_quota.QuotaData) string {
	if q.RawData == nil {
		return "{}"
	}
	data, err := json.Marshal(q.RawData)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func numberValue(value any) float64 {
	v, _ := toFloat(value)
	return v
}
