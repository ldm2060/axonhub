package usage_monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseField_JSONPath_SimpleValue(t *testing.T) {
	rawData := []byte(`{"data": {"usage": {"total_tokens": 1300000}}}`)
	config := FieldConfig{
		Key:    "token_usage",
		Label:  "Token Usage",
		Path:   "$.data.usage.total_tokens",
		Type:   "jsonpath",
		Format: "number",
		Unit:   "tokens",
	}
	result := ParseField(rawData, config)

	assert.Equal(t, "token_usage", result.Key)
	assert.Equal(t, "Token Usage", result.Label)
	assert.Empty(t, result.Error)
	assert.Equal(t, float64(1300000), result.Value)
	assert.Equal(t, "tokens", result.Unit)
	assert.Equal(t, "number", result.Format)
}

func TestParseField_JSONPath_PercentageWithTotal(t *testing.T) {
	rawData := []byte(`{"usage": {"used": 250, "limit": 1000}}`)
	config := FieldConfig{
		Key:       "usage_percent",
		Label:     "Usage",
		Path:      "$.usage.used",
		TotalPath: "$.usage.limit",
		Type:      "jsonpath",
		Format:    "percentage",
		Unit:      "requests",
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, float64(250), result.Value)
	assert.Equal(t, float64(1000), result.Total)
	assert.InDelta(t, 25.0, result.Percent, 0.01)
}

func TestParseField_JSONPath_InvalidJSON(t *testing.T) {
	rawData := []byte(`not valid json`)
	config := FieldConfig{
		Key:    "test",
		Label:  "Test",
		Path:   "$.data",
		Type:   "jsonpath",
		Format: "number",
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "invalid JSON")
}

func TestParseField_JSONPath_MissingPath(t *testing.T) {
	rawData := []byte(`{"data": {"usage": 100}}`)
	config := FieldConfig{
		Key:    "missing",
		Label:  "Missing",
		Path:   "$.data.nonexistent",
		Type:   "jsonpath",
		Format: "number",
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "JSONPath")
	assert.Contains(t, result.Error, "failed")
}

func TestParseField_JSONPath_StringValue(t *testing.T) {
	rawData := []byte(`{"percent": "85.5", "total": "1000"}`)
	config := FieldConfig{
		Key:       "pct",
		Label:     "Percentage",
		Path:      "$.percent",
		TotalPath: "$.total",
		Type:      "jsonpath",
		Format:    "percentage",
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, "85.5", result.Value)
	assert.Equal(t, "1000", result.Total)
	assert.InDelta(t, 8.55, result.Percent, 0.01)
}

func TestParseField_Regex_Fraction(t *testing.T) {
	rawData := []byte(`Used: 250 of 1000 tokens`)
	config := FieldConfig{
		Key:        "tokens",
		Label:      "Tokens",
		Path:       `Used: (\d+) of (\d+)`,
		Type:       "regex",
		Format:     "fraction",
		GroupIndex: []int{1, 2},
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, "250", result.Value)
	assert.Equal(t, "1000", result.Total)
	assert.InDelta(t, 25.0, result.Percent, 0.01)
}

func TestParseField_Regex_Number(t *testing.T) {
	rawData := []byte(`Total requests: 5432`)
	config := FieldConfig{
		Key:        "requests",
		Label:      "Requests",
		Path:       `Total requests: (\d+)`,
		Type:       "regex",
		Format:     "number",
		GroupIndex: []int{1},
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, "5432", result.Value)
}

func TestParseField_Regex_NoMatch(t *testing.T) {
	rawData := []byte(`no matching content here`)
	config := FieldConfig{
		Key:    "nomatch",
		Label:  "NoMatch",
		Path:   `Usage: (\d+)`,
		Type:   "regex",
		Format: "number",
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "regex matched 0 groups")
}

func TestParseField_Regex_InvalidPattern(t *testing.T) {
	rawData := []byte(`some data`)
	config := FieldConfig{
		Key:    "bad",
		Label:  "Bad",
		Path:   `[invalid(`,
		Type:   "regex",
		Format: "number",
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "invalid regex")
}

func TestParseField_Regex_TextFormat(t *testing.T) {
	rawData := []byte(`Plan: Professional`)
	config := FieldConfig{
		Key:    "plan",
		Label:  "Plan",
		Path:   `Plan: (\w+)`,
		Type:   "regex",
		Format: "text",
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, "Professional", result.Value)
}

func TestParseField_UnknownType(t *testing.T) {
	rawData := []byte(`{"key": "value"}`)
	config := FieldConfig{
		Key:    "unknown",
		Label:  "Unknown",
		Path:   "$.key",
		Type:   "xml",
		Format: "text",
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "unknown field type")
}

func TestParseField_Regex_Fraction_MissingGroupIndex(t *testing.T) {
	rawData := []byte(`Used: 250 of 1000 tokens`)
	config := FieldConfig{
		Key:    "tokens",
		Label:  "Tokens",
		Path:   `Used: (\d+) of (\d+)`,
		Type:   "regex",
		Format: "fraction",
		// GroupIndex omitted — should produce error
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "fraction format requires groupIndex with 2 elements")
}

func TestParseField_Regex_Number_NoGroupIndex_UsesFirstCaptureGroup(t *testing.T) {
	rawData := []byte(`Count: 42`)
	config := FieldConfig{
		Key:    "count",
		Label:  "Count",
		Path:   `Count: (\d+)`,
		Type:   "regex",
		Format: "number",
		// GroupIndex omitted — should fall back to first capture group
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, "42", result.Value)
}

func TestParseField_JSONPath_TotalPathFailure(t *testing.T) {
	rawData := []byte(`{"usage": 100}`)
	config := FieldConfig{
		Key:       "pct",
		Label:     "Pct",
		Path:      "$.usage",
		TotalPath: "$.nonexistent",
		Type:      "jsonpath",
		Format:    "percentage",
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "totalPath")
}

func TestParseField_Regex_Number_NoCaptureGroup(t *testing.T) {
	rawData := []byte(`Count: 42`)
	config := FieldConfig{
		Key:    "count",
		Label:  "Count",
		Path:   `Count: \d+`,
		Type:   "regex",
		Format: "number",
		// No GroupIndex, and regex has no capture group — only full match in matches[0]
	}
	result := ParseField(rawData, config)

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "no capture group available")
}

func TestParseField_Regex_Number_GroupIndexOutOfRange(t *testing.T) {
	rawData := []byte(`Count: 42`)
	config := FieldConfig{
		Key:        "count",
		Label:      "Count",
		Path:       `Count: (\d+)`,
		Type:       "regex",
		Format:     "number",
		GroupIndex: []int{5}, // out of range — extractGroup returns empty string
	}
	result := ParseField(rawData, config)

	// extractGroup silently returns empty string for out-of-range index
	assert.Empty(t, result.Error)
	assert.Equal(t, "", result.Value)
}

func TestParseField_JSONPath_ArrayIndex(t *testing.T) {
	rawData := []byte(`{"data": {"limits": [{"type": "TIME_LIMIT", "percentage": 4, "nextResetTime": 1782648382987}, {"type": "TOKENS_LIMIT", "percentage": 1, "nextResetTime": 1780316398388}]}}`)
	config := FieldConfig{
		Key:    "token_pct",
		Label:  "Token Usage %",
		Path:   "$.data.limits[1].percentage",
		Type:   "jsonpath",
		Format: "percentage",
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, float64(1), result.Value)
}

func TestParseField_JSONPath_ArrayIndex_Datetime(t *testing.T) {
	rawData := []byte(`{"data": {"limits": [{"type": "TIME_LIMIT", "nextResetTime": 1782648382987}, {"type": "TOKENS_LIMIT", "nextResetTime": 1780316398388}]}}`)
	config := FieldConfig{
		Key:    "token_reset",
		Label:  "Token Reset",
		Path:   "$.data.limits[1].nextResetTime",
		Type:   "jsonpath",
		Format: "datetime",
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, float64(1780316398388), result.Value)
}

func TestParseField_JSONPath_UnwrapSlice(t *testing.T) {
	// JSONPath expressions that return single-element arrays should be unwrapped
	rawData := []byte(`{"items": [42]}`)
	config := FieldConfig{
		Key:    "val",
		Label:  "Val",
		Path:   "$.items[0]",
		Type:   "jsonpath",
		Format: "number",
	}
	result := ParseField(rawData, config)

	assert.Empty(t, result.Error)
	assert.Equal(t, float64(42), result.Value)
}

func TestParseFields_Expression(t *testing.T) {
	rawData := []byte(`{"used": 250, "total": 1000}`)
	configs := []FieldConfig{
		{Key: "used", Label: "Used", Path: "$.used", Type: "jsonpath", Format: "number"},
		{Key: "total", Label: "Total", Path: "$.total", Type: "jsonpath", Format: "number"},
		{Key: "pct", Label: "Percentage", Format: "percentage", Expression: "${used}/${total}*100"},
	}
	results := ParseFields(rawData, configs)

	assert.Empty(t, results[0].Error)
	assert.Equal(t, float64(250), results[0].Value)
	assert.Empty(t, results[1].Error)
	assert.Equal(t, float64(1000), results[1].Value)
	assert.Empty(t, results[2].Error)
	assert.InDelta(t, 25.0, results[2].Value.(float64), 0.01)
	assert.InDelta(t, 25.0, results[2].Percent, 0.01)
}

func TestParseFields_ExpressionSubtraction(t *testing.T) {
	rawData := []byte(`{"total": 1000, "used": 250}`)
	configs := []FieldConfig{
		{Key: "total", Label: "Total", Path: "$.total", Type: "jsonpath", Format: "number"},
		{Key: "used", Label: "Used", Path: "$.used", Type: "jsonpath", Format: "number"},
		{Key: "remaining", Label: "Remaining", Format: "number", Expression: "${total}-${used}"},
	}
	results := ParseFields(rawData, configs)

	assert.Empty(t, results[2].Error)
	assert.InDelta(t, 750.0, results[2].Value.(float64), 0.01)
}

func TestParseFields_ExpressionUnresolvedRef(t *testing.T) {
	rawData := []byte(`{"used": 250}`)
	configs := []FieldConfig{
		{Key: "used", Label: "Used", Path: "$.used", Type: "jsonpath", Format: "number"},
		{Key: "pct", Label: "Pct", Format: "percentage", Expression: "${used}/${missing}*100"},
	}
	results := ParseFields(rawData, configs)

	assert.NotEmpty(t, results[1].Error)
	assert.Contains(t, results[1].Error, "unresolved reference")
}

func TestEvalArithmetic(t *testing.T) {
	tests := []struct {
		expr     string
		expected float64
	}{
		{"4+3", 7},
		{"10-3", 7},
		{"4*3", 12},
		{"12/4", 3},
		{"2+3*4", 14},
		{"(2+3)*4", 20},
		{"100/0", 0}, // div by zero
		{"-5+3", -2}, // unary minus
		{"10/(2+3)", 2},
	}
	for _, tt := range tests {
		result, err := evalArithmetic(tt.expr)
		if tt.expected == 0 && tt.expr == "100/0" {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err, "expr: %s", tt.expr)
		assert.InDelta(t, tt.expected, result, 0.01, "expr: %s", tt.expr)
	}
}

func TestExtractVariables(t *testing.T) {
	body := []byte(`{"data": {"used": 100, "total": 500}, "level": "normal"}`)
	vars := []Variable{
		{Key: "used", Path: "$.data.used", Type: "jsonpath"},
		{Key: "total", Path: "$.data.total", Type: "jsonpath"},
		{Key: "level", Path: "$.level", Type: "jsonpath"},
	}
	result := ExtractVariables(body, vars)
	assert.Equal(t, float64(100), result["used"])
	assert.Equal(t, float64(500), result["total"])
	assert.Equal(t, "normal", result["level"])
}

func TestExtractVariables_Regex(t *testing.T) {
	body := []byte(`Used: 250 of 1000 tokens`)
	vars := []Variable{
		{Key: "used", Path: `Used: (\d+) of (\d+)`, Type: "regex", GroupIndex: []int{1}},
		{Key: "total", Path: `Used: (\d+) of (\d+)`, Type: "regex", GroupIndex: []int{2}},
	}
	result := ExtractVariables(body, vars)
	assert.Equal(t, "250", result["used"])
	assert.Equal(t, "1000", result["total"])
}

func TestExtractVariables_MissingPath(t *testing.T) {
	body := []byte(`{"data": {"used": 100}}`)
	vars := []Variable{
		{Key: "used", Path: "$.data.used", Type: "jsonpath"},
		{Key: "missing", Path: "$.data.nonexistent", Type: "jsonpath"},
	}
	result := ExtractVariables(body, vars)
	assert.Equal(t, float64(100), result["used"])
	_, ok := result["missing"]
	assert.False(t, ok, "missing variable should not be in result")
}

func TestRenderDisplayFields_SimpleRef(t *testing.T) {
	vars := map[string]any{"used": float64(100), "total": float64(500)}
	fields := []DisplayField{
		{Key: "usage", Label: "Usage", ValueRef: "used", Format: "number", DisplayOrder: 0},
	}
	result := RenderDisplayFields(vars, fields)
	assert.Len(t, result, 1)
	assert.Equal(t, float64(100), result[0].Value)
	assert.Equal(t, "usage", result[0].Key)
	assert.Equal(t, "Usage", result[0].Label)
}

func TestRenderDisplayFields_Expression(t *testing.T) {
	vars := map[string]any{"used": float64(100), "total": float64(500)}
	fields := []DisplayField{
		{Key: "pct", Label: "Percent", ValueRef: "${used}/${total}*100", Format: "percentage", DisplayOrder: 0},
	}
	result := RenderDisplayFields(vars, fields)
	assert.Len(t, result, 1)
	assert.InDelta(t, 20.0, result[0].Value, 0.01)
}

func TestRenderDisplayFields_PercentageDirect(t *testing.T) {
	vars := map[string]any{"pct": float64(85.5)}
	fields := []DisplayField{
		{Key: "pct", Label: "Pct", ValueRef: "pct", Format: "percentage", DisplayOrder: 0},
	}
	result := RenderDisplayFields(vars, fields)
	assert.Len(t, result, 1)
	assert.InDelta(t, 85.5, result[0].Percent, 0.01)
}

func TestRenderDisplayFields_MissingVariable(t *testing.T) {
	vars := map[string]any{"used": float64(100)}
	fields := []DisplayField{
		{Key: "pct", Label: "Pct", ValueRef: "missing", Format: "text", DisplayOrder: 0},
	}
	result := RenderDisplayFields(vars, fields)
	assert.Len(t, result, 1)
	// Missing simple variable → value is nil (not an error), UI renders as N/A
	assert.Nil(t, result[0].Value)
	assert.Empty(t, result[0].Error)
}

func TestRenderDisplayFields_TotalRef(t *testing.T) {
	vars := map[string]any{"used": float64(250), "used_total": float64(1000)}
	fields := []DisplayField{
		{Key: "usage", Label: "Usage", ValueRef: "used", TotalRef: "used_total", Format: "fraction", DisplayOrder: 0},
	}
	result := RenderDisplayFields(vars, fields)
	assert.Len(t, result, 1)
	assert.Equal(t, float64(250), result[0].Value)
	assert.Equal(t, float64(1000), result[0].Total)
	assert.InDelta(t, 25.0, result[0].Percent, 0.01)
}

func TestRenderDisplayFields_GitHubCopilotPercentRemainingRenderedAsUsage(t *testing.T) {
	rawData := []byte(`{
		"copilot_plan": "individual",
		"access_type_sku": "copilot_free",
		"quota_snapshots": {
			"chat": {"percent_remaining": 100, "remaining": 50, "unlimited": false}
		}
	}`)

	tmpl := GetChannelTemplate("github_copilot")
	if !assert.NotNil(t, tmpl) {
		return
	}

	vars := ExtractVariables(rawData, tmpl.Variables)
	results := RenderDisplayFields(vars, tmpl.DisplayFields)

	var chatUsage *ParsedField
	for i := range results {
		if results[i].Key == "chat_pct" {
			chatUsage = &results[i]
			break
		}
	}

	if assert.NotNil(t, chatUsage) {
		assert.Empty(t, chatUsage.Error)
		assert.InDelta(t, 0, chatUsage.Value, 0.01)
		assert.InDelta(t, 0, chatUsage.Percent, 0.01)
	}

	derived := DeriveQuotaStatus("github_copilot", results)
	assert.Equal(t, "available", derived.Status)
	assert.True(t, derived.Ready)
	if assert.Len(t, derived.Limits, 1) {
		assert.InDelta(t, 0, derived.Limits[0].UsageRatio, 0.01)
	}
}
