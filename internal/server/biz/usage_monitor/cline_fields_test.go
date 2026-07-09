package usage_monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func TestClineQuotaToParsedFields(t *testing.T) {
	quota := provider_quota.QuotaData{
		RawData: map[string]any{
			"model_scope": "cline_pass_only",
			"windows": map[string]any{
				"last5h": map[string]any{"usage_percent": 50.0, "next_reset_at": "2099-01-01T00:00:00Z"},
			},
			"balance": map[string]any{"raw_balance": int64(42)},
		},
	}

	fields := ClineQuotaToParsedFields(quota)
	assert.Len(t, fields, 4)
	assert.Equal(t, "model_scope", fields[0].Key)
	assert.Equal(t, "last5h_used_pct", fields[1].Key)
	assert.Equal(t, 50.0, fields[1].Percent)
	assert.Equal(t, "last5h_reset", fields[2].Key)
	assert.Equal(t, "2099-01-01T00:00:00Z", fields[2].Value)
	assert.Equal(t, "balance", fields[3].Key)
}

func TestDeriveCline(t *testing.T) {
	reset := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	tests := []struct {
		name   string
		scope  string
		pct    float64
		status string
		ready  bool
	}{
		{name: "pass-only exhaustion blocks routing", scope: "cline_pass_only", pct: 100, status: "exhausted", ready: false},
		{name: "mixed exhaustion remains routable", scope: "mixed", pct: 100, status: "warning", ready: true},
		{name: "unknown exhaustion remains routable", scope: "unknown", pct: 100, status: "warning", ready: true},
		{name: "pass-only threshold warns", scope: "cline_pass_only", pct: 80, status: "warning", ready: true},
		{name: "direct credit remains available", scope: "direct_only", pct: 100, status: "available", ready: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			derived := DeriveQuotaStatus("cline", []ParsedField{
				{Key: "model_scope", Value: tt.scope, Format: string(FieldFormatText)},
				{Key: "last5h_used_pct", Value: tt.pct, Percent: tt.pct, Format: string(FieldFormatPercentage)},
				{Key: "last5h_reset", Value: reset, Format: string(FieldFormatDatetime)},
			})
			assert.Equal(t, tt.status, derived.Status)
			assert.Equal(t, tt.ready, derived.Ready)
			if tt.scope != "direct_only" {
				assert.NotNil(t, derived.NextResetAt)
			}
		})
	}
}
