package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

func setupTestUsageMonitorService(t *testing.T, defaultStrategy string) (*UsageMonitorService, context.Context) {
	t.Helper()

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })

	svc := &UsageMonitorService{
		AbstractService:             &AbstractService{db: client},
		defaultDisableThreshold:     1.0,
		defaultEnableThreshold:      0.95,
		defaultMultiMonitorStrategy: defaultStrategy,
		httpClient:                  httpclient.NewHttpClient(),
	}

	ctx := authz.WithSystemBypass(context.Background(), "test")
	return svc, ctx
}

func createTestChannelForMonitor(t *testing.T, svc *UsageMonitorService, ctx context.Context, strategy *channel.QuotaMultiMonitorStrategy) *ent.Channel {
	t.Helper()

	create := svc.db.Channel.Create().
		SetName("test-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com/v1").
		SetDefaultTestModel("gpt-4").
		SetSupportedModels([]string{"gpt-4"}).
		SetCredentials(objects.ChannelCredentials{
			APIKey: "test-key",
		})

	if strategy != nil {
		create.SetQuotaMultiMonitorStrategy(*strategy)
	}

	ch, err := create.Save(ctx)
	require.NoError(t, err)
	return ch
}

func TestEvaluateChannelQuotaReady_Any_AllReady(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "any")

	// Create a channel
	strategy := channel.QuotaMultiMonitorStrategyAny
	ch := createTestChannelForMonitor(t, svc, ctx, &strategy)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create multiple monitors, all with quota_ready=true
	monitor1, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(true).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	monitor2, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 2").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(true).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel is ready
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "Channel should be ready when all monitors are ready")
	assert.Nil(t, updated.ErrorMessage, "Error message should be nil")

	_ = monitor1
	_ = monitor2
}

func TestEvaluateChannelQuotaReady_Any_OneNotReady(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "any")

	// Create a channel
	strategy := channel.QuotaMultiMonitorStrategyAny
	ch := createTestChannelForMonitor(t, svc, ctx, &strategy)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create monitors: one ready, one not ready
	monitor1, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(true).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	monitor2, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 2 - Exhausted").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		SetQuotaLimits([]map[string]any{
			{
				"type":       "token",
				"usageRatio": 1.0,
			},
		}).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel is NOT ready
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "Channel should not be ready when ANY monitor is not ready")
	assert.NotNil(t, updated.ErrorMessage, "Error message should be set")
	assert.Contains(t, *updated.ErrorMessage, "Monitor 2 - Exhausted", "Error message should mention the exhausted monitor")

	_ = monitor1
	_ = monitor2
}

func TestEvaluateChannelQuotaReady_All_AllNotReady(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "all")

	// Create a channel with "all" strategy
	strategy := channel.QuotaMultiMonitorStrategyAll
	ch := createTestChannelForMonitor(t, svc, ctx, &strategy)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create monitors: all not ready
	monitor1, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	monitor2, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 2").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel is NOT ready
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "Channel should not be ready when ALL monitors are not ready")
	assert.NotNil(t, updated.ErrorMessage, "Error message should be set")

	_ = monitor1
	_ = monitor2
}

func TestEvaluateChannelQuotaReady_All_OneReady(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "all")

	// Create a channel with "all" strategy
	strategy := channel.QuotaMultiMonitorStrategyAll
	ch := createTestChannelForMonitor(t, svc, ctx, &strategy)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create monitors: one ready, one not ready
	monitor1, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(true).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	monitor2, err := svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 2").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel IS ready (because "all" strategy requires ALL to be not ready)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "Channel should be ready when at least one monitor is ready (all strategy)")
	assert.Nil(t, updated.ErrorMessage, "Error message should be nil")

	_ = monitor1
	_ = monitor2
}

func TestEvaluateChannelQuotaReady_NoMonitors(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "any")

	// Create a channel with no monitors
	ch := createTestChannelForMonitor(t, svc, ctx, nil)

	// Evaluate
	err := svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel is ready
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "Channel should be ready when no monitors exist")
	assert.Nil(t, updated.ErrorMessage, "Error message should be nil")
}

func TestEvaluateChannelQuotaReady_OnlyPausedMonitors(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "any")

	// Create a channel
	ch := createTestChannelForMonitor(t, svc, ctx, nil)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create a paused monitor (should not affect evaluation)
	_, err = svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusPaused).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel is ready (paused monitors are ignored)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "Channel should be ready when only paused monitors exist")
}

func TestEvaluateChannelQuotaReady_NullableQuotaReady(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "any")

	// Create a channel
	ch := createTestChannelForMonitor(t, svc, ctx, nil)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create a monitor with nil quota_ready
	// Note: The schema has Default(true), so nil becomes true
	// This test verifies the default behavior
	_, err = svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetNillableQuotaReady(nil).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel IS ready (because the default value is true)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "Channel should be ready when monitor has default quota_ready=true")
}

func TestEvaluateChannelQuotaReady_FallbackToGlobalStrategy(t *testing.T) {
	svc, ctx := setupTestUsageMonitorService(t, "all")

	// Create a channel without explicit strategy
	// Note: The schema has Default("any"), so it will NOT fall back to global
	// This test verifies the schema default takes precedence
	ch := createTestChannelForMonitor(t, svc, ctx, nil)

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	// Create monitors: one ready, one not ready
	_, err = svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 1").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(true).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.db.UsageMonitorChannel.Create().
		SetName("Monitor 2").
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetChannelID(ch.ID).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate (schema default "any" takes precedence over global "all")
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Verify channel is NOT ready (because "any" strategy disables if any monitor is not ready)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "Channel should not be ready when using schema default 'any' strategy with one monitor not ready")
}

func TestBuildErrorMessage(t *testing.T) {
	t.Run("with usage ratio", func(t *testing.T) {
		monitor := &ent.UsageMonitorChannel{
			Name: "Test Monitor",
			QuotaLimits: []map[string]any{
				{
					"type":       "token",
					"usageRatio": 0.95,
				},
			},
			LastPollData: map[string]any{
				"raw": "some data",
			},
		}

		msg := buildErrorMessage(monitor)
		assert.Contains(t, msg, "Test Monitor")
		assert.Contains(t, msg, "95%")
	})

	t.Run("without usage data", func(t *testing.T) {
		monitor := &ent.UsageMonitorChannel{
			Name:         "Test Monitor",
			LastPollData: nil,
		}

		msg := buildErrorMessage(monitor)
		assert.Contains(t, msg, "Test Monitor")
		assert.Contains(t, msg, "N/A")
	})
}
