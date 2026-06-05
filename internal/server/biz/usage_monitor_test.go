package biz

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
)

func TestEnrichDisplayFieldsFromTemplate_UpgradesLegacyCopilotPercentRemainingValueRef(t *testing.T) {
	ch := &ent.UsageMonitorChannel{
		Source:       usagemonitorchannel.SourceTemplate,
		ProviderType: usagemonitorchannel.ProviderTypeGithubCopilot,
	}
	displayFields := []usage_monitor.DisplayField{
		{Key: "chat_pct", Label: "Chat Usage", ValueRef: "chat_pct", Format: "percentage"},
	}

	enrichDisplayFieldsFromTemplate(ch, displayFields)

	assert.Equal(t, "used_percent_from_remaining(chat_pct)", displayFields[0].ValueRef)
}

func TestUsageMonitorService_ListChannelsReloadsPersistedChannelsWhenCacheExpired(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:usage_monitor_list_channels?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	owner := client.User.Create().
		SetEmail("monitor-owner@example.com").
		SetPassword("password").
		SetIsOwner(true).
		SaveX(ctx)

	svc := &UsageMonitorService{AbstractService: &AbstractService{db: client}}
	created := client.UsageMonitorChannel.Create().
		SetName("persisted-monitor").
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetPollInterval(3600).
		SetFields([]map[string]any{{"key": "remaining", "label": "Remaining", "path": "$.remaining", "type": "jsonpath", "format": "number", "displayOrder": 0}}).
		SetVariables([]map[string]any{{"key": "remaining", "path": "$.remaining", "type": "jsonpath"}}).
		SetDisplayFields([]map[string]any{{"key": "remaining", "label": "Remaining", "valueRef": "remaining", "format": "number", "displayOrder": 0}}).
		SetOwnerID(owner.ID).
		SaveX(ctx)

	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UsageMonitorChannel]{
		Name:            "usage_monitor_channels_test",
		TTL:             time.Nanosecond,
		RefreshInterval: time.Hour,
		KeyFunc:         func(v *ent.UsageMonitorChannel) int { return v.ID },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
		DeletedFunc: func(v *ent.UsageMonitorChannel) bool {
			return v.DeletedAt != 0
		},
	})
	defer svc.cache.Stop()
	require.NoError(t, svc.cache.Load(ctx))
	require.Eventually(t, func() bool {
		return svc.cache.Len() == 0
	}, time.Second, 10*time.Millisecond)

	channels, err := svc.ListChannels(ctx)

	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, created.ID, channels[0].ID)
	assert.Equal(t, "persisted-monitor", channels[0].Name)
}
