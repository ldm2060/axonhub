package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAggregatedTime(t *testing.T) {
	// modernc/sqlite returns MAX(created_at) as a Go time.String(), optionally
	// followed by a monotonic " m=..." suffix when the stored value was produced
	// from a time.Now().String(). This is the regression that surfaced as
	// "unsupported Scan, storing driver.Value type string into *time.Time".
	cases := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "modernc sqlite go-string form",
			input: "2026-06-27 13:18:22.6316672 +0000 UTC",
			want:  time.Date(2026, 6, 27, 13, 18, 22, 631667200, time.UTC),
		},
		{
			name:  "modernc sqlite go-string with monotonic suffix",
			input: "2026-07-12 06:21:36.6129443 +0800 CST m=-3599.922974999",
			want:  time.Date(2026, 7, 11, 22, 21, 36, 612944300, time.UTC),
		},
		{
			name:  "rfc3339 nano (postgres/mysql text)",
			input: "2026-06-15T21:40:24.3922482Z",
			want:  time.Date(2026, 6, 15, 21, 40, 24, 392248200, time.UTC),
		},
		{
			name:  "rfc3339",
			input: "2026-06-15T21:40:24Z",
			want:  time.Date(2026, 6, 15, 21, 40, 24, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAggregatedTime(tc.input)
			require.NoError(t, err)
			assert.True(t, got.Equal(tc.want), "got %v want %v", got, tc.want)
		})
	}

	t.Run("empty rejected", func(t *testing.T) {
		_, err := parseAggregatedTime("   ")
		require.Error(t, err)
	})
}

func TestScanLastActiveAt(t *testing.T) {
	t.Run("nil source leaves dst nil", func(t *testing.T) {
		var dst *time.Time
		require.NoError(t, scanLastActiveAt(nil, &dst))
		assert.Nil(t, dst)
	})

	t.Run("string source", func(t *testing.T) {
		var dst *time.Time
		require.NoError(t, scanLastActiveAt("2026-06-15T21:40:24Z", &dst))
		require.NotNil(t, dst)
		assert.Equal(t, time.Date(2026, 6, 15, 21, 40, 24, 0, time.UTC), *dst)
	})

	t.Run("time.Time source", func(t *testing.T) {
		now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
		var dst *time.Time
		require.NoError(t, scanLastActiveAt(now, &dst))
		require.NotNil(t, dst)
		assert.Equal(t, now, *dst)
	})
}
