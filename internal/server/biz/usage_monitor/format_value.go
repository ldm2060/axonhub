package usage_monitor

import (
	"fmt"
	"time"
)

// FormatFieldValue converts a parsed field value to a string, handling
// datetime fields specially: large float64 values are treated as millisecond
// timestamps and converted to ISO 8601, avoiding scientific notation output.
func FormatFieldValue(v any, format string) string {
	if format == "datetime" {
		if f, ok := toFloat64Val(v); ok {
			if f > 946684800000 {
				t := time.UnixMilli(int64(f))
				return t.UTC().Format(time.RFC3339)
			}
			if f > 946684800 {
				t := time.Unix(int64(f), 0)
				return t.UTC().Format(time.RFC3339)
			}
		}
		if s, ok := v.(string); ok {
			if _, err := time.Parse(time.RFC3339, s); err == nil {
				return s
			}
		}
	}
	return fmt.Sprintf("%v", v)
}

func toFloat64Val(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
