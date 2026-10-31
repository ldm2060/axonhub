package usage_monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func TestApplyEstimateConfigs_PercentageField(t *testing.T) {
	fields := []ParsedField{
		{Key: "plan", Format: "text", Value: "pro"},
		{Key: "credits_pct", Format: "percentage", Percent: 45},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "credits_pct", Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	assert.InDelta(t, 0.45, limit.UsageRatio, 1e-9)
	assert.Equal(t, EstimateWindowWeekly, limit.Window)
	assert.Equal(t, "credits_pct", limit.FieldKey)
	assert.Equal(t, "available", limit.Status)
}

func TestApplyEstimateConfigs_RemainingInverted(t *testing.T) {
	fields := []ParsedField{
		{Key: "credits_pct", Format: "percentage", Percent: 70},
	}
	derived := DeriveQuotaStatus("", fields)

	// A "remaining 70%" field means 30% used.
	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "credits_pct", Remaining: true, Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.3, derived.Limits[0].UsageRatio, 1e-9)
	assert.Equal(t, "available", derived.Status)
}

func TestApplyEstimateConfigs_FractionRawValue(t *testing.T) {
	fields := []ParsedField{
		{Key: "utilization", Format: "fraction", Value: 0.9},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "utilization", Window: EstimateWindowDaily}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.9, derived.Limits[0].UsageRatio, 1e-9)
	assert.Equal(t, "warning", derived.Status)
	assert.True(t, derived.Ready)
}

func TestApplyEstimateConfigs_ExhaustedThresholds(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 100},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "used_pct", Window: EstimateWindow5h}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.Equal(t, "exhausted", derived.Limits[0].Status)
	assert.Equal(t, "exhausted", derived.Status)
	assert.False(t, derived.Ready)
}

func TestApplyEstimateConfigs_MissingFieldFallsBack(t *testing.T) {
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 50},
	}
	derived := DeriveQuotaStatus("", fields)
	genericRatio := derived.Limits[0].UsageRatio

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "nope", Window: EstimateWindowWeekly}}, &derived)

	// The generic max-ratio limit survives untouched.
	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, genericRatio, derived.Limits[0].UsageRatio, 1e-9)
	assert.Empty(t, derived.Limits[0].Window)
}

func TestApplyEstimateConfigs_MultipleWindows(t *testing.T) {
	// The same field priced against both a 5h and a weekly window.
	reset := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 25},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{
		{FieldKey: "used_pct", Window: EstimateWindow5h},
		{FieldKey: "used_pct", Window: EstimateWindowWeekly},
	}, &derived)

	require.Len(t, derived.Limits, 2)
	assert.Equal(t, EstimateWindow5h, derived.Limits[0].Window)
	assert.Equal(t, EstimateWindowWeekly, derived.Limits[1].Window)
	for _, limit := range derived.Limits {
		assert.InDelta(t, 0.25, limit.UsageRatio, 1e-9)
		assert.Equal(t, "used_pct", limit.FieldKey)
	}
}

func TestApplyEstimateConfigs_WindowPeriodStart(t *testing.T) {
	// earliestDatetime only considers future datetimes, so the fixtures do too.
	reset := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "used_pct", Window: EstimateWindow5h}}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	require.NotNil(t, limit.PeriodStart)
	require.NotNil(t, limit.NextResetAt)
	// A fixed-length window starts window-length before its reset.
	assert.Equal(t, reset.Add(-5*time.Hour), *limit.PeriodStart)
	assert.Equal(t, reset, *limit.NextResetAt)
	assert.Equal(t, reset, *derived.NextResetAt)
}

func TestApplyEstimateConfigs_MonthlyPeriodStartClamped(t *testing.T) {
	// March 31 resets to a month whose previous month has fewer days.
	reset := time.Date(2027, 3, 31, 8, 0, 0, 0, time.UTC)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "used_pct", Window: EstimateWindowMonthly}}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	require.NotNil(t, limit.PeriodStart)
	assert.Equal(t, EstimateWindowMonthly, limit.Window)
	// Feb 31 does not exist: clamped to Feb 28 at the same wall clock.
	assert.Equal(t, time.Date(2027, 2, 28, 8, 0, 0, 0, time.UTC), *limit.PeriodStart)
}

func TestApplyEstimateConfigs_ResetFieldSelection(t *testing.T) {
	early := time.Now().UTC().Add(10 * 24 * time.Hour).Truncate(time.Second)
	cycle := time.Now().UTC().Add(22 * 24 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "next_invoice", Format: "datetime", Value: early.Format(time.RFC3339)},
		{Key: "cycle_reset", Format: "datetime", Value: cycle.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	// Without an explicit reset field the earliest future datetime wins.
	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "used_pct", Window: EstimateWindowWeekly}}, &derived)
	require.Len(t, derived.Limits, 1)
	require.NotNil(t, derived.Limits[0].NextResetAt)
	assert.Equal(t, early, *derived.Limits[0].NextResetAt)

	// With one, the named field wins over the earliest.
	derived = DeriveQuotaStatus("", fields)
	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "used_pct", ResetFieldKey: "cycle_reset", Window: EstimateWindowWeekly}}, &derived)
	require.Len(t, derived.Limits, 1)
	require.NotNil(t, derived.Limits[0].NextResetAt)
	assert.Equal(t, cycle, *derived.Limits[0].NextResetAt)
}

func TestApplyEstimateConfigs_InvalidRatioFallsBack(t *testing.T) {
	// A plain number field above 1 is not a usable ratio.
	fields := []ParsedField{
		{Key: "credits", Format: "number", Value: 300.0},
	}
	derived := DeriveQuotaStatus("", fields)
	origLimits := derived.Limits

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "credits", Window: EstimateWindowWeekly}}, &derived)

	assert.Equal(t, origLimits, derived.Limits)
}

func TestApplyEstimateConfigs_LimitShapesEstimateReady(t *testing.T) {
	// With a PeriodStart present the limit is index-shaped for
	// fillMonitorPeriodQuotaEstimates: Window + PeriodStart stamped so the
	// estimator has a period to query.
	reset := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "used_pct", Format: "percentage", Percent: 40},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "used_pct", Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	assert.Equal(t, provider_quota.QuotaLimitTypeToken, limit.Type)
	assert.NotEmpty(t, limit.Window)
	assert.NotNil(t, limit.PeriodStart)
}

func TestApplyEstimateConfigs_WindowOnlyStampsTemplateRatio(t *testing.T) {
	// Template monitors: a window-only entry stamps the configured window onto
	// the template's windowless limit; ratio, status, and field key stay the
	// template's own, and the routing status is preserved.
	reset := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	derived := QuotaDerivedStatus{
		Status: "available",
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.017, Ready: true, FieldKey: "premium_pct"},
		},
	}

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{Window: EstimateWindowMonthly}}, &derived)

	require.Len(t, derived.Limits, 1)
	limit := derived.Limits[0]
	assert.Equal(t, EstimateWindowMonthly, limit.Window)
	assert.InDelta(t, 0.017, limit.UsageRatio, 1e-9)
	assert.Equal(t, "available", limit.Status)
	assert.True(t, limit.Ready)
	assert.Equal(t, "premium_pct", limit.FieldKey)
	require.NotNil(t, limit.PeriodStart)
	require.NotNil(t, limit.NextResetAt)
	assert.Equal(t, reset, *limit.NextResetAt)
}

func TestApplyEstimateConfigs_MultipleWindowOnlyEntries(t *testing.T) {
	// One windowless template ratio priced against both a 5h and a weekly window.
	reset := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)}}
	derived := QuotaDerivedStatus{
		Status: "available",
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.02, Ready: true},
		},
	}

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{
		{Window: EstimateWindow5h},
		{Window: EstimateWindowWeekly},
	}, &derived)

	require.Len(t, derived.Limits, 2)
	assert.Equal(t, EstimateWindow5h, derived.Limits[0].Window)
	assert.Equal(t, EstimateWindowWeekly, derived.Limits[1].Window)
	for _, limit := range derived.Limits {
		assert.InDelta(t, 0.02, limit.UsageRatio, 1e-9)
	}
}

func TestApplyEstimateConfigs_FieldEntryOnTemplate(t *testing.T) {
	// A field entry on a template monitor replaces the windowless ratio with
	// the selected field's ratio; the template's windowed limits survive.
	reset := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	fields := []ParsedField{
		{Key: "tokens_pct", Format: "percentage", Percent: 85},
		{Key: "reset_at", Format: "datetime", Value: reset.Format(time.RFC3339)},
	}
	windowed := provider_quota.QuotaLimitStatus{Status: "available", UsageRatio: 0.1}.WithWindow("5h", 5*time.Hour)
	derived := QuotaDerivedStatus{
		Status: "available",
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			windowed,
			{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.4, Ready: true},
		},
	}

	ApplyEstimateConfigs(fields, nil, []EstimateConfig{{FieldKey: "tokens_pct", Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 2)
	assert.Equal(t, "5h", derived.Limits[0].Window)
	assert.InDelta(t, 0.1, derived.Limits[0].UsageRatio, 1e-9)
	weekly := derived.Limits[1]
	assert.Equal(t, EstimateWindowWeekly, weekly.Window)
	assert.InDelta(t, 0.85, weekly.UsageRatio, 1e-9)
	assert.Equal(t, "tokens_pct", weekly.FieldKey)
	assert.Equal(t, "warning", weekly.Status)
	// Field entries redefine the routing status.
	assert.Equal(t, "warning", derived.Status)
}

func TestApplyEstimateConfigs_WindowOnlyKeepsWindowedLimits(t *testing.T) {
	// Limits that already carry a window are untouched by window-only entries.
	windowed := provider_quota.QuotaLimitStatus{Status: "warning", UsageRatio: 0.9}.WithWindow("5h", 5*time.Hour)
	derived := QuotaDerivedStatus{Status: "warning", Ready: true, Limits: []provider_quota.QuotaLimitStatus{windowed}}

	ApplyEstimateConfigs(nil, nil, []EstimateConfig{{Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.Equal(t, "5h", derived.Limits[0].Window)
	assert.InDelta(t, 0.9, derived.Limits[0].UsageRatio, 1e-9)
	assert.Nil(t, derived.Limits[0].PeriodStart)
}

func TestApplyEstimateConfigs_InvalidWindowNoop(t *testing.T) {
	derived := QuotaDerivedStatus{Status: "available", Ready: true, Limits: []provider_quota.QuotaLimitStatus{{Status: "available", UsageRatio: 0.1}}}

	ApplyEstimateConfigs(nil, nil, []EstimateConfig{{Window: "fortnight"}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.Empty(t, derived.Limits[0].Window)
}

func TestEstimateConfigs_MapRoundTrip(t *testing.T) {
	cfgs := []EstimateConfig{
		{FieldKey: "credits_pct", Remaining: true, ResetFieldKey: "cycle_reset", Window: EstimateWindowMonthly},
		{FieldKey: "used_pct", Window: EstimateWindow5h},
	}

	decoded, err := EstimateConfigsFromMap(EstimateConfigsToMap(cfgs))
	require.NoError(t, err)
	require.Len(t, decoded, 2)
	assert.Equal(t, cfgs, decoded)

	// Legacy flat single-config objects decode to a one-element list.
	legacy, err := EstimateConfigsFromMap(map[string]any{"fieldKey": "used_pct", "window": EstimateWindowWeekly})
	require.NoError(t, err)
	require.Len(t, legacy, 1)
	assert.Equal(t, "used_pct", legacy[0].FieldKey)

	// Nil maps and entries without any field/window mean "not configured".
	nilCfg, err := EstimateConfigsFromMap(nil)
	require.NoError(t, err)
	assert.Nil(t, nilCfg)
	emptyCfg, err := EstimateConfigsFromMap(map[string]any{"items": []any{map[string]any{"fieldKey": ""}}})
	require.NoError(t, err)
	assert.Nil(t, emptyCfg)
}

func TestValidateEstimateConfigs(t *testing.T) {
	keys := []string{"used_pct", "cycle_reset"}

	// Valid configs: field selection and the window-only template mode.
	assert.NoError(t, ValidateEstimateConfigs([]EstimateConfig{
		{FieldKey: "used_pct", Window: EstimateWindowWeekly},
		{Window: EstimateWindowMonthly},
	}, keys, nil))

	// Unknown window.
	err := ValidateEstimateConfigs([]EstimateConfig{{FieldKey: "used_pct", Window: "fortnight"}}, keys, nil)
	assert.ErrorContains(t, err, "window")

	// Unknown field key.
	err = ValidateEstimateConfigs([]EstimateConfig{{FieldKey: "nope", Window: EstimateWindowWeekly}}, keys, nil)
	assert.ErrorContains(t, err, "fieldKey")

	// Unknown reset field key.
	err = ValidateEstimateConfigs([]EstimateConfig{{FieldKey: "used_pct", ResetFieldKey: "nope", Window: EstimateWindowWeekly}}, keys, nil)
	assert.ErrorContains(t, err, "resetFieldKey")

	// A window is required on every entry.
	err = ValidateEstimateConfigs([]EstimateConfig{{}}, keys, nil)
	assert.ErrorContains(t, err, "window")

	// No display fields known yet: field keys are accepted unchecked.
	assert.NoError(t, ValidateEstimateConfigs([]EstimateConfig{{FieldKey: "anything", Window: EstimateWindow5h}}, nil, nil))
}

func TestApplyEstimateConfigs_VariableRawPercent(t *testing.T) {
	// A raw variable value above 1 is read as a percentage and divided by 100.
	fields := []ParsedField{}
	vars := map[string]any{"used_pct": 25.0}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, vars, []EstimateConfig{{FieldKey: "var:used_pct", Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.25, derived.Limits[0].UsageRatio, 1e-9)
	assert.Equal(t, "var:used_pct", derived.Limits[0].FieldKey)
}

func TestApplyEstimateConfigs_VariableRawRatio(t *testing.T) {
	// Values within [0, 1] pass through as ratios.
	fields := []ParsedField{}
	vars := map[string]any{"util": 0.3}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, vars, []EstimateConfig{{FieldKey: "var:util", Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.3, derived.Limits[0].UsageRatio, 1e-9)
}

func TestApplyEstimateConfigs_VariableRemaining(t *testing.T) {
	fields := []ParsedField{}
	vars := map[string]any{"remaining_pct": 70.0}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, vars, []EstimateConfig{{FieldKey: "var:remaining_pct", Remaining: true, Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	assert.InDelta(t, 0.3, derived.Limits[0].UsageRatio, 1e-9)
}

func TestApplyEstimateConfigs_VariableMissingFallsBack(t *testing.T) {
	fields := []ParsedField{{Key: "used_pct", Format: "percentage", Percent: 50}}
	derived := DeriveQuotaStatus("", fields)
	origLimits := derived.Limits

	ApplyEstimateConfigs(fields, map[string]any{}, []EstimateConfig{{FieldKey: "var:missing", Window: EstimateWindowWeekly}}, &derived)

	assert.Equal(t, origLimits, derived.Limits)
}

func TestApplyEstimateConfigs_VariableResetTime(t *testing.T) {
	// Unix-second variable values resolve as the reset time.
	fields := []ParsedField{{Key: "used_pct", Format: "percentage", Percent: 25}}
	reset := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	vars := map[string]any{"used_pct": 25.0, "reset_ts": float64(reset.Unix())}
	derived := DeriveQuotaStatus("", fields)

	ApplyEstimateConfigs(fields, vars, []EstimateConfig{{FieldKey: "var:used_pct", ResetFieldKey: "var:reset_ts", Window: EstimateWindowWeekly}}, &derived)

	require.Len(t, derived.Limits, 1)
	require.NotNil(t, derived.Limits[0].NextResetAt)
	// time.Unix yields Local; compare instants.
	assert.True(t, reset.Equal(*derived.Limits[0].NextResetAt))
}

func TestValidateEstimateConfigs_VariableReferences(t *testing.T) {
	displayKeys := []string{"used_pct"}
	variableKeys := []string{"raw_used", "reset_ts"}

	// var:-prefixed references validate against the variable keys.
	assert.NoError(t, ValidateEstimateConfigs([]EstimateConfig{
		{FieldKey: "var:raw_used", ResetFieldKey: "var:reset_ts", Window: EstimateWindowWeekly},
		{FieldKey: "used_pct", Window: EstimateWindow5h},
	}, displayKeys, variableKeys))

	// An unknown variable fails.
	err := ValidateEstimateConfigs([]EstimateConfig{{FieldKey: "var:nope", Window: EstimateWindowWeekly}}, displayKeys, variableKeys)
	assert.ErrorContains(t, err, "fieldKey")

	// An unknown var: reset reference fails.
	err = ValidateEstimateConfigs([]EstimateConfig{{FieldKey: "used_pct", ResetFieldKey: "var:nope", Window: EstimateWindowWeekly}}, displayKeys, variableKeys)
	assert.ErrorContains(t, err, "resetFieldKey")
}
