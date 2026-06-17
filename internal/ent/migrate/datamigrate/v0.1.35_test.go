package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/migrate/datamigrate"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/objects"
)

func newV0_1_35TestContext(t *testing.T) (*ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	return client, ctx
}

func createV0_1_35TestUser(t *testing.T, client *ent.Client, ctx context.Context, email string) *ent.User {
	t.Helper()

	u, err := client.User.Create().
		SetEmail(email).
		SetPassword("hashedpassword").
		SetFirstName("Test").
		SetLastName("User").
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	return u
}

func createV0_1_35TestChannel(t *testing.T, client *ent.Client, ctx context.Context, name string, quotaBindingReady bool) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetQuotaBindingReady(quotaBindingReady).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func createV0_1_35TestMonitor(t *testing.T, client *ent.Client, ctx context.Context, ownerID int, name string, channelID *int, autoDisableEnabled bool, autoDisableThreshold float64) *ent.UsageMonitorChannel {
	t.Helper()

	creator := client.UsageMonitorChannel.Create().
		SetName(name).
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://example.com/api").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]interface{}{}).
		SetPollInterval(300).
		SetOwnerID(ownerID).
		SetFields([]map[string]interface{}{}).
		SetAutoDisableEnabled(autoDisableEnabled).
		SetAutoDisableThreshold(autoDisableThreshold)

	if channelID != nil {
		creator = creator.SetChannelID(*channelID)
	}

	mon, err := creator.Save(ctx)
	require.NoError(t, err)

	return mon
}

// TestV0_1_35_MigratesMonitorWithAutoDisableEnabled creates a UsageMonitorChannel
// linked to a Channel with auto_disable_enabled=true and verifies the migration
// creates an enabled binding with the correct condition.
func TestV0_1_35_MigratesMonitorWithAutoDisableEnabled(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner1@example.com")
	ch := createV0_1_35TestChannel(t, client, ctx, "test-channel", false)

	mon := createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-enabled", &ch.ID, true, 0.85)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify the binding was created
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		All(ctx)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "expected exactly one binding")

	b := bindings[0]
	assert.Equal(t, ch.ID, b.ChannelID)
	assert.Equal(t, mon.ID, b.UsageMonitorChannelID)
	assert.True(t, b.Enabled, "binding should be enabled when auto_disable_enabled was true")
	assert.Empty(t, b.TriggerStatuses)
	require.Len(t, b.Conditions, 1, "expected one condition")
	assert.Equal(t, "maxUsageRatio", b.Conditions[0].Field)
	assert.Equal(t, objects.QuotaMonitorOperatorGTE, b.Conditions[0].Operator)
	assert.Equal(t, "0.8500", b.Conditions[0].Value)
}

// TestV0_1_35_MigratesMonitorWithAutoDisableDisabled creates a UsageMonitorChannel
// linked to a Channel with auto_disable_enabled=false and verifies the migration
// creates a disabled binding with empty trigger_statuses and conditions.
func TestV0_1_35_MigratesMonitorWithAutoDisableDisabled(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner2@example.com")
	ch := createV0_1_35TestChannel(t, client, ctx, "test-channel-2", false)

	mon := createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-disabled", &ch.ID, false, 1.0)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindings, err := client.ChannelUsageMonitorBinding.Query().
		All(ctx)
	require.NoError(t, err)
	require.Len(t, bindings, 1)

	b := bindings[0]
	assert.Equal(t, ch.ID, b.ChannelID)
	assert.Equal(t, mon.ID, b.UsageMonitorChannelID)
	assert.False(t, b.Enabled, "binding should be disabled when auto_disable_enabled was false")
	assert.Empty(t, b.TriggerStatuses)
	assert.Empty(t, b.Conditions)
}

// TestV0_1_35_DefaultThresholdWhenZero verifies that when auto_disable_enabled=true
// but the threshold is <= 0, the default threshold of 1.0 is used.
func TestV0_1_35_DefaultThresholdWhenZero(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner3@example.com")
	ch := createV0_1_35TestChannel(t, client, ctx, "test-channel-3", false)

	createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-zero-threshold", &ch.ID, true, 0.0)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindings, err := client.ChannelUsageMonitorBinding.Query().
		All(ctx)
	require.NoError(t, err)
	require.Len(t, bindings, 1)

	require.Len(t, bindings[0].Conditions, 1)
	assert.Equal(t, "1.0000", bindings[0].Conditions[0].Value, "default threshold should be 1.0 when <= 0")
}

// TestV0_1_35_ResetsQuotaBindingReady verifies that the migration resets
// Channel.quotaBindingReady to true for all existing channels.
func TestV0_1_35_ResetsQuotaBindingReady(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner4@example.com")
	// Create a channel with quotaBindingReady=false
	ch := createV0_1_35TestChannel(t, client, ctx, "channel-not-ready", false)
	// Create a monitor linked to the channel so the migration has something to process
	createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-for-reset", &ch.ID, false, 1.0)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify the channel's quotaBindingReady is now true
	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, updated.QuotaBindingReady, "quotaBindingReady should be reset to true")
}

// TestV0_1_35_Idempotent verifies that running the migration twice creates
// only one binding per (channel_id, monitor_id) pair.
func TestV0_1_35_Idempotent(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner5@example.com")
	ch := createV0_1_35TestChannel(t, client, ctx, "channel-idempotent", false)
	mon := createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-idempotent", &ch.ID, true, 0.9)

	// First run
	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindingsAfterFirst, err := client.ChannelUsageMonitorBinding.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, bindingsAfterFirst, 1, "expected one binding after first run")

	// Second run
	err = datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindingsAfterSecond, err := client.ChannelUsageMonitorBinding.Query().All(ctx)
	require.NoError(t, err)
	assert.Len(t, bindingsAfterSecond, 1, "expected still only one binding after second run (idempotent)")

	// Verify the binding data is unchanged
	b := bindingsAfterSecond[0]
	assert.Equal(t, ch.ID, b.ChannelID)
	assert.Equal(t, mon.ID, b.UsageMonitorChannelID)
	assert.True(t, b.Enabled)
}

// TestV0_1_35_SkipsMonitorsWithoutChannelID verifies that monitors with
// no channel_id are not migrated.
func TestV0_1_35_SkipsMonitorsWithoutChannelID(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner6@example.com")
	// Create a monitor without a channel_id (nil)
	createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-no-channel", nil, true, 0.8)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindings, err := client.ChannelUsageMonitorBinding.Query().All(ctx)
	require.NoError(t, err)
	assert.Empty(t, bindings, "no bindings should be created for monitors without channel_id")
}

// TestV0_1_35_MultipleMonitorsForOneChannel verifies migration when
// multiple monitors are linked to the same channel.
func TestV0_1_35_MultipleMonitorsForOneChannel(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)

	owner := createV0_1_35TestUser(t, client, ctx, "owner7@example.com")
	ch := createV0_1_35TestChannel(t, client, ctx, "shared-channel", false)

	mon1 := createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-a", &ch.ID, true, 0.8)
	mon2 := createV0_1_35TestMonitor(t, client, ctx, owner.ID, "monitor-b", &ch.ID, false, 1.0)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindings, err := client.ChannelUsageMonitorBinding.Query().All(ctx)
	require.NoError(t, err)
	assert.Len(t, bindings, 2, "expected two bindings for the two monitors")

	// Find each binding by monitor ID
	var enabledBinding, disabledBinding *ent.ChannelUsageMonitorBinding
	for _, b := range bindings {
		if b.UsageMonitorChannelID == mon1.ID {
			enabledBinding = b
		} else if b.UsageMonitorChannelID == mon2.ID {
			disabledBinding = b
		}
	}

	require.NotNil(t, enabledBinding, "should find binding for monitor-a")
	assert.True(t, enabledBinding.Enabled)
	require.Len(t, enabledBinding.Conditions, 1)
	assert.Equal(t, "0.8000", enabledBinding.Conditions[0].Value)

	require.NotNil(t, disabledBinding, "should find binding for monitor-b")
	assert.False(t, disabledBinding.Enabled)
	assert.Empty(t, disabledBinding.Conditions)
}
