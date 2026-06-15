package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"github.com/ldm2060/axonhub/llm/httpclient"
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

func TestUsageMonitorService_UpdateChannelPersistsDisplayFieldGrouping(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:usage_monitor_display_field_grouping?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	owner := client.User.Create().
		SetEmail("monitor-group-owner@example.com").
		SetPassword("password").
		SetIsOwner(true).
		SaveX(ctx)

	svc := &UsageMonitorService{AbstractService: &AbstractService{db: client}}
	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UsageMonitorChannel]{
		Name:            "usage_monitor_channel_grouping_test",
		TTL:             time.Hour,
		RefreshInterval: time.Hour,
		KeyFunc:         func(v *ent.UsageMonitorChannel) int { return v.ID },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
		DeletedFunc: func(v *ent.UsageMonitorChannel) bool {
			return v.DeletedAt != 0
		},
	})
	defer svc.cache.Stop()

	created := client.UsageMonitorChannel.Create().
		SetName("group-monitor").
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetPollInterval(3600).
		SetFields([]map[string]any{{"key": "used", "label": "Used", "path": "$.used", "type": "jsonpath", "format": "number", "displayOrder": 0}}).
		SetVariables([]map[string]any{{"key": "used", "path": "$.used", "type": "jsonpath"}, {"key": "team_name", "path": "$.team.name", "type": "jsonpath"}}).
		SetDisplayFields([]map[string]any{{"key": "used", "label": "Used", "valueRef": "used", "format": "number", "displayOrder": 0}}).
		SetOwnerID(owner.ID).
		SaveX(ctx)
	svc.cache.Set(created.ID, created)

	displayFields := []usage_monitor.DisplayField{
		{
			Key:           "used",
			Label:         "Used",
			ValueRef:      "used",
			Format:        "number",
			DisplayOrder:  0,
			Group:         "team",
			GroupLabelRef: "team_name",
		},
	}

	updated, err := svc.UpdateChannel(ctx, created.ID, usage_monitor.UpdateUsageMonitorChannelInput{
		DisplayFields: &displayFields,
	})

	require.NoError(t, err)
	require.Len(t, updated.DisplayFields, 1)
	assert.Equal(t, "team", updated.DisplayFields[0]["group"])
	assert.Equal(t, "team_name", updated.DisplayFields[0]["groupLabelRef"])
	assert.Equal(t, "team", updated.Fields[0]["group"])
	assert.Equal(t, "team_name", updated.Fields[0]["groupLabelRef"])
}

func TestUsageMonitorService_PollChannelPersistsParsedGrouping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"team": map[string]any{
				"name": "Core Team",
				"used": 42,
			},
		}))
	}))
	defer server.Close()

	client := enttest.Open(t, dialect.SQLite, "file:usage_monitor_parsed_grouping?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	owner := client.User.Create().
		SetEmail("monitor-parsed-group-owner@example.com").
		SetPassword("password").
		SetIsOwner(true).
		SaveX(ctx)

	svc := &UsageMonitorService{
		AbstractService: &AbstractService{db: client},
		genericChecker:  usage_monitor.NewGenericQuotaChecker(httpclient.NewHttpClientWithClient(server.Client())),
	}
	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UsageMonitorChannel]{
		Name:            "usage_monitor_channel_parsed_grouping_test",
		TTL:             time.Hour,
		RefreshInterval: time.Hour,
		KeyFunc:         func(v *ent.UsageMonitorChannel) int { return v.ID },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
		DeletedFunc: func(v *ent.UsageMonitorChannel) bool {
			return v.DeletedAt != 0
		},
	})
	defer svc.cache.Stop()

	created := client.UsageMonitorChannel.Create().
		SetName("parsed-group-monitor").
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL(server.URL).
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetPollInterval(3600).
		SetFields([]map[string]any{{"key": "used", "label": "Used", "path": "$.team.used", "type": "jsonpath", "format": "number", "displayOrder": 0, "group": "team", "groupLabelRef": "team_name"}}).
		SetVariables([]map[string]any{{"key": "used", "path": "$.team.used", "type": "jsonpath"}, {"key": "team_name", "path": "$.team.name", "type": "jsonpath"}}).
		SetDisplayFields([]map[string]any{{"key": "used", "label": "Used", "valueRef": "used", "format": "number", "displayOrder": 0, "group": "team", "groupLabelRef": "team_name"}}).
		SetOwnerID(owner.ID).
		SaveX(ctx)
	svc.cache.Set(created.ID, created)

	svc.pollChannel(ctx, created)

	updated := client.UsageMonitorChannel.GetX(ctx, created.ID)
	fields, ok := updated.LastPollData["fields"].([]any)
	require.True(t, ok)
	require.Len(t, fields, 1)
	field, ok := fields[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "team", field["group"])
	assert.Equal(t, "Core Team", field["groupLabel"])
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
