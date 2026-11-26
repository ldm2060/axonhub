package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/apikey"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/request"
	"github.com/ldm2060/axonhub/internal/ent/user"
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

// TestMemberUsageAttributesAPIKeyOwner guards the attribution rule: API traffic
// leaves request.user_id empty, so a member's usage is only visible through the
// API key owner. Filtering by request.user_id alone zeroed every member's used
// figure in the pooling panel.
func TestMemberUsageAttributesAPIKeyOwner(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:pool-usage?mode=memory&_fk=1")
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)

	projectRow := client.Project.Create().SetName("pool-usage-project").SaveX(ctx)
	member := client.User.Create().
		SetEmail("pool-member@example.com").
		SetPassword("test-password").
		SetStatus(user.StatusActivated).
		SaveX(ctx)
	other := client.User.Create().
		SetEmail("pool-other@example.com").
		SetPassword("test-password").
		SetStatus(user.StatusActivated).
		SaveX(ctx)

	memberKey := client.APIKey.Create().
		SetName("pool-member-key").
		SetKey("ah-pool-member").
		SetUserID(member.ID).
		SetProjectID(projectRow.ID).
		SetType(apikey.TypeUser).
		SetStatus(apikey.StatusEnabled).
		SaveX(ctx)
	otherKey := client.APIKey.Create().
		SetName("pool-other-key").
		SetKey("ah-pool-other").
		SetUserID(other.ID).
		SetProjectID(projectRow.ID).
		SetType(apikey.TypeUser).
		SetStatus(apikey.StatusEnabled).
		SaveX(ctx)

	channelRow := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("pool-usage-channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "provider-key"}).
		SetSupportedModels([]string{"gpt-test"}).
		SetDefaultTestModel("gpt-test").
		SetStatus(channel.StatusEnabled).
		SaveX(ctx)
	otherChannel := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("pool-usage-other-channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "provider-key"}).
		SetSupportedModels([]string{"gpt-test"}).
		SetDefaultTestModel("gpt-test").
		SetStatus(channel.StatusEnabled).
		SaveX(ctx)

	logUsage := func(projectID, apiKeyID int, userID int, source request.Source, channelID int, cost float64) {
		mut := client.Request.Create().
			SetProjectID(projectID).
			SetModelID("gpt-test").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetSource(source).
			SetStatus(request.StatusCompleted)
		if apiKeyID != 0 {
			mut = mut.SetAPIKeyID(apiKeyID)
		}
		if userID != 0 {
			mut = mut.SetUserID(userID)
		}
		req := mut.SaveX(ctx)

		logMut := client.UsageLog.Create().
			SetRequestID(req.ID).
			SetProjectID(projectID).
			SetChannelID(channelID).
			SetModelID("gpt-test").
			SetTotalCost(cost)
		if apiKeyID != 0 {
			logMut = logMut.SetAPIKeyID(apiKeyID)
		}
		logMut.SaveX(ctx)
	}

	// Member's API traffic: no request user, attributed through the API key.
	logUsage(projectRow.ID, memberKey.ID, 0, request.SourceAPI, channelRow.ID, 3.5)
	// Member's playground traffic keeps its direct user.
	logUsage(projectRow.ID, 0, member.ID, request.SourcePlayground, channelRow.ID, 1.25)
	// Another member's API traffic must not count.
	logUsage(projectRow.ID, otherKey.ID, 0, request.SourceAPI, channelRow.ID, 5)
	// The member's usage on a different channel must not count.
	logUsage(projectRow.ID, memberKey.ID, 0, request.SourceAPI, otherChannel.ID, 2)

	svc := NewChannelPoolingService(ChannelPoolingServiceParams{Ent: client})

	// A non-UTC period start, as a provider-reported window can be: SQLite
	// compares stored timestamps as text, so the window must not depend on the
	// offset the caller happened to carry.
	periodStart := time.Now().Add(-time.Hour).In(time.FixedZone("CST", 8*60*60))

	used, err := svc.memberUsage(ctx, channelRow.ID, member.ID, periodStart)
	require.NoError(t, err)
	require.InDelta(t, 4.75, used, 1e-9)

	used, err = svc.memberUsage(ctx, channelRow.ID, other.ID, periodStart)
	require.NoError(t, err)
	require.InDelta(t, 5.0, used, 1e-9)
}
