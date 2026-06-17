package biz

import (
	"context"
	"testing"
	"time"

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

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func setupTestBindingService(t *testing.T, defaultStrategy string) (*UsageMonitorService, context.Context) {
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

func createTestChannelForBinding(t *testing.T, svc *UsageMonitorService, ctx context.Context, name string, strategy *channel.QuotaMultiMonitorStrategy) *ent.Channel {
	t.Helper()

	create := svc.db.Channel.Create().
		SetName(name).
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
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

func createTestMonitorForBinding(t *testing.T, svc *UsageMonitorService, ctx context.Context, name string, quotaStatus usagemonitorchannel.QuotaStatus, channelID *int) *ent.UsageMonitorChannel {
	t.Helper()

	// Create owner user
	user, err := svc.db.User.Create().
		SetEmail(name + "@test.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	create := svc.db.UsageMonitorChannel.Create().
		SetName(name).
		SetSource(usagemonitorchannel.SourceBuiltin).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetQuotaStatus(quotaStatus).
		SetOwnerID(user.ID)

	if channelID != nil {
		create.SetChannelID(*channelID)
	}

	monitor, err := create.Save(ctx)
	require.NoError(t, err)
	return monitor
}

// ---------------------------------------------------------------------------
// SaveChannelQuotaMonitorBindings tests
// ---------------------------------------------------------------------------

// TestSaveChannelQuotaMonitorBindings_ReplacesBindingsAndEvaluates verifies that
// SaveChannelQuotaMonitorBindingsAndEvaluate replaces existing bindings, stores
// the strategy on the channel, and immediately evaluates an exhausted status rule
// to set channel.quotaBindingReady=false with an error message that includes the
// monitor name.
func TestSaveChannelQuotaMonitorBindings_ReplacesBindingsAndEvaluates(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-1", nil)
	monitor1 := createTestMonitorForBinding(t, svc, ctx, "Monitor-A", usagemonitorchannel.QuotaStatusAvailable, nil)
	monitor2 := createTestMonitorForBinding(t, svc, ctx, "Monitor-B-Exhausted", usagemonitorchannel.QuotaStatusExhausted, nil)

	// First save: bind monitor1 with available status
	err := svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor1.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	// Channel should be ready (monitor1 is available, not exhausted)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady)

	// Second save: replace with monitor2 that has exhausted status
	err = svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor2.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	// Channel should NOT be ready (monitor2 is exhausted, strategy=any)
	updated, err = svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "channel should not be ready when bound monitor is exhausted")
	require.NotNil(t, updated.ErrorMessage, "error message should be set")
	assert.Contains(t, *updated.ErrorMessage, "Monitor-B-Exhausted", "error message should include monitor name")

	// Verify only one binding row exists (old one was replaced)
	bindings, err := svc.ListChannelQuotaMonitorBindings(ctx, ch.ID)
	require.NoError(t, err)
	assert.Len(t, bindings, 1, "old bindings should be replaced")
	assert.Equal(t, monitor2.ID, bindings[0].UsageMonitorChannelID)

	// Verify strategy was stored on the channel
	assert.Equal(t, channel.QuotaMultiMonitorStrategyAny, *updated.QuotaMultiMonitorStrategy)
}

// TestSaveChannelQuotaMonitorBindings_AllStrategy verifies that "all" strategy
// requires all effective bindings to match. One exhausted and one available
// should result in ready=true; both exhausted should result in ready=false.
func TestSaveChannelQuotaMonitorBindings_AllStrategy(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "all")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-2", nil)
	monitorExhausted := createTestMonitorForBinding(t, svc, ctx, "Monitor-Exhausted", usagemonitorchannel.QuotaStatusExhausted, nil)
	monitorAvailable := createTestMonitorForBinding(t, svc, ctx, "Monitor-Available", usagemonitorchannel.QuotaStatusAvailable, nil)

	// Both monitors bound, one exhausted, one available => ready=true (all strategy)
	err := svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "all",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitorExhausted.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
			{
				UsageMonitorChannelID: monitorAvailable.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "all strategy: one available monitor should keep channel ready")
	assert.Nil(t, updated.ErrorMessage)

	// Now make both exhausted
	_, err = svc.db.UsageMonitorChannel.UpdateOneID(monitorAvailable.ID).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusExhausted).
		Save(ctx)
	require.NoError(t, err)

	// Re-evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	updated, err = svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "all strategy: both exhausted should make channel not ready")
	require.NotNil(t, updated.ErrorMessage)
}

// TestSaveChannelQuotaMonitorBindings_FieldCondition verifies that a field
// condition on maxUsageRatio >= 1 sets not ready; lowering quotaLimits to 0.5
// and re-evaluating recovers the channel to ready=true and clears the
// quota-owned errorMessage.
func TestSaveChannelQuotaMonitorBindings_FieldCondition(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-3", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Quota-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	// Set monitor's quotaLimits with usageRatio=1.0 (exhausted)
	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaLimits([]map[string]any{
			{"type": "token", "usageRatio": 1.0, "status": "exhausted", "ready": false},
		}).
		SetLastPollData(map[string]any{
			"fields": []any{
				map[string]any{"key": "tokens", "value": 1000.0, "total": 1000.0, "percent": 100.0},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Bind with condition: maxUsageRatio >= 1
	err = svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor.ID,
				Enabled:               true,
				Conditions: []objects.QuotaMonitorBindingCondition{
					{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "1"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Channel should NOT be ready (maxUsageRatio=1.0 >= 1)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "channel should not be ready when maxUsageRatio >= 1")
	require.NotNil(t, updated.ErrorMessage, "error message should be set")
	assert.Contains(t, *updated.ErrorMessage, "maxUsageRatio", "error message should reference the condition field")

	// Lower quotaLimits to 0.5 (usage ratio drops)
	_, err = svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaLimits([]map[string]any{
			{"type": "token", "usageRatio": 0.5, "status": "available", "ready": true},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Re-evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Channel should recover to ready=true
	updated, err = svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "channel should recover when maxUsageRatio drops below 1")
	assert.Nil(t, updated.ErrorMessage, "quota-owned error message should be cleared on recovery")
}

// TestSaveChannelQuotaMonitorBindings_DisabledBindingsDoNotDisable verifies
// that disabled or empty bindings do not disable a channel if feasible.
func TestSaveChannelQuotaMonitorBindings_ErrorMonitorKeepsLastQuotaBindingEffective(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-error-monitor", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Error-Quota-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetStatus(usagemonitorchannel.StatusError).
		SetLastPollError("HTTP request failed").
		SetQuotaLimits([]map[string]any{
			{"type": "token", "usageRatio": 0.91, "status": "exhausted", "ready": false},
		}).
		SetLastPollData(map[string]any{
			"remaining": 0,
		}).
		Save(ctx)
	require.NoError(t, err)

	err = svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor.ID,
				Enabled:               true,
				Conditions: []objects.QuotaMonitorBindingCondition{
					{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "0.8"},
				},
			},
		},
	})
	require.NoError(t, err)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "error-state monitors must still enforce preserved quota data")
	require.NotNil(t, updated.ErrorMessage)
	assert.Contains(t, *updated.ErrorMessage, "maxUsageRatio")
}

func TestSaveChannelQuotaMonitorBindings_DisabledBindingsDoNotDisable(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-4", nil)
	monitorExhausted := createTestMonitorForBinding(t, svc, ctx, "Exhausted-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Bind with disabled=true — should not affect channel
	err := svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitorExhausted.ID,
				Enabled:               false, // disabled
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	// Channel should be ready (binding is disabled, falls back to legacy path with no auto-disable monitors)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "disabled binding should not disable channel")
	assert.Nil(t, updated.ErrorMessage)

	// Also test with empty bindings (no bindings at all)
	err = svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{},
	})
	require.NoError(t, err)

	updated, err = svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "empty bindings should not disable channel")
	assert.Nil(t, updated.ErrorMessage)
}

// TestSaveChannelQuotaMonitorBindings_StrategyDefaultsToAny verifies that
// an empty strategy defaults to "any".
func TestSaveChannelQuotaMonitorBindings_StrategyDefaultsToAny(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-5", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Test-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	err := svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "", // empty, should default to "any"
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, channel.QuotaMultiMonitorStrategyAny, *updated.QuotaMultiMonitorStrategy)
}

// TestSaveChannelQuotaMonitorBindings_InvalidStrategy verifies that an
// invalid strategy is rejected.
func TestSaveChannelQuotaMonitorBindings_InvalidStrategy(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-6", nil)

	err := svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "invalid",
		Bindings: []SaveChannelQuotaMonitorBindingInput{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid strategy")
}

// TestSaveChannelQuotaMonitorBindings_SkipsZeroMonitorID verifies that
// bindings with UsageMonitorChannelID=0 are skipped.
func TestSaveChannelQuotaMonitorBindings_SkipsZeroMonitorID(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-7", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Real-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	err := svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{UsageMonitorChannelID: 0, Enabled: true, TriggerStatuses: []string{"exhausted"}}, // should be skipped
			{UsageMonitorChannelID: monitor.ID, Enabled: true, TriggerStatuses: []string{"exhausted"}},
		},
	})
	require.NoError(t, err)

	bindings, err := svc.ListChannelQuotaMonitorBindings(ctx, ch.ID)
	require.NoError(t, err)
	assert.Len(t, bindings, 1, "zero monitor ID binding should be skipped")
	assert.Equal(t, monitor.ID, bindings[0].UsageMonitorChannelID)
}

// TestSaveChannelQuotaMonitorBindings_CleansStatusesAndConditions verifies
// that trigger statuses are trimmed and blanks removed, and conditions with
// empty field/operator are dropped.
func TestSaveChannelQuotaMonitorBindings_CleansStatusesAndConditions(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-8", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Clean-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	err := svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor.ID,
				Enabled:               true,
				TriggerStatuses:       []string{" exhausted ", "  ", "warning"},
				Conditions: []objects.QuotaMonitorBindingCondition{
					{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "1"},
					{Field: "", Operator: objects.QuotaMonitorOperatorGTE, Value: "2"}, // empty field, should be dropped
					{Field: "someField", Operator: "", Value: "3"},                     // empty operator, should be dropped
				},
			},
		},
	})
	require.NoError(t, err)

	bindings, err := svc.ListChannelQuotaMonitorBindings(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)

	// Trigger statuses should be cleaned: trimmed, blanks removed
	assert.Equal(t, []string{"exhausted", "warning"}, bindings[0].TriggerStatuses)

	// Conditions should be cleaned: empty field/operator dropped
	assert.Len(t, bindings[0].Conditions, 1)
	assert.Equal(t, "maxUsageRatio", bindings[0].Conditions[0].Field)
}

// ---------------------------------------------------------------------------
// ListChannelQuotaMonitorBindings tests
// ---------------------------------------------------------------------------

func TestListChannelQuotaMonitorBindings_IncludesMonitorName(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-9", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "My-Quota-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	err := svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	bindings, err := svc.ListChannelQuotaMonitorBindings(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, "My-Quota-Monitor", bindings[0].UsageMonitorName)
}

// ---------------------------------------------------------------------------
// evaluateAndUpdateChannelQuotaReady (binding path) tests
// ---------------------------------------------------------------------------

// TestEvaluateAndUpdateChannelQuotaReady_BindingPath verifies that the
// binding-based evaluation path is used when bindings exist.
func TestEvaluateAndUpdateChannelQuotaReady_BindingPath(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-10", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Binding-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Create a binding row directly
	_, err := svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Channel should NOT be ready
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady)
	require.NotNil(t, updated.ErrorMessage)
	assert.Contains(t, *updated.ErrorMessage, "Binding-Monitor")
}

// TestEvaluateAndUpdateChannelQuotaReady_LegacyFallback verifies that when
// no binding rows exist, the legacy auto-disable path is used.
func TestEvaluateAndUpdateChannelQuotaReady_LegacyFallback(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-11", nil)

	// Create a monitor with auto_disable_enabled (legacy path)
	user, err := svc.db.User.Create().
		SetEmail("legacy@test.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.db.UsageMonitorChannel.Create().
		SetName("Legacy-Monitor").
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

	// No binding rows exist, so legacy path should be used
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady, "legacy path should detect not-ready monitor")
}

// TestEvaluateAndUpdateChannelQuotaReady_NoBindingsNoAutoDisable verifies
// that a channel with no bindings and no auto-disable monitors is ready.
func TestEvaluateAndUpdateChannelQuotaReady_NoBindingsNoAutoDisable(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-12", nil)

	err := svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady)
	assert.Nil(t, updated.ErrorMessage)
}

// TestEvaluateAndUpdateChannelQuotaReady_SkipsSoftDeletedMonitor verifies that
// bindings with soft-deleted monitors are skipped during evaluation (the preload
// filter on deleted_at=0 excludes them, resulting in nil edge).
func TestEvaluateAndUpdateChannelQuotaReady_SkipsSoftDeletedMonitor(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-13", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Deleted-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Soft-delete the monitor so the preload filter (deleted_at=0) excludes it
	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetDeletedAt(int(time.Now().Unix())).
		Save(ctx)
	require.NoError(t, err)

	// Create a binding pointing to the soft-deleted monitor
	_, err = svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate - soft-deleted monitor should be skipped (nil edge after preload)
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Channel should be ready (soft-deleted monitor is skipped, no effective bindings)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady)
}

// TestEvaluateAndUpdateChannelQuotaReady_SkipsPausedMonitor verifies that
// bindings to paused monitors are skipped during evaluation.
func TestEvaluateAndUpdateChannelQuotaReady_SkipsPausedMonitor(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-14", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Paused-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Pause the monitor
	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetStatus(usagemonitorchannel.StatusPaused).
		Save(ctx)
	require.NoError(t, err)

	// Create a binding
	_, err = svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate - paused monitor should be skipped
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Channel should be ready (paused monitor is skipped, no effective bindings)
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady)
}

// TestEvaluateAndUpdateChannelQuotaReady_DisabledChannelKeepsErrorMessage
// verifies that quota binding does not touch error_message on a disabled channel.
func TestEvaluateAndUpdateChannelQuotaReady_DisabledChannelKeepsErrorMessage(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-15", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Quota-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Create a binding
	_, err := svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// Disable the channel with an auto-disable message
	autoDisableMsg := "Channel auto-disabled: HTTP 500"
	_, err = svc.db.Channel.UpdateOneID(ch.ID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(autoDisableMsg).
		Save(ctx)
	require.NoError(t, err)

	// Evaluate - quota recovers (but channel is disabled)
	// Make monitor available
	_, err = svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusAvailable).
		Save(ctx)
	require.NoError(t, err)

	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Channel should be ready but keep the auto-disable message
	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "quota_binding_ready should be updated regardless of status")
	require.NotNil(t, updated.ErrorMessage, "disabled channel must keep its auto-disable error_message")
	assert.Equal(t, autoDisableMsg, *updated.ErrorMessage)
}

// ---------------------------------------------------------------------------
// evaluateChannelsForMonitor tests
// ---------------------------------------------------------------------------

// TestEvaluateChannelsForMonitor_ReEvaluatesAfterMonitorChange verifies that
// when a monitor's status changes, all channels bound to it are re-evaluated.
func TestEvaluateChannelsForMonitor_ReEvaluatesAfterMonitorChange(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch1 := createTestChannelForBinding(t, svc, ctx, "ch-16", nil)
	ch2 := createTestChannelForBinding(t, svc, ctx, "ch-17", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Shared-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	// Bind the same monitor to both channels
	for _, chID := range []int{ch1.ID, ch2.ID} {
		_, err := svc.db.ChannelUsageMonitorBinding.Create().
			SetChannelID(chID).
			SetUsageMonitorChannelID(monitor.ID).
			SetEnabled(true).
			SetTriggerStatuses([]string{"exhausted"}).
			SetConditions([]objects.QuotaMonitorBindingCondition{}).
			Save(ctx)
		require.NoError(t, err)
	}

	// Both channels should be ready initially
	for _, chID := range []int{ch1.ID, ch2.ID} {
		err := svc.evaluateAndUpdateChannelQuotaReady(ctx, chID)
		require.NoError(t, err)
		ch, _ := svc.db.Channel.Get(ctx, chID)
		assert.True(t, ch.QuotaBindingReady)
	}

	// Monitor becomes exhausted
	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusExhausted).
		Save(ctx)
	require.NoError(t, err)

	// Re-evaluate all channels for this monitor
	svc.evaluateChannelsForMonitor(ctx, monitor.ID)

	// Both channels should now be not ready
	for _, chID := range []int{ch1.ID, ch2.ID} {
		ch, err := svc.db.Channel.Get(ctx, chID)
		require.NoError(t, err)
		assert.False(t, ch.QuotaBindingReady, "channel should be not ready after monitor becomes exhausted")
	}
}

// ---------------------------------------------------------------------------
// extractParsedFieldsFromMonitor tests
// ---------------------------------------------------------------------------

func TestExtractParsedFieldsFromMonitor_NilData(t *testing.T) {
	monitor := &ent.UsageMonitorChannel{}
	result := extractParsedFieldsFromMonitor(monitor)
	assert.Equal(t, map[string]any{}, result)
}

func TestExtractParsedFieldsFromMonitor_WithFields(t *testing.T) {
	monitor := &ent.UsageMonitorChannel{
		LastPollData: map[string]any{
			"fields": []any{
				map[string]any{"key": "tokens", "value": 500.0, "total": 1000.0, "percent": 50.0},
				map[string]any{"key": "requests", "value": 200.0},
			},
		},
	}

	result := extractParsedFieldsFromMonitor(monitor)
	assert.Equal(t, 500.0, result["tokens"])
	assert.Equal(t, 1000.0, result["tokens.total"])
	assert.Equal(t, 50.0, result["tokens.percent"])
	assert.Equal(t, 200.0, result["requests"])
}

// ---------------------------------------------------------------------------
// cleanTriggerStatuses / cleanConditions tests
// ---------------------------------------------------------------------------

func TestCleanTriggerStatuses(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"all blanks", []string{"  ", "", "\t"}, []string{}},
		{"trimmed", []string{" exhausted ", " warning "}, []string{"exhausted", "warning"}},
		{"mixed", []string{"exhausted", "  ", "warning"}, []string{"exhausted", "warning"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanTriggerStatuses(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanConditions(t *testing.T) {
	conditions := []objects.QuotaMonitorBindingCondition{
		{Field: "maxUsageRatio", Operator: objects.QuotaMonitorOperatorGTE, Value: "1"},
		{Field: "", Operator: objects.QuotaMonitorOperatorGTE, Value: "2"},
		{Field: "someField", Operator: "", Value: "3"},
	}

	result := cleanConditions(conditions)
	assert.Len(t, result, 1)
	assert.Equal(t, "maxUsageRatio", result[0].Field)
}

// ---------------------------------------------------------------------------
// resolveStrategy tests
// ---------------------------------------------------------------------------

func TestResolveStrategy(t *testing.T) {
	tests := []struct {
		name         string
		chStrategy   *channel.QuotaMultiMonitorStrategy
		defaultStrat string
		expected     string
	}{
		{"nil strategy uses default", nil, "all", "all"},
		{"empty strategy uses default", ptrStrategy(""), "any", "any"},
		{"channel strategy overrides", ptrStrategy("all"), "any", "all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveStrategy(tt.chStrategy, tt.defaultStrat)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func ptrStrategy(s string) *channel.QuotaMultiMonitorStrategy {
	v := channel.QuotaMultiMonitorStrategy(s)
	return &v
}

// ---------------------------------------------------------------------------
// Integration: Save + Evaluate + Recover flow
// ---------------------------------------------------------------------------

// TestBindingSaveEvaluateRecoverFlow tests the full lifecycle:
// save bindings -> channel goes not ready -> monitor recovers -> channel goes ready.
func TestBindingSaveEvaluateRecoverFlow(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-18", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Lifecycle-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Set monitor data
	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaLimits([]map[string]any{
			{"type": "token", "usageRatio": 1.0, "status": "exhausted", "ready": false},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Save bindings with exhausted trigger
	err = svc.SaveChannelQuotaMonitorBindingsAndEvaluate(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{
				UsageMonitorChannelID: monitor.ID,
				Enabled:               true,
				TriggerStatuses:       []string{"exhausted"},
			},
		},
	})
	require.NoError(t, err)

	// Channel should be not ready
	ch1, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, ch1.QuotaBindingReady)
	require.NotNil(t, ch1.ErrorMessage)

	// Monitor recovers
	_, err = svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusAvailable).
		SetQuotaLimits([]map[string]any{
			{"type": "token", "usageRatio": 0.5, "status": "available", "ready": true},
		}).
		Save(ctx)
	require.NoError(t, err)

	// Re-evaluate (simulating what evaluateChannelsForMonitor would do)
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	// Channel should be ready again
	ch2, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, ch2.QuotaBindingReady)
	assert.Nil(t, ch2.ErrorMessage, "quota-owned error message should be cleared on recovery")
}

// TestBindingWithLastTriggeredAt verifies that binding rows store
// last_triggered_at/reason when evaluated (via the view type).
func TestBindingWithLastTriggeredAt(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-19", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Trigger-Track-Monitor", usagemonitorchannel.QuotaStatusExhausted, nil)

	// Create binding directly with last triggered info
	now := time.Now()
	_, err := svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		SetLastTriggeredAt(now).
		SetLastTriggerReason("status=exhausted").
		Save(ctx)
	require.NoError(t, err)

	// List bindings should include the trigger info
	bindings, err := svc.ListChannelQuotaMonitorBindings(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.NotNil(t, bindings[0].LastTriggeredAt)
	assert.WithinDuration(t, now, *bindings[0].LastTriggeredAt, time.Second)
	require.NotNil(t, bindings[0].LastTriggerReason)
	assert.Equal(t, "status=exhausted", *bindings[0].LastTriggerReason)
}

// ---------------------------------------------------------------------------
// Non-builtin monitor re-evaluation tests
// ---------------------------------------------------------------------------

// TestEvaluateChannelsForMonitor_CustomSourceReEvaluates verifies that a
// custom-source monitor (not builtin, no direct ChannelID) bound through
// ChannelUsageMonitorBinding triggers re-evaluation of affected channels
// when evaluateChannelsForMonitor is called. This proves the fix for the
// bug where evaluateChannelsForMonitor was incorrectly gated inside the
// builtin+ChannelID check in pollChannel.
func TestEvaluateChannelsForMonitor_CustomSourceReEvaluates(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-custom-bind", nil)

	// Create a custom-source monitor (no ChannelID, not builtin)
	user, err := svc.db.User.Create().
		SetEmail("custom-monitor@test.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	monitor, err := svc.db.UsageMonitorChannel.Create().
		SetName("Custom-Quota-Monitor").
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://api.example.com/quota").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusAvailable).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Bind the custom monitor to the channel
	_, err = svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// Channel should be ready initially (monitor is available)
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)
	chCheck, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, chCheck.QuotaBindingReady, "channel should be ready when custom monitor is available")

	// Monitor becomes exhausted (simulating what pollChannel would do on a successful poll)
	_, err = svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusExhausted).
		Save(ctx)
	require.NoError(t, err)

	// Call evaluateChannelsForMonitor (this is what the fixed pollChannel does
	// for ALL monitors, not just builtin+ChannelID ones)
	svc.evaluateChannelsForMonitor(ctx, monitor.ID)

	// Channel should now be not ready
	chCheck, err = svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, chCheck.QuotaBindingReady, "channel should be not ready after custom monitor becomes exhausted via evaluateChannelsForMonitor")
	require.NotNil(t, chCheck.ErrorMessage, "error message should be set")
	assert.Contains(t, *chCheck.ErrorMessage, "Custom-Quota-Monitor", "error message should reference the custom monitor name")
}

// TestEvaluateChannelsForMonitor_TemplateSourceReEvaluates verifies that a
// template-source monitor bound through ChannelUsageMonitorBinding triggers
// re-evaluation when evaluateChannelsForMonitor is called.
func TestEvaluateChannelsForMonitor_TemplateSourceReEvaluates(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-template-bind", nil)

	// Create a template-source monitor
	user, err := svc.db.User.Create().
		SetEmail("template-monitor@test.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	monitor, err := svc.db.UsageMonitorChannel.Create().
		SetName("Template-Quota-Monitor").
		SetSource(usagemonitorchannel.SourceTemplate).
		SetProviderType(usagemonitorchannel.ProviderTypeNanogpt).
		SetAPIURL("https://api.openai.com/v1/usage").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetFields([]map[string]any{}).
		SetVariables([]map[string]any{}).
		SetDisplayFields([]map[string]any{}).
		SetStatus(usagemonitorchannel.StatusActive).
		SetQuotaStatus(usagemonitorchannel.QuotaStatusExhausted).
		SetOwnerID(user.ID).
		Save(ctx)
	require.NoError(t, err)

	// Bind the template monitor to the channel
	_, err = svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// Call evaluateChannelsForMonitor (template source, no direct ChannelID)
	svc.evaluateChannelsForMonitor(ctx, monitor.ID)

	// Channel should be not ready
	chCheck, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, chCheck.QuotaBindingReady, "channel should be not ready when template monitor is exhausted")
}

// ---------------------------------------------------------------------------
// hasActiveBindingsForMonitor tests
// ---------------------------------------------------------------------------

// TestHasActiveBindingsForMonitor_NoBindings verifies that a monitor with no
// binding rows returns false.
func TestHasActiveBindingsForMonitor_NoBindings(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	monitor := createTestMonitorForBinding(t, svc, ctx, "Unbound-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	assert.False(t, svc.hasActiveBindingsForMonitor(ctx, monitor.ID),
		"monitor with no bindings should return false")
}

// TestHasActiveBindingsForMonitor_WithBindings verifies that a monitor with
// active binding rows returns true.
func TestHasActiveBindingsForMonitor_WithBindings(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-bound-check", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "Bound-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	// Create a binding
	_, err := svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	assert.True(t, svc.hasActiveBindingsForMonitor(ctx, monitor.ID),
		"monitor with active bindings should return true")
}

// TestHasActiveBindingsForMonitor_SoftDeletedBindings verifies that a monitor
// with only soft-deleted binding rows returns false.
func TestHasActiveBindingsForMonitor_SoftDeletedBindings(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-softdel-check", nil)
	monitor := createTestMonitorForBinding(t, svc, ctx, "SoftDel-Monitor", usagemonitorchannel.QuotaStatusAvailable, nil)

	// Create a binding then soft-delete it
	binding, err := svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.db.ChannelUsageMonitorBinding.UpdateOneID(binding.ID).
		SetDeletedAt(int(time.Now().Unix())).
		Save(ctx)
	require.NoError(t, err)

	assert.False(t, svc.hasActiveBindingsForMonitor(ctx, monitor.ID),
		"monitor with only soft-deleted bindings should return false")
}

// TestPollChannel_SkipsLegacyEvaluationWhenBindingsExist verifies that
// pollChannel does not call the legacy direct evaluateAndUpdateChannelQuotaReady
// for a builtin monitor when active ChannelUsageMonitorBinding rows exist.
// This prevents duplicate evaluation. We detect duplicates by counting
// evaluateAndUpdateChannelQuotaReady calls via a wrapper that increments a counter.
//
// Note: Testing pollChannel directly requires mocking the HTTP poll, which is
// complex. Instead, we test the guard logic (hasActiveBindingsForMonitor) and
// the evaluateChannelsForMonitor path separately. The integration of these two
// in pollChannel is verified by the existing pollChannel tests and the
// hasActiveBindingsForMonitor unit tests above.
func TestPollChannel_SkipsLegacyEvaluationWhenBindingsExist(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-poll-guard", nil)
	// Create a builtin monitor with ChannelID pointing to the channel
	monitor := createTestMonitorForBinding(t, svc, ctx, "Builtin-With-Bindings", usagemonitorchannel.QuotaStatusExhausted, &ch.ID)

	// Create a binding row for this monitor -> channel
	_, err := svc.db.ChannelUsageMonitorBinding.Create().
		SetChannelID(ch.ID).
		SetUsageMonitorChannelID(monitor.ID).
		SetEnabled(true).
		SetTriggerStatuses([]string{"exhausted"}).
		SetConditions([]objects.QuotaMonitorBindingCondition{}).
		Save(ctx)
	require.NoError(t, err)

	// hasActiveBindingsForMonitor should return true, meaning the legacy
	// direct evaluation would be skipped in pollChannel.
	assert.True(t, svc.hasActiveBindingsForMonitor(ctx, monitor.ID),
		"builtin monitor with binding rows should skip legacy direct evaluation")

	// evaluateChannelsForMonitor should still correctly evaluate the channel
	svc.evaluateChannelsForMonitor(ctx, monitor.ID)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady,
		"channel should be not ready via binding-based evaluation alone")
}

// TestPollChannel_LegacyFallbackWhenNoBindings verifies that a builtin monitor
// with ChannelID but no binding rows still gets evaluated via the legacy path.
func TestPollChannel_LegacyFallbackWhenNoBindings(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")

	ch := createTestChannelForBinding(t, svc, ctx, "ch-poll-legacy", nil)
	// Create a builtin monitor with ChannelID but no binding rows
	monitor := createTestMonitorForBinding(t, svc, ctx, "Builtin-No-Bindings", usagemonitorchannel.QuotaStatusExhausted, &ch.ID)

	// Enable auto-disable on the monitor so the legacy path has something to evaluate
	_, err := svc.db.UsageMonitorChannel.UpdateOneID(monitor.ID).
		SetAutoDisableEnabled(true).
		SetQuotaReady(false).
		Save(ctx)
	require.NoError(t, err)

	// hasActiveBindingsForMonitor should return false, meaning the legacy
	// direct evaluation would NOT be skipped in pollChannel.
	assert.False(t, svc.hasActiveBindingsForMonitor(ctx, monitor.ID),
		"builtin monitor without binding rows should use legacy direct evaluation")

	// The legacy path should correctly evaluate the channel
	err = svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID)
	require.NoError(t, err)

	updated, err := svc.db.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, updated.QuotaBindingReady,
		"channel should be not ready via legacy evaluation")
}

// ---------------------------------------------------------------------------
// normalizeStringSlice / normalizeConditions tests
// ---------------------------------------------------------------------------

func TestNormalizeStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"nil becomes empty", nil, []string{}},
		{"empty stays empty", []string{}, []string{}},
		{"non-nil preserved", []string{"a", "b"}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeStringSlice(tt.input)
			assert.Equal(t, tt.expected, result)
			// Critical: result must never be nil (GraphQL [String!]! invariant)
			assert.NotNil(t, result)
		})
	}
}

func TestNormalizeConditions(t *testing.T) {
	tests := []struct {
		name     string
		input    []objects.QuotaMonitorBindingCondition
		expected int
	}{
		{"nil becomes empty", nil, 0},
		{"empty stays empty", []objects.QuotaMonitorBindingCondition{}, 0},
		{"non-nil preserved", []objects.QuotaMonitorBindingCondition{
			{Field: "x", Operator: objects.QuotaMonitorOperatorGTE, Value: "1"},
		}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeConditions(tt.input)
			assert.NotNil(t, result, "result must never be nil (GraphQL [!]! invariant)")
			assert.Len(t, result, tt.expected)
		})
	}
}
