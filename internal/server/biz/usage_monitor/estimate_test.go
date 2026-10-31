package usage_monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func TestApplyEstimateConfig_PercentageField(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "pro"},
		{Key: "credits_pct", Format: "percentage", Percent: 45},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "credits_pct", Window: EstimateWindowWeekly}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	assert.InDelta(t, 0.45, limit.UsageRatio, 1e-9)
	assert.Equal(t, EstimateWindowWeekly, limit.Window)
	assert.Equal(t, "available", limit.Status)
}

func TestApplyEstimateConfig_RemainingInverted(t *testing.T) {
	fields := []ParsedField{
		{Key: "credits_pct", Format: "percentage", Percent: 70},
	}
	derived := DeriveQuotaStatus("", fields)

	// A "remaining 70%" field means 30% used.
	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "credits_pct", Remaining: true, Window: EstimateWindowWeekly}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.3, derived.Limits[0].UsageRatio, 1e-9)
	assert.Equal(t, "available", derived.Status)
}

func TestApplyEstimateConfig_FractionRawValue(t *testing.T) {
	fields := []ParsedField{
		{Key: "utilization", Format: "fraction", Value: 0.9},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "utilization", Window: EstimateWindowDaily}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.9, derived.Limits[0].UsageRatio, 1e-9)
	assert.Equal(t, "warning", derived.Status)
	assert.True(t, derived.Ready)
}

func TestApplyEstimateConfig_ExhaustedThresholds(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 100},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "used_pct", Window: EstimateWindow5h}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.Equal(t, "exhausted", derived.Limits[0].Status)
	assert.Equal(t, "exhausted", derived.Status)
	assert.False(t, derived.Ready)
}

func TestApplyEstimateConfig_MissingFieldFallsBack(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 50},
	}
	derived := DeriveQuotaStatus("", fields)
	genericRatio := derived.Limits[0].UsageRatio

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "nope", Window: EstimateWindowWeekly}, &derived)

	// The generic max-ratio limit survives untouched.
	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, genericRatio, derived.Limits[0].UsageRatio, 1e-9)
	assert.Empty(t, derived.Limits[0].Window)
}

func TestApplyEstimateConfig_WindowPeriodStart(t *testing.T) {
	// earliestDatetime only considers future datetimes, so the fixtures do too.
	reset := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "used_pct", Window: EstimateWindow5h}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	require.NotNil(t, limit.PeriodStart)
	require.NotNil(t, limit.NextResetAt)
	// A fixed-length window starts window-length before its reset.
	assert.Equal(t, reset.Add(-5*time.Hour), *limit.PeriodStart)
	assert.Equal(t, reset, *limit.NextResetAt)
	assert.Equal(t, reset, *derived.NextResetAt)
}

func TestApplyEstimateConfig_MonthlyPeriodStartClamped(t *testing.T) {
	// March 31 resets to a month whose previous month has fewer days.
	reset := time.Date(2027, 3, 31, 8, 0, 0, 0, time.UTC)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "used_pct", Window: EstimateWindowMonthly}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	require.NotNil(t, limit.PeriodStart)
	assert.Equal(t, EstimateWindowMonthly, limit.Window)
	// Feb 31 does not exist: clamped to Feb 28 at the same wall clock.
	assert.Equal(t, time.Date(2027, 2, 28, 8, 0, 0, 0, time.UTC), *limit.PeriodStart)
}

func TestApplyEstimateConfig_ResetFieldSelection(t *testing.T) {
	early := time.Now().UTC().Add(10 * 24 * time.Hour).Truncate(time.Second)
	cycle := time.Now().UTC().Add(22 * 24 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "next_invoice", Format: "datetime", Value: early.Format(time.RFC3339)},
		{Key: "cycle_reset", Format: "datetime", Value: cycle.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	// Without an explicit reset field the earliest future datetime wins.
	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "used_pct", Window: EstimateWindowWeekly}, &derived)
	require.Len(t, derived.Limits, 1)
	require.NotNil(t, derived.Limits[0].NextResetAt)
	assert.Equal(t, early, *derived.Limits[0].NextResetAt)

	// With one, the named field wins over the earliest.
	derived = DeriveQuotaStatus("", fields)
	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "used_pct", ResetFieldKey: "cycle_reset", Window: EstimateWindowWeekly}, &derived)
	require.Len(t, derived.Limits, 1)
	require.NotNil(t, derived.Limits[0].NextResetAt)
	assert.Equal(t, cycle, *derived.Limits[0].NextResetAt)
}

func TestApplyEstimateConfig_InvalidRatioFallsBack(t *testing.T) {
	// A plain number field above 1 is not a usable ratio.
	fields := []ParsedField{
		{Key: "credits", Format: "number", Value: 300.0},
	}
	derived := DeriveQuotaStatus("", fields)
	origLimits := derived.Limits

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "credits", Window: EstimateWindowWeekly}, &derived)

	assert.Equal(t, origLimits, derived.Limits)
}

func TestEstimateConfig_MapRoundTrip(t *testing.T) {
	cfg := EstimateConfig{FieldKey: "credits_pct", Remaining: true, ResetFieldKey: "cycle_reset", Window: EstimateWindowMonthly}

	decoded, err := EstimateConfigFromMap(cfg.ToMap())
	require.NoError(t, err)
	require.NotNil(t, decoded)
	assert.Equal(t, cfg, *decoded)

	// Nil and keyless maps mean "not configured".
	nilCfg, err := EstimateConfigFromMap(nil)
	require.NoError(t, err)
	assert.Nil(t, nilCfg)
	emptyCfg, err := EstimateConfigFromMap(map[string]any{"window": EstimateWindowWeekly})
	require.NoError(t, err)
	assert.Nil(t, emptyCfg)
}

func TestValidateEstimateConfig(t *testing.T) {
	keys := []string{"used_pct", "cycle_reset"}

	// Valid config.
	assert.NoError(t, ValidateEstimateConfig(EstimateConfig{FieldKey: "used_pct", Window: EstimateWindowWeekly}, keys))

	// Unknown window.
	err := ValidateEstimateConfig(EstimateConfig{FieldKey: "used_pct", Window: "fortnight"}, keys)
	assert.ErrorContains(t, err, "window")

	// Unknown field key.
	err = ValidateEstimateConfig(EstimateConfig{FieldKey: "nope", Window: EstimateWindowWeekly}, keys)
	assert.ErrorContains(t, err, "fieldKey")

	// Unknown reset field key.
	err = ValidateEstimateConfig(EstimateConfig{FieldKey: "used_pct", ResetFieldKey: "nope", Window: EstimateWindowWeekly}, keys)
	assert.ErrorContains(t, err, "resetFieldKey")

	// Empty field key.
	err = ValidateEstimateConfig(EstimateConfig{Window: EstimateWindowWeekly}, keys)
	assert.Error(t, err)

	// No display fields known yet: field keys are accepted unchecked.
	assert.NoError(t, ValidateEstimateConfig(EstimateConfig{FieldKey: "anything", Window: EstimateWindow5h}, nil))
}

func TestApplyEstimateConfig_LimitShapesEstimateReady(t *testing.T) {
	// With a PeriodStart present the limit is index-shaped for
	// fillMonitorPeriodQuotaEstimates: Window + PeriodStart stamped so the
	// estimator has a period to query.
	reset := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfig(fields, EstimateConfig{FieldKey: "used_pct", Window: EstimateWindowWeekly}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	assert.Equal(t, provider_quota.QuotaLimitTypeToken, limit.Type)
	assert.NotEmpty(t, limit.Window)
	assert.NotNil(t, limit.PeriodStart)
}
