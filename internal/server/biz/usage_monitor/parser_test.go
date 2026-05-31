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
