package usage_monitor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oliveagle/jsonpath"
)

// ParseFields parses all field configs in two passes: first non-computed fields,
// then expression fields that reference values from the first pass.
func ParseFields(rawData []byte, configs []FieldConfig) []ParsedField {
	fieldValues := make(map[string]float64)
	results := make([]ParsedField, len(configs))

	// Pass 1: parse non-computed fields
	for i, cfg := range configs {
		if cfg.Expression != "" {
			continue
		}
		results[i] = ParseField(rawData, cfg)
		if results[i].Error == "" && results[i].Value != nil {
			if v, err := toFloat(results[i].Value); err == nil {
				fieldValues[cfg.Key] = v
			}
			if results[i].Total != nil {
				if v, err := toFloat(results[i].Total); err == nil {
					fieldValues[cfg.Key+"_total"] = v
				}
			}
		}
	}

	// Pass 2: evaluate expression fields
	for i, cfg := range configs {
		if cfg.Expression == "" {
			continue
		}
		results[i] = evalExpressionField(cfg, fieldValues)
	}

	return results
}

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
	value = unwrapSlice(value)
	result.Value = value

	if config.TotalPath != "" {
		total, err := jsonpath.JsonPathLookup(data, config.TotalPath)
		if err != nil {
			result.Error = fmt.Sprintf("totalPath %q failed: %v", config.TotalPath, err)
			return
		}
		total = unwrapSlice(total)
		result.Total = total
		result.Percent = computePercent(value, total)
	} else if config.Format == "percentage" {
		// The value itself is the percentage (e.g. Zhipu returns "percentage": 41)
		if n, err := toFloat(value); err == nil {
			result.Percent = n
		}
	}
}

// unwrapSlice extracts the first element if the value is a single-element slice,
// which commonly happens with JSONPath filter expressions like [?(@.type=='X')].
func unwrapSlice(v any) any {
	arr, ok := v.([]any)
	if !ok || len(arr) != 1 {
		return v
	}
	return arr[0]
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

func evalExpressionField(cfg FieldConfig, fieldValues map[string]float64) ParsedField {
	result := ParsedField{
		Key:    cfg.Key,
		Label:  cfg.Label,
		Format: cfg.Format,
		Unit:   cfg.Unit,
	}

	val, err := evalExpression(cfg.Expression, fieldValues)
	if err != nil {
		result.Error = fmt.Sprintf("expression %q failed: %v", cfg.Expression, err)
		return result
	}

	result.Value = val

	if cfg.Format == "percentage" {
		result.Percent = val
	}

	return result
}

// evalExpression evaluates a simple arithmetic expression with ${key} references.
// Supports: +, -, *, /, (), and numeric constants.
func evalExpression(expr string, values map[string]float64) (float64, error) {
	// Replace ${key} references with numeric values
	refRe := regexp.MustCompile(`\$\{(\w+)\}`)
	resolved := refRe.ReplaceAllStringFunc(expr, func(match string) string {
		key := match[2 : len(match)-1] // strip ${ and }
		v, ok := values[key]
		if !ok {
			return "NaN"
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	})

	// Check for unresolved references
	if strings.Contains(resolved, "NaN") {
		return 0, fmt.Errorf("unresolved reference in expression")
	}

	return evalArithmetic(resolved)
}

// evalArithmetic evaluates a simple arithmetic expression with +, -, *, /, ().
// Uses recursive descent parsing.
func evalArithmetic(expr string) (float64, error) {
	p := &exprParser{input: strings.TrimSpace(expr)}
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %q", p.pos, p.input[p.pos:])
	}
	return result, nil
}

type exprParser struct {
	input string
	pos   int
}

func (p *exprParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *exprParser) consume() byte {
	ch := p.peek()
	if ch != 0 {
		p.pos++
	}
	return ch
}

func (p *exprParser) skipSpaces() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

// parseExpr: addition and subtraction (lowest precedence).
func (p *exprParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		ch := p.peek()
		if ch != '+' && ch != '-' {
			break
		}
		p.consume()
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if ch == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

// parseTerm: multiplication and division.
func (p *exprParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		ch := p.peek()
		if ch != '*' && ch != '/' {
			break
		}
		p.consume()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if ch == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

// parseFactor: unary minus, parentheses, numbers.
func (p *exprParser) parseFactor() (float64, error) {
	p.skipSpaces()
	ch := p.peek()

	if ch == '-' {
		p.consume()
		v, err := p.parseFactor()
		return -v, err
	}

	if ch == '(' {
		p.consume()
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.peek() != ')' {
			return 0, fmt.Errorf("expected ')'")
		}
		p.consume()
		return v, nil
	}

	return p.parseNumber()
}

func (p *exprParser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.pos
	for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9' || p.input[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number at position %d", start)
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
}
