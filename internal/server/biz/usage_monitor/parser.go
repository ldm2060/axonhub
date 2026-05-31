package usage_monitor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/oliveagle/jsonpath"
)

func ParseField(rawData []byte, config FieldConfig) ParsedField {
	result := ParsedField{
		Key:    config.Key,
		Label:  config.Label,
		Format: config.Format,
		Unit:   config.Unit,
	}

	switch FieldType(config.Type) {
	case FieldTypeJSONPath:
		parseJSONPath(rawData, config, &result)
	case FieldTypeRegex:
		parseRegex(rawData, config, &result)
	default:
		result.Error = fmt.Sprintf("unknown field type: %s", config.Type)
	}

	return result
}

func parseJSONPath(rawData []byte, config FieldConfig, result *ParsedField) {
	var data interface{}
	if err := json.Unmarshal(rawData, &data); err != nil {
		result.Error = fmt.Sprintf("invalid JSON: %v", err)
		return
	}

	value, err := jsonpath.JsonPathLookup(data, config.Path)
	if err != nil {
		result.Error = fmt.Sprintf("JSONPath %q failed: %v", config.Path, err)
		return
	}
	result.Value = value

	if config.TotalPath != "" {
		total, err := jsonpath.JsonPathLookup(data, config.TotalPath)
		if err != nil {
			result.Error = fmt.Sprintf("totalPath %q failed: %v", config.TotalPath, err)
			return
		}
		result.Total = total
		result.Percent = computePercent(value, total)
	}
}

func parseRegex(rawData []byte, config FieldConfig, result *ParsedField) {
	re, err := regexp.Compile(config.Path)
	if err != nil {
		result.Error = fmt.Sprintf("invalid regex: %v", err)
		return
	}

	matches := re.FindStringSubmatch(string(rawData))
	if len(matches) == 0 {
		result.Error = "regex matched 0 groups"
		return
	}

	switch FieldFormat(config.Format) {
	case FieldFormatFraction:
		if len(config.GroupIndex) >= 2 {
			result.Value = extractGroup(matches, config.GroupIndex[0])
			result.Total = extractGroup(matches, config.GroupIndex[1])
			result.Percent = computePercent(result.Value, result.Total)
		} else {
			result.Error = "fraction format requires groupIndex with 2 elements"
		}
	case FieldFormatNumber:
		if len(config.GroupIndex) >= 1 {
			result.Value = extractGroup(matches, config.GroupIndex[0])
		} else if len(matches) >= 2 {
			result.Value = matches[1]
		} else {
			result.Error = "no capture group available"
		}
	default:
		if len(matches) >= 2 {
			result.Value = matches[1]
		} else {
			result.Value = matches[0]
		}
	}
}

func extractGroup(matches []string, idx int) string {
	if idx < len(matches) {
		return matches[idx]
	}
	return ""
}

func computePercent(value, total interface{}) float64 {
	v, err1 := toFloat(value)
	t, err2 := toFloat(total)
	if err1 != nil || err2 != nil || t == 0 {
		return 0
	}
	return (v / t) * 100
}

func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", v)
	}
}
