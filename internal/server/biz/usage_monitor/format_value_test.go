package usage_monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A Unix timestamp extracted as a string (e.g. via a regex field, or a
// JSON-quoted number) must be converted to RFC3339, not echoed back as the raw
// number. Regression test for the case where the monitor card showed
// "1782357029" verbatim.
func TestFormatFieldValue_Datetime_NumericString_Seconds(t *testing.T) {
	got := FormatFieldValue("1782357029", "datetime")
	want := time.Unix(1782357029, 0).UTC().Format(time.RFC3339)
	assert.Equal(t, want, got)
	assert.NotEqual(t, "1782357029", got)
}

func TestFormatFieldValue_Datetime_NumericString_Milliseconds(t *testing.T) {
	got := FormatFieldValue("1780316398388", "datetime")
	want := time.UnixMilli(1780316398388).UTC().Format(time.RFC3339)
	assert.Equal(t, want, got)
}

func TestFormatFieldValue_Datetime_Float_Seconds(t *testing.T) {
	got := FormatFieldValue(float64(1782357029), "datetime")
	want := time.Unix(1782357029, 0).UTC().Format(time.RFC3339)
	assert.Equal(t, want, got)
}

func TestFormatFieldValue_Datetime_Float_Milliseconds(t *testing.T) {
	got := FormatFieldValue(float64(1780316398388), "datetime")
	want := time.UnixMilli(1780316398388).UTC().Format(time.RFC3339)
	assert.Equal(t, want, got)
}

func TestFormatFieldValue_Datetime_RFC3339String(t *testing.T) {
	in := "2026-06-25T03:10:29Z"
	assert.Equal(t, in, FormatFieldValue(in, "datetime"))
}

func TestFormatFieldValue_Datetime_NonNumericString_Fallback(t *testing.T) {
	// Non-date, non-numeric strings fall through unchanged.
	assert.Equal(t, "not-a-date", FormatFieldValue("not-a-date", "datetime"))
}

func TestFormatFieldValue_NonDatetime(t *testing.T) {
	assert.Equal(t, "42", FormatFieldValue("42", "number"))
}
