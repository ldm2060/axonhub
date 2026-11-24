package biz

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
)

func newVisibilityTestService(t *testing.T, client *ent.Client) *UsageMonitorService {
	t.Helper()

	svc := &UsageMonitorService{AbstractService: &AbstractService{db: client}}
	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UsageMonitorChannel]{
		Name:            "usage_monitor_visibility_test",
		TTL:             time.Hour,
		RefreshInterval: time.Hour,
		KeyFunc:         func(v *ent.UsageMonitorChannel) int { return v.ID },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
		DeletedFunc: func(v *ent.UsageMonitorChannel) bool {
			return v.DeletedAt != 0
		},
	})
	t.Cleanup(svc.cache.Stop)
	require.NoError(t, svc.cache.Load(context.Background()))

	return svc
}

func TestUsageMonitorService_ListChannelsFiltersMonitorsByChannelAccess(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:usage_monitor_visibility?mode=memory&_fk=0")
	defer client.Close()

	baseCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	createUser := func(email string, isOwner bool) *ent.User {
		builder := client.User.Create().SetEmail(email).SetPassword("password")
		if isOwner {
			builder = builder.SetIsOwner(true)
		}
		return builder.SaveX(baseCtx)
	}
	createChannel := func(typ channel.Type, name string, owner *ent.User, visibility channel.Visibility, sharedWith []int) *ent.Channel {
		return client.Channel.Create().
			SetType(typ).
			SetName(name).
			SetBaseURL("https://api.anthropic.com").
			SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
			SetSupportedModels([]string{"claude-sonnet-4-5"}).
			SetDefaultTestModel("claude-sonnet-4-5").
			SetStatus(channel.StatusEnabled).
			SetOwnerID(owner.ID).
			SetVisibility(visibility).
			SetSharedWith(sharedWith).
			SaveX(baseCtx)
	}
	createMonitor := func(name string, owner *ent.User, boundChannel *ent.Channel, providerType usagemonitorchannel.ProviderType) *ent.UsageMonitorChannel {
		builder := client.UsageMonitorChannel.Create().
			SetName(name).
			SetSource(usagemonitorchannel.SourceTemplate).
			SetAPIURL("https://example.com/quota").
			SetAPIMethod(usagemonitorchannel.APIMethodGET).
			SetAPIHeaders(map[string]any{}).
			SetPollInterval(3600).
			SetFields([]map[string]any{{"key": "used", "label": "Used", "path": "$.used", "type": "jsonpath", "format": "number", "displayOrder": 0}}).
			SetOwnerID(owner.ID)
		if boundChannel != nil {
			builder = builder.SetChannelID(boundChannel.ID)
		}
		if providerType != "" {
			builder = builder.SetProviderType(providerType)
		}
		return builder.SaveX(baseCtx)
	}

	owner := createUser("visibility-owner@example.com", false)
	viewer := createUser("visibility-viewer@example.com", false)
	stranger := createUser("visibility-stranger@example.com", false)
	systemOwner := createUser("visibility-system-owner@example.com", true)

	shared := createChannel(channel.TypeClaudecode, "Shared", owner, channel.VisibilityShared, []int{viewer.ID})
	private := createChannel(channel.TypeClaudecode, "Private", owner, channel.VisibilityPrivate, nil)
	published := createChannel(channel.TypeClaudecode, "Published", owner, channel.VisibilityPublished, nil)
	// The only codex channel in the system is the owner's private one, so the
	// stranger's codex template monitor has a candidate channel it may not use.
	createChannel(channel.TypeCodex, "Private Codex", owner, channel.VisibilityPrivate, nil)

	// The pool monitor is bound to the owner's private channel but prices the
	// pool of a channel the viewer holds a share on.
	poolMonitor := createMonitor("Pool Monitor", owner, private, "")
	pooled := createChannel(channel.TypeClaudecode, "Pooled", owner, channel.VisibilityShared, []int{viewer.ID})
	pooled.Settings = &objects.ChannelSettings{
		Pooling: &objects.ChannelPoolingConfig{
			Enabled:   true,
			MonitorID: poolMonitor.ID,
			Window:    "monthly",
			Members:   []objects.ChannelPoolingMember{{UserID: viewer.ID, SharePct: 40}},
		},
	}
	client.Channel.UpdateOneID(pooled.ID).SetSettings(pooled.Settings).SaveX(baseCtx)

	boundShared := createMonitor("Bound Shared", owner, shared, "")
	boundPrivate := createMonitor("Bound Private", owner, private, "")
	boundPublished := createMonitor("Bound Published", owner, published, "")
	providerTypeMonitor := createMonitor("Provider Type", owner, nil, usagemonitorchannel.ProviderTypeClaudecode)
	// The stranger's own codex monitor: its only candidate channel is the owner's
	// private codex channel, so availability alone would hide it from its author.
	strangerProviderTypeMonitor := createMonitor("Stranger Provider Type", stranger, nil, usagemonitorchannel.ProviderTypeCodex)
	customOwn := createMonitor("Custom Own", viewer, nil, "")
	customOther := createMonitor("Custom Other", owner, nil, "")

	svc := newVisibilityTestService(t, client)

	t.Run("viewer sees monitors of channels they may use", func(t *testing.T) {
		monitors, err := svc.ListChannels(contexts.WithUser(baseCtx, viewer))

		require.NoError(t, err)
		assert.Equal(t,
			[]int{poolMonitor.ID, boundShared.ID, boundPublished.ID, providerTypeMonitor.ID, customOwn.ID},
			monitorIDs(monitors))
	})

	t.Run("stranger sees published channels, their own monitor, and nothing else", func(t *testing.T) {
		monitors, err := svc.ListChannels(contexts.WithUser(baseCtx, stranger))

		require.NoError(t, err)
		assert.Equal(t,
			[]int{boundPublished.ID, providerTypeMonitor.ID, strangerProviderTypeMonitor.ID},
			monitorIDs(monitors))
	})

	t.Run("system owner sees every monitor", func(t *testing.T) {
		monitors, err := svc.ListChannels(contexts.WithUser(baseCtx, systemOwner))

		require.NoError(t, err)
		assert.Equal(t,
			[]int{
				poolMonitor.ID, boundShared.ID, boundPrivate.ID, boundPublished.ID,
				providerTypeMonitor.ID, strangerProviderTypeMonitor.ID, customOwn.ID, customOther.ID,
			},
			monitorIDs(monitors))
	})

	t.Run("a request without a principal is unrestricted", func(t *testing.T) {
		monitors, err := svc.ListChannels(baseCtx)

		require.NoError(t, err)
		assert.Len(t, monitors, 8)
	})
}

func monitorIDs(monitors []*ent.UsageMonitorChannel) []int {
	ids := make([]int, 0, len(monitors))
	for _, monitor := range monitors {
		ids = append(ids, monitor.ID)
	}
	return ids
}
