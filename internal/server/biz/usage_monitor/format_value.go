package usage_monitor

import (
	"fmt"
	"strconv"
	"time"
)

// FormatFieldValue converts a parsed field value to a string, handling
// datetime fields specially. Numeric values and numeric strings are treated
// as Unix timestamps: values larger than year-2000-in-milliseconds are read as
// millisecond timestamps, otherwise as second timestamps. RFC 3339 strings
// pass through unchanged. This avoids scientific notation and raw-timestamp
// output (e.g. "1782357029") for datetime fields whose value was extracted as
// a numeric string, such as a regex-parsed or JSON-quoted timestamp.
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

// toFloat64Val extracts a float64 from numeric values or numeric strings.
// Numeric strings are parsed so datetime fields whose value was extracted as a
// string (e.g. via regex, or a JSON-quoted number) can still be interpreted as
// Unix timestamps.
func toFloat64Val(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
