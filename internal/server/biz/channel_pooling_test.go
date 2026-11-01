package biz

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
)

func TestValidatePoolingSettings(t *testing.T) {
	valid := &objects.ChannelSettings{
		Pooling: &objects.ChannelPoolingConfig{
			Enabled:   true,
			MonitorID: 7,
			Window:    objects.PoolingWindowWeekly,
			Members:   []objects.ChannelPoolingMember{{UserID: 2, SharePct: 25}, {UserID: 3, SharePct: 15}},
		},
	}
	require.NoError(t, ValidatePoolingSettings(valid))
	require.NoError(t, ValidatePoolingSettings(nil))
	require.NoError(t, ValidatePoolingSettings(&objects.ChannelSettings{}))

	t.Run("disabled config skips strict validation", func(t *testing.T) {
		disabled := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{Enabled: false, MonitorID: 0, Window: "bogus"},
		}
		require.NoError(t, ValidatePoolingSettings(disabled))
	})

	t.Run("unknown window", func(t *testing.T) {
		bad := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{Enabled: true, MonitorID: 7, Window: "7d"},
		}
		require.ErrorContains(t, ValidatePoolingSettings(bad), "unknown pooling window")
	})

	t.Run("missing monitor", func(t *testing.T) {
		bad := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{Enabled: true, MonitorID: 0, Window: "weekly"},
		}
		require.ErrorContains(t, ValidatePoolingSettings(bad), "bound usage monitor")
	})

	t.Run("duplicate member", func(t *testing.T) {
		bad := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{
				Enabled:   true,
				MonitorID: 7,
				Window:    "weekly",
				Members:   []objects.ChannelPoolingMember{{UserID: 2, SharePct: 10}, {UserID: 2, SharePct: 10}},
			},
		}
		require.ErrorContains(t, ValidatePoolingSettings(bad), "more than once")
	})

	t.Run("share bounds", func(t *testing.T) {
		zero := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{
				Enabled: true, MonitorID: 7, Window: "weekly",
				Members: []objects.ChannelPoolingMember{{UserID: 2, SharePct: 0}},
			},
		}
		require.Error(t, ValidatePoolingSettings(zero))

		over := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{
				Enabled: true, MonitorID: 7, Window: "weekly",
				Members: []objects.ChannelPoolingMember{{UserID: 2, SharePct: 100.5}},
			},
		}
		require.Error(t, ValidatePoolingSettings(over))
	})

	t.Run("share sum overflow", func(t *testing.T) {
		bad := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{
				Enabled:   true,
				MonitorID: 7,
				Window:    "weekly",
				Members:   []objects.ChannelPoolingMember{{UserID: 2, SharePct: 60}, {UserID: 3, SharePct: 60}},
			},
		}
		require.ErrorContains(t, ValidatePoolingSettings(bad), "exceeding 100%")
	})

	t.Run("sum of exactly 100 is allowed", func(t *testing.T) {
		full := &objects.ChannelSettings{
			Pooling: &objects.ChannelPoolingConfig{
				Enabled:   true,
				MonitorID: 7,
				Window:    "weekly",
				Members:   []objects.ChannelPoolingMember{{UserID: 2, SharePct: 40}, {UserID: 3, SharePct: 60}},
			},
		}
		require.NoError(t, ValidatePoolingSettings(full))
	})
}

func TestUpdateChannelSettingsInput_ApplyPooling(t *testing.T) {
	stored := &objects.ChannelPoolingConfig{
		Enabled:   true,
		MonitorID: 7,
		Window:    "weekly",
		Members:   []objects.ChannelPoolingMember{{UserID: 2, SharePct: 25}},
	}

	t.Run("absent pooling keeps stored config", func(t *testing.T) {
		merged := (&UpdateChannelSettingsInput{ExtraModelPrefix: lo.ToPtr("x")}).Apply(&objects.ChannelSettings{Pooling: stored})
		require.Same(t, stored, merged.Pooling)
	})

	t.Run("present pooling replaces the whole object", func(t *testing.T) {
		replacement := &objects.ChannelPoolingConfig{
			Enabled: false,
			Members: []objects.ChannelPoolingMember{{UserID: 3, SharePct: 10}},
		}
		merged := (&UpdateChannelSettingsInput{Pooling: replacement}).Apply(&objects.ChannelSettings{Pooling: stored})
		require.Same(t, replacement, merged.Pooling)
		require.False(t, merged.Pooling.Enabled)
		require.Len(t, merged.Pooling.Members, 1)
	})

	t.Run("nil stored pooling survives an absent patch", func(t *testing.T) {
		merged := (&UpdateChannelSettingsInput{}).Apply(&objects.ChannelSettings{})
		require.Nil(t, merged.Pooling)
	})
}

func TestPoolEstimate(t *testing.T) {
	start := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	reset := start.Add(7 * 24 * time.Hour)
	monitor := &ent.UsageMonitorChannel{
		QuotaLimits: []map[string]any{
			// 5h window: no period quota estimate priced yet.
			{"window": "5h", "usageRatio": 0.4},
			// Claude-style weekly pool, stamped 7d by the deriver.
			{"window": "7d", "periodStart": start.Format(time.RFC3339), "nextResetAt": reset.Format(time.RFC3339), "periodQuota": 70.0},
		},
	}

	t.Run("weekly matches the 7d alias", func(t *testing.T) {
		total, periodStart, nextReset, ok := poolEstimate(monitor, objects.PoolingWindowWeekly)
		require.True(t, ok)
		require.InDelta(t, 70.0, total, 1e-9)
		require.NotNil(t, periodStart)
		require.Equal(t, start, *periodStart)
		require.NotNil(t, nextReset)
	})

	t.Run("exact window label matches", func(t *testing.T) {
		monthly := &ent.UsageMonitorChannel{
			QuotaLimits: []map[string]any{
				{"window": "monthly", "periodStart": start.Format(time.RFC3339), "periodQuota": 30.5},
			},
		}
		total, _, _, ok := poolEstimate(monthly, objects.PoolingWindowMonthly)
		require.True(t, ok)
		require.InDelta(t, 30.5, total, 1e-9)
	})

	t.Run("unpriced or startless limits are skipped", func(t *testing.T) {
		noQuota := &ent.UsageMonitorChannel{
			QuotaLimits: []map[string]any{
				{"window": "7d", "periodStart": start.Format(time.RFC3339)},
				{"window": "7d", "periodQuota": 70.0},
			},
		}
		_, _, _, ok := poolEstimate(noQuota, objects.PoolingWindowWeekly)
		require.False(t, ok)
	})

	t.Run("unknown pooling window never matches", func(t *testing.T) {
		_, _, _, ok := poolEstimate(monitor, "bogus")
		require.False(t, ok)
	})
}

func TestPoolMemberStatus(t *testing.T) {
	require.Equal(t, PoolStatusAvailable, poolMemberStatus(1.0, 10))
	require.Equal(t, PoolStatusWarning, poolMemberStatus(8.0, 10))
	require.Equal(t, PoolStatusAvailable, poolMemberStatus(7.999, 10))
	require.Equal(t, PoolStatusPaused, poolMemberStatus(10.0, 10))
	require.Equal(t, PoolStatusPaused, poolMemberStatus(12.0, 10))
	// No estimate (quota 0): everything is available — fail-open display.
	require.Equal(t, PoolStatusAvailable, poolMemberStatus(5.0, 0))
}
