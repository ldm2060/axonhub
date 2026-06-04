package usage_monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveQuotaStatus_Generic_Percentage(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 50},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
	assert.Len(t, result.Limits, 1)
	assert.Equal(t, 0.5, result.Limits[0].UsageRatio)
}

func TestDeriveQuotaStatus_Generic_Fraction(t *testing.T) {
	fields := []ParsedField{
		{Key: "utilization", Format: "fraction", Value: 0.9},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "warning", result.Status)
	assert.True(t, result.Ready)
}

func TestDeriveQuotaStatus_Generic_Exhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 100},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

func TestDeriveQuotaStatus_Generic_NoNumericFields(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "pro"},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "unknown", result.Status)
	assert.True(t, result.Ready)
	assert.Empty(t, result.Limits)
}

func TestDeriveQuotaStatus_Generic_MultiplePercentages_Wins(t *testing.T) {
	fields := []ParsedField{
		{Key: "daily_pct", Format: "percentage", Percent: 30},
		{Key: "weekly_pct", Format: "percentage", Percent: 85},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "warning", result.Status)
	assert.Equal(t, 0.85, result.Limits[0].UsageRatio)
}

func TestDeriveQuotaStatus_Generic_MixedFormats(t *testing.T) {
	fields := []ParsedField{
		{Key: "utilization", Format: "fraction", Value: 0.75},
		{Key: "used_pct", Format: "percentage", Percent: 60},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "available", result.Status)
	assert.Equal(t, 0.75, result.Limits[0].UsageRatio)
}

func TestDeriveQuotaStatus_UnknownProviderType_FallsToGeneric(t *testing.T) {
	fields := []ParsedField{
		{Key: "usage_ratio", Format: "fraction", Value: 0.95},
	}
	result := DeriveQuotaStatus("some_new_provider", fields)
	assert.Equal(t, "warning", result.Status)
	assert.True(t, result.Ready)
}

func TestDeriveQuotaStatus_Generic_WithDatetime(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 50},
		{Key: "reset_at", Format: "datetime", Value: "2099-01-01T00:00:00Z"},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "available", result.Status)
	assert.NotNil(t, result.NextResetAt)
	assert.NotNil(t, result.Limits[0].NextResetAt)
}

// Fraction fields with total (e.g. Xunfei: value=35358, total=90000, percent=39.29)
// must use percent/100 as the ratio, not the raw value.
func TestDeriveQuotaStatus_Generic_FractionWithTotal(t *testing.T) {
	fields := []ParsedField{
		{Key: "pkg_usage", Format: "fraction", Value: 35358, Total: 90000, Percent: 39.29},
		{Key: "rp5h", Format: "fraction", Value: 500, Total: 6000, Percent: 8.33},
		{Key: "rpw", Format: "fraction", Value: 9670, Total: 45000, Percent: 21.49},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
	assert.InDelta(t, 0.3929, result.Limits[0].UsageRatio, 0.01)
}

// Fraction with total that is actually exhausted
func TestDeriveQuotaStatus_Generic_FractionWithTotal_Exhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "pkg_usage", Format: "fraction", Value: 90000, Total: 90000, Percent: 100},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// Pure ratio fraction (no total, value is 0-1) still works
func TestDeriveQuotaStatus_Generic_FractionPureRatio(t *testing.T) {
	fields := []ParsedField{
		{Key: "utilization", Format: "fraction", Value: 0.85},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "warning", result.Status)
	assert.True(t, result.Ready)
	assert.Equal(t, 0.85, result.Limits[0].UsageRatio)
}

// Mix of fraction-with-total and pure-ratio fraction picks the worst
func TestDeriveQuotaStatus_Generic_MixedFractionTypes(t *testing.T) {
	fields := []ParsedField{
		{Key: "pkg_usage", Format: "fraction", Value: 35358, Total: 90000, Percent: 39.29},
		{Key: "utilization", Format: "fraction", Value: 0.85},
	}
	result := DeriveQuotaStatus("", fields)
	assert.Equal(t, "warning", result.Status)
	assert.Equal(t, 0.85, result.Limits[0].UsageRatio)
}

// --- Built-in provider tests ---

// Codex: percentage fields where Percent=75 (raw API value), must convert to 0-1 ratio
func TestDeriveQuotaStatus_Codex_Available(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 50},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
		{Key: "plan_type", Format: "text", Value: "pro"},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
	assert.Len(t, result.Limits, 2) // primary + secondary
	assert.InDelta(t, 0.5, result.Limits[0].UsageRatio, 0.01)
	assert.InDelta(t, 0.3, result.Limits[1].UsageRatio, 0.01)
}

func TestDeriveQuotaStatus_Codex_Warning(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 85},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "warning", result.Status)
	assert.True(t, result.Ready)
}

func TestDeriveQuotaStatus_Codex_Exhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 100},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 50},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// Codex with low usage must NOT be exhausted (was a bug where raw 75 >= 1.0)
func TestDeriveQuotaStatus_Codex_LowUsage_NotExhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 75},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 40},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
}

// Codex: limit_reached="true" forces exhausted status regardless of percentages.
func TestDeriveQuotaStatus_Codex_LimitReached_Exhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 50},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
		{Key: "limit_reached", Format: "text", Value: "true"},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// Codex: limit_reached="false" does not override percentage-based status.
func TestDeriveQuotaStatus_Codex_LimitReached_False(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 50},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
		{Key: "limit_reached", Format: "text", Value: "false"},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
}

// Codex: additional rate limits are included in per-window limits.
func TestDeriveQuotaStatus_Codex_AdditionalRateLimits(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 50},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
		{Key: "additional_0_name", Format: "text", Value: "images"},
		{Key: "additional_0_primary_pct", Format: "percentage", Percent: 90},
		{Key: "additional_0_secondary_pct", Format: "percentage", Percent: 40},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "warning", result.Status) // 90% on additional_0_primary triggers warning
	assert.True(t, result.Ready)
	assert.Len(t, result.Limits, 4) // primary + secondary + additional_0 primary + additional_0 secondary
}

// Codex: additional rate limit that is exhausted drives overall status.
func TestDeriveQuotaStatus_Codex_AdditionalRateLimit_Exhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 50},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
		{Key: "additional_0_name", Format: "text", Value: "extended_context"},
		{Key: "additional_0_primary_pct", Format: "percentage", Percent: 100},
		{Key: "additional_0_secondary_pct", Format: "percentage", Percent: 40},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// Codex: pro_lite plan type works correctly.
func TestDeriveQuotaStatus_Codex_ProLite(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 60},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 20},
		{Key: "plan_type", Format: "text", Value: "pro_lite"},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
}

// Codex: no additional rate limits (missing additional_0_name) only returns primary+secondary.
func TestDeriveQuotaStatus_Codex_NoAdditionalLimits(t *testing.T) {
	fields := []ParsedField{
		{Key: "primary_used_pct", Format: "percentage", Percent: 50},
		{Key: "secondary_used_pct", Format: "percentage", Percent: 30},
	}
	result := DeriveQuotaStatus("codex", fields)
	assert.Len(t, result.Limits, 2)
}

// --- GitHub Copilot tests ---

// GitHub Copilot: limited quotas available (Free account with remaining quota).
func TestDeriveQuotaStatus_GitHubCopilot_LimitedQuotasAvailable(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "individual"},
		{Key: "access_type", Format: "text", Value: "copilot_free"},
		{Key: "limited_quotas", Format: "text", Value: map[string]any{"chat": float64(800), "completions": float64(500)}},
		{Key: "monthly_quotas", Format: "text", Value: map[string]any{"chat": float64(1000), "completions": float64(1000)}},
	}
	result := DeriveQuotaStatus("github_copilot", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
	assert.Len(t, result.Limits, 1)
}

// GitHub Copilot: limited quotas exhausted (Free account).
func TestDeriveQuotaStatus_GitHubCopilot_LimitedQuotasExhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "individual"},
		{Key: "access_type", Format: "text", Value: "copilot_free"},
		{Key: "limited_quotas", Format: "text", Value: map[string]any{"chat": float64(0), "completions": float64(500)}},
		{Key: "monthly_quotas", Format: "text", Value: map[string]any{"chat": float64(1000), "completions": float64(1000)}},
	}
	result := DeriveQuotaStatus("github_copilot", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// GitHub Copilot: quota snapshots with percent_remaining (EDU/Premium account).
func TestDeriveQuotaStatus_GitHubCopilot_SnapshotsAvailable(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "business"},
		{Key: "access_type", Format: "text", Value: "copilot_business"},
		{Key: "chat_pct", Format: "percentage", Value: float64(85)},
		{Key: "chat_unlimited", Format: "text", Value: false},
	}
	result := DeriveQuotaStatus("github_copilot", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
}

// GitHub Copilot: quota snapshots exhausted.
func TestDeriveQuotaStatus_GitHubCopilot_SnapshotsExhausted(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "business"},
		{Key: "chat_pct", Format: "percentage", Value: float64(0)},
		{Key: "chat_unlimited", Format: "text", Value: false},
	}
	result := DeriveQuotaStatus("github_copilot", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// GitHub Copilot: unlimited snapshots should be skipped.
func TestDeriveQuotaStatus_GitHubCopilot_UnlimitedSkipped(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "enterprise"},
		{Key: "chat_pct", Format: "percentage", Value: float64(0)},
		{Key: "chat_unlimited", Format: "text", Value: true},
	}
	result := DeriveQuotaStatus("github_copilot", fields)
	// Unlimited should be skipped, so no quota data -> available
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
}

// GitHub Copilot: no quota fields (backward compat) falls back to available.
func TestDeriveQuotaStatus_GitHubCopilot_NoQuotaFields(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "individual"},
		{Key: "access_type", Format: "text", Value: "copilot_pro"},
	}
	result := DeriveQuotaStatus("github_copilot", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
	assert.Empty(t, result.Limits)
}

// Wafer: percentage field where Percent=75, must convert to 0-1 ratio
func TestDeriveQuotaStatus_Wafer_Available(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 50},
		{Key: "remaining", Format: "number", Value: 500},
	}
	result := DeriveQuotaStatus("wafer", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
	assert.InDelta(t, 0.5, result.Limits[0].UsageRatio, 0.01)
}

func TestDeriveQuotaStatus_Wafer_Warning(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 85},
		{Key: "remaining", Format: "number", Value: 150},
	}
	result := DeriveQuotaStatus("wafer", fields)
	assert.Equal(t, "warning", result.Status)
	assert.True(t, result.Ready)
}

func TestDeriveQuotaStatus_Wafer_Exhausted_NoRemaining(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 100},
		{Key: "remaining", Format: "number", Value: 0},
	}
	result := DeriveQuotaStatus("wafer", fields)
	assert.Equal(t, "exhausted", result.Status)
	assert.False(t, result.Ready)
}

// Wafer with 75% usage must NOT be warning (was a bug where raw 75 >= 0.8)
func TestDeriveQuotaStatus_Wafer_75Pct_NotWarning(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 75},
		{Key: "remaining", Format: "number", Value: 250},
	}
	result := DeriveQuotaStatus("wafer", fields)
	assert.Equal(t, "available", result.Status)
	assert.True(t, result.Ready)
}
