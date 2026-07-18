package usage_monitor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func TestMinimaxQuotaToParsedFields(t *testing.T) {
	quota := provider_quota.QuotaData{RawData: map[string]any{
		"rows": []map[string]any{
			{
				"intervalPercent": 80.0,
				"intervalResetAt": "2026-07-17T17:00:00Z",
				"weeklyPercent":   90.0,
				"weeklyStatus":    "warning",
				"weeklyResetAt":   "2026-07-19T12:00:00Z",
			},
		},
	}}

	fields := MinimaxQuotaToParsedFields(quota)
	require.Len(t, fields, 4)
	require.Equal(t, "interval_used_pct", fields[0].Key)
	require.InDelta(t, 80, fields[0].Percent, 0.001)
	require.Equal(t, "weekly_used_pct", fields[2].Key)
	require.InDelta(t, 90, fields[2].Percent, 0.001)
}

func TestDeriveMinimax(t *testing.T) {
	fields := []ParsedField{
		{Key: "interval_used_pct", Value: float64(80), Percent: 80, Format: string(FieldFormatPercentage)},
		{Key: "weekly_used_pct", Value: float64(100), Percent: 100, Format: string(FieldFormatPercentage)},
	}

	derived := DeriveQuotaStatus("minimax", fields)
	require.Equal(t, "exhausted", derived.Status)
	require.False(t, derived.Ready)
	require.Len(t, derived.Limits, 2)
	require.Equal(t, provider_quota.QuotaLimitTypeToken, derived.Limits[0].Type)
}
