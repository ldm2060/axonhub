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
