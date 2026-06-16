package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ldm2060/axonhub/internal/objects"
)

// ---------------------------------------------------------------------------
// evaluateQuotaMonitorBindingRule
// ---------------------------------------------------------------------------

func TestEvaluateBinding_StatusRuleMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		MonitorName:     "monitor-a",
		QuotaStatus:     "exhausted",
		TriggerStatuses: []string{"exhausted", "warning"},
		Conditions:      nil,
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective, "rule with trigger statuses should be effective")
	assert.True(t, result.Matched, "status 'exhausted' should match trigger_statuses")
	assert.Contains(t, result.Reason, "exhausted", "reason should include the matched status")
	assert.Contains(t, result.Reason, "monitor-a", "reason should include the monitor name")
}

func TestEvaluateBinding_StatusRuleNoMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "available",
		TriggerStatuses: []string{"exhausted", "warning"},
		Conditions:      nil,
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.False(t, result.Matched, "status 'available' should not match trigger_statuses")
}

func TestEvaluateBinding_NumericConditionMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		MonitorName: "monitor-b",
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"},
		},
		ParsedFields: map[string]any{
			"remaining": 0.0,
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "remaining <= 0 should match when remaining is 0")
	assert.Contains(t, result.Reason, "remaining", "reason should reference the field")
	assert.Contains(t, result.Reason, "monitor-b", "reason should include the monitor name for condition match")
	assert.Equal(t, "remaining", result.MatchedField, "MatchedField should be the condition's field")
}

func TestEvaluateBinding_NumericConditionNoMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"},
		},
		ParsedFields: map[string]any{
			"remaining": 5.0,
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.False(t, result.Matched, "remaining <= 0 should not match when remaining is 5")
	assert.Empty(t, result.MatchedField, "MatchedField should be empty when no condition matches")
}

func TestEvaluateBinding_TextContainsMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "plan", Operator: objects.QuotaMonitorOperatorContains, Value: "pro"},
		},
		ParsedFields: map[string]any{
			"plan": "professional",
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "'professional' contains 'pro'")
	assert.Equal(t, "plan", result.MatchedField)
}

func TestEvaluateBinding_TextNotContainsMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "plan", Operator: objects.QuotaMonitorOperatorNotContains, Value: "enterprise"},
		},
		ParsedFields: map[string]any{
			"plan": "professional",
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "'professional' does not contain 'enterprise'")
	assert.Equal(t, "plan", result.MatchedField)
}

func TestEvaluateBinding_InvalidNumericValueDoesNotMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"},
		},
		ParsedFields: map[string]any{
			"remaining": "not-a-number",
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.False(t, result.Matched, "non-numeric field value should not match numeric operator")
	assert.NotEmpty(t, result.Diagnostics, "should include diagnostics for parse failure")
}

func TestEvaluateBinding_InvalidNumericExpectedDoesNotMatch(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "abc"},
		},
		ParsedFields: map[string]any{
			"remaining": 5.0,
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.False(t, result.Matched, "non-numeric expected value should not match numeric operator")
	assert.NotEmpty(t, result.Diagnostics, "should include diagnostics for parse failure")
}

func TestEvaluateBinding_EmptyRulesIneffective(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "exhausted",
		TriggerStatuses: []string{},
		Conditions:      []objects.QuotaMonitorBindingCondition{},
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.False(t, result.Effective, "rule with empty statuses and conditions should be ineffective")
	assert.False(t, result.Matched, "ineffective rule should not match")
}

func TestEvaluateBinding_NilRulesIneffective(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:  "exhausted",
		ParsedFields: map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.False(t, result.Effective)
	assert.False(t, result.Matched)
}

func TestEvaluateBinding_StatusAndConditionsAreOR(t *testing.T) {
	// Status does not match but a condition does => overall match
	input := quotaMonitorBindingRuleInput{
		MonitorName:     "monitor-c",
		QuotaStatus:     "available",
		TriggerStatuses: []string{"exhausted"},
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"},
		},
		ParsedFields: map[string]any{
			"remaining": 0.0,
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "status and conditions are OR; condition match should yield overall match")
	assert.Contains(t, result.Reason, "monitor-c", "reason should include monitor name for condition match")
	assert.Equal(t, "remaining", result.MatchedField)
}

func TestEvaluateBinding_MultipleConditionsAreOR(t *testing.T) {
	// First condition does not match, second does => overall match
	// Use Fields (pre-flattened) because maxUsageRatio is set directly.
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"},
			{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "0.95"},
		},
		Fields: map[string]any{
			"remaining":     10.0,
			"maxUsageRatio": 0.98,
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "multiple conditions are OR; second match should yield overall match")
	assert.Equal(t, "maxUsageRatio", result.MatchedField, "MatchedField should be the first condition that matched")
}

func TestEvaluateBinding_StatusTrimmed(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "  exhausted  ",
		TriggerStatuses: []string{"exhausted"},
		Conditions:      nil,
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Matched, "status should be trimmed before matching")
}

func TestEvaluateBinding_TriggerStatusTrimmed(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "exhausted",
		TriggerStatuses: []string{"  exhausted  ", " warning "},
		Conditions:      nil,
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Matched, "trigger statuses should be trimmed before matching")
}

func TestEvaluateBinding_EmptyTriggerStatusIgnored(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "exhausted",
		TriggerStatuses: []string{"", "  ", "exhausted"},
		Conditions:      nil,
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "empty/whitespace-only trigger statuses should be ignored, but valid ones should match")
}

// Test that the legacy Status/Rule/Fields fields still work (backward compat).
func TestEvaluateBinding_LegacyFieldsWork(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		Status: "exhausted",
		Rule: quotaMonitorBindingRule{
			TriggerStatuses: []string{"exhausted"},
		},
		Fields: map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "legacy Status/Rule/Fields should still work")
}

// Test that QuotaStatus takes precedence over Status.
func TestEvaluateBinding_QuotaStatusOverridesStatus(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "exhausted",
		Status:          "available",
		TriggerStatuses: []string{"exhausted"},
		ParsedFields:    map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Matched, "QuotaStatus should take precedence over Status")
}

// Test that top-level TriggerStatuses take precedence over Rule.TriggerStatuses.
func TestEvaluateBinding_TopLevelTriggerStatusesPrecedence(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus:     "exhausted",
		TriggerStatuses: []string{"exhausted"},
		Rule: quotaMonitorBindingRule{
			TriggerStatuses: []string{"warning"},
		},
		ParsedFields: map[string]any{},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Matched, "top-level TriggerStatuses should take precedence over Rule.TriggerStatuses")
}

// Test internal flattening: ParsedFields + QuotaLimits + LastPollData merge.
func TestEvaluateBinding_InternalFlattening(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "0.95"},
		},
		ParsedFields: map[string]any{
			"status": "available",
		},
		QuotaLimits: []map[string]any{
			{"type": "token", "usageRatio": 0.98, "status": "warning", "ready": true},
		},
		LastPollData: map[string]any{
			"pollKey": "pollVal",
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "maxUsageRatio should be computed from QuotaLimits internally")
}

// Test that ParsedFields wins over LastPollData on key collision.
func TestEvaluateBinding_ParsedFieldsWinsOverLastPollData(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "sharedKey", Operator: objects.QuotaMonitorOperatorEQ, Value: "from-parsed"},
		},
		ParsedFields: map[string]any{
			"sharedKey": "from-parsed",
		},
		LastPollData: map[string]any{
			"sharedKey": "from-poll",
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Matched, "ParsedFields value should be used when key collides with LastPollData")
}

// Test that LastPollData fields are available when not in ParsedFields.
func TestEvaluateBinding_LastPollDataMerged(t *testing.T) {
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "pollOnlyKey", Operator: objects.QuotaMonitorOperatorEQ, Value: "from-poll"},
		},
		ParsedFields: map[string]any{},
		LastPollData: map[string]any{
			"pollOnlyKey": "from-poll",
		},
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Matched, "LastPollData fields should be merged when not present in ParsedFields")
}

// ---------------------------------------------------------------------------
// compareQuotaBindingCondition
// ---------------------------------------------------------------------------

func TestCompareQuotaBindingCondition_EqualityOperators(t *testing.T) {
	tests := []struct {
		name     string
		operator objects.QuotaMonitorConditionOperator
		actual   any
		expected string
		match    bool
	}{
		{"eq strings", objects.QuotaMonitorOperatorEQ, "hello", "hello", true},
		{"eq strings different", objects.QuotaMonitorOperatorEQ, "hello", "world", false},
		{"eq numbers", objects.QuotaMonitorOperatorEQ, 5.0, "5", true},
		{"neq strings", objects.QuotaMonitorOperatorNEQ, "hello", "world", true},
		{"neq same strings", objects.QuotaMonitorOperatorNEQ, "hello", "hello", false},
		{"lt", objects.QuotaMonitorOperatorLT, 3.0, "5", true},
		{"lt false", objects.QuotaMonitorOperatorLT, 7.0, "5", false},
		{"lte equal", objects.QuotaMonitorOperatorLTE, 5.0, "5", true},
		{"lte less", objects.QuotaMonitorOperatorLTE, 3.0, "5", true},
		{"lte greater", objects.QuotaMonitorOperatorLTE, 7.0, "5", false},
		{"gt", objects.QuotaMonitorOperatorGT, 7.0, "5", true},
		{"gt false", objects.QuotaMonitorOperatorGT, 3.0, "5", false},
		{"gte equal", objects.QuotaMonitorOperatorGTE, 5.0, "5", true},
		{"gte greater", objects.QuotaMonitorOperatorGTE, 7.0, "5", true},
		{"gte less", objects.QuotaMonitorOperatorGTE, 3.0, "5", false},
		{"contains match", objects.QuotaMonitorOperatorContains, "hello world", "world", true},
		{"contains no match", objects.QuotaMonitorOperatorContains, "hello world", "xyz", false},
		{"not_contains match", objects.QuotaMonitorOperatorNotContains, "hello world", "xyz", true},
		{"not_contains no match", objects.QuotaMonitorOperatorNotContains, "hello world", "world", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := objects.QuotaMonitorBindingCondition{
				Field:    "test",
				Operator: tt.operator,
				Value:    tt.expected,
			}
			matched, _ := compareQuotaBindingCondition(cond, tt.actual)
			assert.Equal(t, tt.match, matched)
		})
	}
}

// ---------------------------------------------------------------------------
// numberFromAny
// ---------------------------------------------------------------------------

func TestNumberFromAny(t *testing.T) {
	tests := []struct {
		name  string
		input any
		ok    bool
		value float64
	}{
		{"int", int(5), true, 5.0},
		{"int8", int8(5), true, 5.0},
		{"int16", int16(5), true, 5.0},
		{"int32", int32(5), true, 5.0},
		{"int64", int64(5), true, 5.0},
		{"uint", uint(5), true, 5.0},
		{"uint8", uint8(5), true, 5.0},
		{"uint16", uint16(5), true, 5.0},
		{"uint32", uint32(5), true, 5.0},
		{"uint64", uint64(5), true, 5.0},
		{"float32", float32(3.14), true, float64(float32(3.14))},
		{"float64", float64(3.14), true, 3.14},
		{"string number", "42", true, 42.0},
		{"string float", "3.14", true, 3.14},
		{"string with percent", "95%", true, 95.0},
		{"string with trailing percent and space", "95 %", true, 95.0},
		{"json.Number integer", json.Number("42"), true, 42.0},
		{"json.Number float", json.Number("3.14"), true, 3.14},
		{"invalid string", "not-a-number", false, 0},
		{"empty string", "", false, 0},
		{"bool", true, false, 0},
		{"nil", nil, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := numberFromAny(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.InDelta(t, tt.value, v, 0.001)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// flattenQuotaMonitorFields / maxUsageRatio
// ---------------------------------------------------------------------------

func TestMaxUsageRatio_FlattenedFromQuotaLimits(t *testing.T) {
	quotaLimits := []map[string]any{
		{"type": "token", "status": "available", "usageRatio": 0.5, "ready": true},
		{"type": "image", "status": "exhausted", "usageRatio": 1.0, "ready": false},
	}
	fields := flattenQuotaMonitorFields(map[string]any{
		"quota_limits": quotaLimits,
		"status":       "exhausted",
	}, quotaLimits)

	ratio, ok := fields["maxUsageRatio"]
	assert.True(t, ok, "maxUsageRatio should be present")
	assert.InDelta(t, 1.0, ratio, 0.001, "maxUsageRatio should be max of all usageRatio values")
}

func TestMaxUsageRatio_EmptyQuotaLimits(t *testing.T) {
	fields := flattenQuotaMonitorFields(map[string]any{
		"status": "available",
	}, nil)

	ratio, ok := fields["maxUsageRatio"]
	assert.True(t, ok, "maxUsageRatio should still be present even with empty limits")
	assert.InDelta(t, 0.0, ratio, 0.001, "maxUsageRatio should be 0 for empty limits")
}

func TestMaxUsageRatio_VirtualFieldUsedInCondition(t *testing.T) {
	quotaLimits := []map[string]any{
		{"type": "token", "usageRatio": 0.98, "status": "warning", "ready": true},
	}
	input := quotaMonitorBindingRuleInput{
		QuotaStatus: "available",
		Conditions: []objects.QuotaMonitorBindingCondition{
			{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "0.95"},
		},
		ParsedFields: map[string]any{
			"quota_limits": quotaLimits,
			"status":       "available",
		},
		QuotaLimits: quotaLimits,
	}
	result := evaluateQuotaMonitorBindingRule(input)
	assert.True(t, result.Effective)
	assert.True(t, result.Matched, "maxUsageRatio >= 0.95 should match when max ratio is 0.98")
}

// ---------------------------------------------------------------------------
// buildEvalFields
// ---------------------------------------------------------------------------

func TestBuildEvalFields_ParsedFieldsWinsOverLastPollData(t *testing.T) {
	fields := buildEvalFields(
		map[string]any{"key": "parsed", "onlyInParsed": true},
		nil,
		map[string]any{"key": "poll", "onlyInPoll": true},
	)
	assert.Equal(t, "parsed", fields["key"], "ParsedFields should win on key collision")
	assert.Equal(t, true, fields["onlyInParsed"], "ParsedFields-only key should be present")
	assert.Equal(t, true, fields["onlyInPoll"], "LastPollData-only key should be present")
}

func TestBuildEvalFields_MaxUsageRatioComputed(t *testing.T) {
	fields := buildEvalFields(
		map[string]any{"status": "available"},
		[]map[string]any{{"usageRatio": 0.8}},
		nil,
	)
	ratio, ok := fields["maxUsageRatio"]
	assert.True(t, ok)
	assert.InDelta(t, 0.8, ratio, 0.001)
}

// ---------------------------------------------------------------------------
// aggregateQuotaMonitorBindingResults
// ---------------------------------------------------------------------------

func TestAggregateBinding_StrategyAny_AnyMatchedNotReady(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: false, Reason: ""},
		{Effective: true, Matched: true, Reason: "exhausted"},
		{Effective: true, Matched: false, Reason: ""},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("any", results)
	assert.False(t, ready, "strategy any: any matched binding should make channel not ready")
	assert.Contains(t, reasons, "exhausted")
}

func TestAggregateBinding_StrategyAny_AllUnmatchedReady(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: false, Reason: ""},
		{Effective: true, Matched: false, Reason: ""},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("any", results)
	assert.True(t, ready, "strategy any: all unmatched => ready")
	assert.Empty(t, reasons)
}

func TestAggregateBinding_StrategyAny_IneffectiveBindingsIgnored(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: false, Matched: false, Reason: ""},
		{Effective: false, Matched: true, Reason: "should-not-count"},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("any", results)
	assert.True(t, ready, "strategy any: ineffective bindings should be ignored even if matched=true")
	assert.Empty(t, reasons)
}

func TestAggregateBinding_StrategyAny_NoEffectiveBindings(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: false, Matched: false, Reason: ""},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("any", results)
	assert.True(t, ready, "strategy any: no effective bindings => ready")
	assert.Empty(t, reasons)
}

func TestAggregateBinding_StrategyAll_AllMatchedNotReady(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: true, Reason: "exhausted"},
		{Effective: true, Matched: true, Reason: "maxUsageRatio >= 0.95"},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("all", results)
	assert.False(t, ready, "strategy all: all matched => not ready")
	assert.Contains(t, reasons, "exhausted")
	assert.Contains(t, reasons, "maxUsageRatio >= 0.95")
}

func TestAggregateBinding_StrategyAll_OneDoesNotMatchReady(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: true, Reason: "exhausted"},
		{Effective: true, Matched: false, Reason: ""},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("all", results)
	assert.True(t, ready, "strategy all: one not matched => ready")
	assert.Empty(t, reasons)
}

func TestAggregateBinding_StrategyAll_IneffectiveBindingsIgnored(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: true, Reason: "exhausted"},
		{Effective: false, Matched: false, Reason: ""},
	}
	ready, reasons := aggregateQuotaMonitorBindingResults("all", results)
	assert.False(t, ready, "strategy all: only effective bindings count; one effective matched => all effective matched => not ready")
	assert.Contains(t, reasons, "exhausted")
}

func TestAggregateBinding_DefaultIsAny(t *testing.T) {
	results := []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: true, Reason: "exhausted"},
		{Effective: true, Matched: false, Reason: ""},
	}
	ready, _ := aggregateQuotaMonitorBindingResults("something-else", results)
	assert.False(t, ready, "unknown strategy defaults to any; any matched => not ready")
}

func TestAggregateBinding_EmptyResults(t *testing.T) {
	ready, reasons := aggregateQuotaMonitorBindingResults("any", nil)
	assert.True(t, ready, "empty results => ready")
	assert.Empty(t, reasons)
}
