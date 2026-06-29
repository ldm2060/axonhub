package usage_monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func TestOpenCodeGoQuotaToParsedFields_AllWindows(t *testing.T) {
	q := provider_quota.QuotaData{
		ProviderType: "opencode_go",
		Status:       "available",
		RawData: map[string]any{
			"plan_type": "go",
			"windows": map[string]any{
				"rolling": map[string]any{"usage_percent": 50.0, "reset_time": "2099-01-01T00:00:00Z", "status": "available"},
				"weekly":  map[string]any{"usage_percent": 30.0, "reset_time": "2099-01-08T00:00:00Z", "status": "available"},
				"monthly": map[string]any{"usage_percent": 20.0, "reset_time": "2099-02-01T00:00:00Z", "status": "available"},
			},
		},
	}

	fields := OpenCodeGoQuotaToParsedFields(q)
	// 3 windows * 2 fields each.
	assert.Len(t, fields, 6)

	// Stable order: rolling, weekly, monthly — pct then reset.
	assert.Equal(t, "rolling_used_pct", fields[0].Key)
	assert.Equal(t, "percentage", fields[0].Format)
	assert.Equal(t, 50.0, fields[0].Percent)
	assert.Equal(t, "rolling", fields[0].Group)

	assert.Equal(t, "rolling_reset", fields[1].Key)
	assert.Equal(t, "datetime", fields[1].Format)
	assert.Equal(t, "2099-01-01T00:00:00Z", fields[1].Value)

	assert.Equal(t, "monthly_used_pct", fields[4].Key)
	assert.Equal(t, 20.0, fields[4].Percent)
}

func TestOpenCodeGoQuotaToParsedFields_PartialWindows(t *testing.T) {
	q := provider_quota.QuotaData{
		RawData: map[string]any{
			"windows": map[string]any{
				"weekly": map[string]any{"usage_percent": 85.0, "reset_time": "2099-01-08T00:00:00Z"},
			},
		},
	}

	fields := OpenCodeGoQuotaToParsedFields(q)
	assert.Len(t, fields, 2)
	assert.Equal(t, "weekly_used_pct", fields[0].Key)
	assert.Equal(t, 85.0, fields[0].Percent)
}

func TestOpenCodeGoQuotaToParsedFields_NoWindows(t *testing.T) {
	assert.Nil(t, OpenCodeGoQuotaToParsedFields(provider_quota.QuotaData{RawData: map[string]any{"plan_type": "go"}}))
	assert.Nil(t, OpenCodeGoQuotaToParsedFields(provider_quota.QuotaData{}))
}

func TestOpenCodeGoQuotaRawJSON(t *testing.T) {
	q := provider_quota.QuotaData{
		RawData: map[string]any{"plan_type": "go"},
	}
	s := OpenCodeGoQuotaRawJSON(q)
	assert.Contains(t, s, "plan_type")
	assert.Contains(t, s, "go")

	assert.Equal(t, "{}", OpenCodeGoQuotaRawJSON(provider_quota.QuotaData{}))
}
