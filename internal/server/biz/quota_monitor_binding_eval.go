package biz

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/ldm2060/axonhub/internal/objects"
)

// quotaMonitorBindingRuleInput holds the data needed to evaluate a single

// quotaMonitorBindingRuleInput holds the data needed to evaluate a single
// binding rule against a monitor's current state.
type quotaMonitorBindingRuleInput struct {
	// Status is the monitor's current quota status (e.g. "available", "warning", "exhausted").
	Status string
	// Rule holds the trigger statuses and conditions for this binding.
	Rule quotaMonitorBindingRule
	// Fields is the flattened field map produced by flattenQuotaMonitorFields.
	Fields map[string]any
}

// quotaMonitorBindingRule represents one binding's trigger configuration.
type quotaMonitorBindingRule struct {
	TriggerStatuses []string
	Conditions      []objects.QuotaMonitorBindingCondition
}

// quotaMonitorBindingRuleResult holds the outcome of evaluating one binding rule.
type quotaMonitorBindingRuleResult struct {
	// Effective is true when the rule has at least one trigger status or condition.
	Effective bool
	// Matched is true when the rule matched (status OR condition hit).
	Matched bool
	// Reason describes why the rule matched (human-readable).
	Reason string
	// Diagnostics contains non-fatal issues encountered during evaluation
	// (e.g. a value that could not be parsed as a number).
	Diagnostics []string
}

// evaluateQuotaMonitorBindingRule evaluates a single binding rule against the
// monitor's current state.
//
// Evaluation logic:
//   - Status rules and field conditions are OR-ed: if either matches, the rule matches.
//   - Multiple field conditions are OR-ed: any one matching is sufficient.
//   - Empty trigger statuses + empty conditions => Effective=false, Matched=false.
//   - Valid statuses are trimmed; empty/whitespace-only entries are ignored.
func evaluateQuotaMonitorBindingRule(input quotaMonitorBindingRuleInput) quotaMonitorBindingRuleResult {
	var result quotaMonitorBindingRuleResult

	// Check status match
	statusMatched, statusReason := matchTriggerStatus(strings.TrimSpace(input.Status), input.Rule.TriggerStatuses)

	// Check condition matches
	var condMatched bool
	var condReasons []string
	for _, cond := range input.Rule.Conditions {
		matched, diag := compareQuotaBindingCondition(cond, input.Fields[cond.Field])
		if matched {
			condMatched = true
			condReasons = append(condReasons, fmt.Sprintf("%s %s %s", cond.Field, string(cond.Operator), cond.Value))
		}
		if diag != "" {
			result.Diagnostics = append(result.Diagnostics, diag)
		}
	}

	hasStatuses := hasNonEmptyStatus(input.Rule.TriggerStatuses)
	hasConditions := len(input.Rule.Conditions) > 0

	result.Effective = hasStatuses || hasConditions
	if !result.Effective {
		return result
	}

	result.Matched = statusMatched || condMatched

	// Build reason
	var reasons []string
	if statusMatched {
		reasons = append(reasons, statusReason)
	}
	reasons = append(reasons, condReasons...)
	result.Reason = strings.Join(reasons, "; ")

	return result
}

// matchTriggerStatus checks if the given status matches any of the trigger statuses.
// Returns whether a match was found and a reason string.
func matchTriggerStatus(status string, triggerStatuses []string) (bool, string) {
	for _, ts := range triggerStatuses {
		trimmed := strings.TrimSpace(ts)
		if trimmed == "" {
			continue
		}
		if status == trimmed {
			return true, fmt.Sprintf("status=%s", status)
		}
	}
	return false, ""
}

// hasNonEmptyStatus returns true if at least one trigger status is non-empty after trimming.
func hasNonEmptyStatus(statuses []string) bool {
	for _, s := range statuses {
		if strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

// compareQuotaBindingCondition compares a field's actual value against the
// condition's expected value using the condition's operator.
//
// Numeric operators (<, <=, =, !=, >=, >) require both actual and expected to
// parse as numbers; otherwise the condition does not match and a diagnostic is returned.
// The equality operator (=) can also use trimmed string comparison as a fallback.
// "contains" and "not_contains" use string contains.
func compareQuotaBindingCondition(cond objects.QuotaMonitorBindingCondition, actual any) (matched bool, diagnostic string) {
	op := cond.Operator
	expectedStr := cond.Value

	switch op {
	case objects.QuotaMonitorOperatorContains:
		actualStr := fmt.Sprintf("%v", actual)
		return strings.Contains(actualStr, expectedStr), ""

	case objects.QuotaMonitorOperatorNotContains:
		actualStr := fmt.Sprintf("%v", actual)
		return !strings.Contains(actualStr, expectedStr), ""

	case objects.QuotaMonitorOperatorEQ:
		// Try numeric first
		actualNum, actualOk := numberFromAny(actual)
		expectedNum, expectedOk := numberFromAny(expectedStr)
		if actualOk && expectedOk {
			return floatEqual(actualNum, expectedNum), ""
		}
		// Fall back to string comparison
		actualStr := strings.TrimSpace(fmt.Sprintf("%v", actual))
		return actualStr == strings.TrimSpace(expectedStr), ""

	case objects.QuotaMonitorOperatorNEQ:
		// Try numeric first
		actualNum, actualOk := numberFromAny(actual)
		expectedNum, expectedOk := numberFromAny(expectedStr)
		if actualOk && expectedOk {
			return !floatEqual(actualNum, expectedNum), ""
		}
		// Fall back to string comparison
		actualStr := strings.TrimSpace(fmt.Sprintf("%v", actual))
		return actualStr != strings.TrimSpace(expectedStr), ""

	default:
		// Numeric-only operators: <, <=, >=, >
		actualNum, actualOk := numberFromAny(actual)
		expectedNum, expectedOk := numberFromAny(expectedStr)
		if !actualOk {
			return false, fmt.Sprintf("field %q: actual value %v is not a number", cond.Field, actual)
		}
		if !expectedOk {
			return false, fmt.Sprintf("field %q: expected value %q is not a number", cond.Field, expectedStr)
		}
		switch op {
		case objects.QuotaMonitorOperatorLT:
			return actualNum < expectedNum, ""
		case objects.QuotaMonitorOperatorLTE:
			return actualNum <= expectedNum, ""
		case objects.QuotaMonitorOperatorGTE:
			return actualNum >= expectedNum, ""
		case objects.QuotaMonitorOperatorGT:
			return actualNum > expectedNum, ""
		default:
			return false, fmt.Sprintf("field %q: unknown operator %q", cond.Field, string(op))
		}
	}
}

// floatEqual compares two floats with a small epsilon for floating-point
// imprecision.
func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// numberFromAny converts common numeric types, strings (including optional
// trailing %), and encoding/json.Number to float64.
// Returns the value and true on success, or 0 and false if the value cannot
// be parsed as a number.
func numberFromAny(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}

	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		s := strings.TrimSpace(n)
		s = strings.TrimSuffix(s, "%")
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// flattenQuotaMonitorFields copies parsed fields from the monitor's data and
// adds the virtual field "maxUsageRatio" computed as the maximum numeric
// usageRatio from quotaLimits.
func flattenQuotaMonitorFields(parsedData map[string]any, quotaLimits []map[string]any) map[string]any {
	fields := make(map[string]any, len(parsedData)+1)

	// Copy all parsed data fields
	maps.Copy(fields, parsedData)

	// Compute maxUsageRatio from quota_limits
	maxRatio := 0.0
	for _, limit := range quotaLimits {
		if ratio, ok := limit["usageRatio"]; ok {
			if r, ok := numberFromAny(ratio); ok && r > maxRatio {
				maxRatio = r
			}
		}
	}
	fields["maxUsageRatio"] = maxRatio

	return fields
}

// aggregateQuotaMonitorBindingResults aggregates the results of evaluating
// multiple binding rules according to the given strategy.
//
// Strategy "any" (default): the channel is not ready if ANY effective binding
// matched. Returns not-ready with the first match reason.
//
// Strategy "all": the channel is not ready only if ALL effective bindings
// matched. If even one effective binding does not match, the channel is ready.
// When all match, returns not-ready with combined reasons.
//
// Ineffective bindings (Effective=false) are ignored in both strategies.
func aggregateQuotaMonitorBindingResults(strategy string, results []quotaMonitorBindingRuleResult) (ready bool, reasons string) {
	if len(results) == 0 {
		return true, ""
	}

	// Filter to effective bindings only
	var effective []quotaMonitorBindingRuleResult
	for _, r := range results {
		if r.Effective {
			effective = append(effective, r)
		}
	}

	if len(effective) == 0 {
		return true, ""
	}

	// Default to "any" for any strategy that is not exactly "all"
	if strategy != "all" {
		// Strategy "any": not ready if any effective binding matched
		var matchedReasons []string
		for _, r := range effective {
			if r.Matched {
				matchedReasons = append(matchedReasons, r.Reason)
			}
		}
		if len(matchedReasons) > 0 {
			return false, strings.Join(matchedReasons, "; ")
		}
		return true, ""
	}

	// Strategy "all": not ready only if ALL effective bindings matched
	allMatched := true
	var matchedReasons []string
	for _, r := range effective {
		if !r.Matched {
			allMatched = false
			break
		}
		matchedReasons = append(matchedReasons, r.Reason)
	}

	if allMatched {
		return false, strings.Join(matchedReasons, "; ")
	}
	return true, ""
}
