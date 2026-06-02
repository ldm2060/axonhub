package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/migrate/datamigrate"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
)

func newV0_1_10TestContext(t *testing.T) (*ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	return client, ctx
}

func createV0_1_10TestUser(t *testing.T, client *ent.Client, ctx context.Context, email string) *ent.User {
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

func createV0_1_10TestMonitorChannel(t *testing.T, client *ent.Client, ctx context.Context, ownerID int, name string, fields []map[string]interface{}) *ent.UsageMonitorChannel {
	t.Helper()

	creator := client.UsageMonitorChannel.Create().
		SetName(name).
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://example.com/api").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]interface{}{}).
		SetPollInterval(300).
		SetOwnerID(ownerID)

	if fields != nil {
		creator = creator.SetFields(fields)
	}

	ch, err := creator.Save(ctx)
	require.NoError(t, err)

	return ch
}

func TestV0_1_10_MigratesFieldsToVariablesAndDisplayFields(t *testing.T) {
	client, ctx := newV0_1_10TestContext(t)

	owner := createV0_1_10TestUser(t, client, ctx, "owner@example.com")

	// Create a channel with old-style fields
	fields := []map[string]interface{}{
		{
			"key":          "used",
			"label":        "Used",
			"path":         "$.data.used",
			"type":         "jsonpath",
			"format":       "number",
			"unit":         "tokens",
			"displayOrder": float64(0),
		},
		{
			"key":          "total",
			"label":        "Total",
			"path":         "$.data.total",
			"type":         "jsonpath",
			"format":       "fraction",
			"totalPath":    "$.data.limit",
			"displayOrder": float64(1),
		},
	}
	ch := createV0_1_10TestMonitorChannel(t, client, ctx, owner.ID, "test-channel", fields)

	err := datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.UsageMonitorChannel.Get(ctx, ch.ID)
	require.NoError(t, err)

	// Variables should be populated
	assert.NotEmpty(t, got.Variables, "variables should be populated after migration")
	// DisplayFields should be populated
	assert.NotEmpty(t, got.DisplayFields, "display_fields should be populated after migration")

	// Verify variable keys
	varKeys := make(map[string]bool)
	for _, v := range got.Variables {
		if key, ok := v["key"].(string); ok {
			varKeys[key] = true
		}
	}
	assert.True(t, varKeys["used"], "should have 'used' variable")
	assert.True(t, varKeys["total_total"], "should have 'total_total' variable from totalPath")

	// Verify display field keys
	dfKeys := make(map[string]bool)
	for _, df := range got.DisplayFields {
		if key, ok := df["key"].(string); ok {
			dfKeys[key] = true
		}
	}
	assert.True(t, dfKeys["used"], "should have 'used' display field")
	assert.True(t, dfKeys["total"], "should have 'total' display field")
}

func TestV0_1_10_SkipsAlreadyMigratedChannels(t *testing.T) {
	client, ctx := newV0_1_10TestContext(t)

	owner := createV0_1_10TestUser(t, client, ctx, "owner@example.com")

	// Create a channel that already has variables (already migrated)
	vars := []map[string]interface{}{
		{"key": "existing_var", "path": "$.data.value", "type": "jsonpath"},
	}
	dfs := []map[string]interface{}{
		{"key": "existing_var", "label": "Existing", "valueRef": "existing_var", "format": "number", "displayOrder": float64(0)},
	}

	ch, err := client.UsageMonitorChannel.Create().
		SetName("already-migrated").
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://example.com/api").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]interface{}{}).
		SetPollInterval(300).
		SetOwnerID(owner.ID).
		SetFields([]map[string]interface{}{
			{"key": "old_field", "label": "Old", "path": "$.data.old", "type": "jsonpath", "format": "number", "displayOrder": float64(0)},
		}).
		SetVariables(vars).
		SetDisplayFields(dfs).
		Save(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.UsageMonitorChannel.Get(ctx, ch.ID)
	require.NoError(t, err)

	// Variables should remain unchanged (not overwritten)
	assert.Equal(t, vars, got.Variables, "existing variables should not be overwritten")
}

func TestV0_1_10_SkipsChannelsWithNoFields(t *testing.T) {
	client, ctx := newV0_1_10TestContext(t)

	owner := createV0_1_10TestUser(t, client, ctx, "owner@example.com")

	// Create a channel with empty fields (empty default)
	ch := createV0_1_10TestMonitorChannel(t, client, ctx, owner.ID, "no-fields", []map[string]interface{}{})

	err := datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.UsageMonitorChannel.Get(ctx, ch.ID)
	require.NoError(t, err)

	// Should remain empty
	assert.Empty(t, got.Variables)
	assert.Empty(t, got.DisplayFields)
}

func TestV0_1_10_Idempotency(t *testing.T) {
	client, ctx := newV0_1_10TestContext(t)

	owner := createV0_1_10TestUser(t, client, ctx, "owner@example.com")

	fields := []map[string]interface{}{
		{
			"key":          "used",
			"label":        "Used",
			"path":         "$.data.used",
			"type":         "jsonpath",
			"format":       "number",
			"displayOrder": float64(0),
		},
	}
	ch := createV0_1_10TestMonitorChannel(t, client, ctx, owner.ID, "test-channel", fields)

	// First run
	err := datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got1, err := client.UsageMonitorChannel.Get(ctx, ch.ID)
	require.NoError(t, err)
	vars1 := got1.Variables
	dfs1 := got1.DisplayFields

	// Second run should be a no-op since variables are already set
	err = datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got2, err := client.UsageMonitorChannel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, vars1, got2.Variables, "variables should remain the same on second run")
	assert.Equal(t, dfs1, got2.DisplayFields, "display_fields should remain the same on second run")
}

func TestV0_1_10_MultipleChannels(t *testing.T) {
	client, ctx := newV0_1_10TestContext(t)

	owner := createV0_1_10TestUser(t, client, ctx, "owner@example.com")

	fields1 := []map[string]interface{}{
		{"key": "a", "label": "A", "path": "$.a", "type": "jsonpath", "format": "number", "displayOrder": float64(0)},
	}
	fields2 := []map[string]interface{}{
		{"key": "b", "label": "B", "path": "$.b", "type": "jsonpath", "format": "text", "displayOrder": float64(0)},
	}

	ch1 := createV0_1_10TestMonitorChannel(t, client, ctx, owner.ID, "channel-1", fields1)
	ch2 := createV0_1_10TestMonitorChannel(t, client, ctx, owner.ID, "channel-2", fields2)

	err := datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got1, err := client.UsageMonitorChannel.Get(ctx, ch1.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, got1.Variables)
	assert.NotEmpty(t, got1.DisplayFields)

	got2, err := client.UsageMonitorChannel.Get(ctx, ch2.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, got2.Variables)
	assert.NotEmpty(t, got2.DisplayFields)
}

func TestV0_1_10_ExpressionFieldMigration(t *testing.T) {
	client, ctx := newV0_1_10TestContext(t)

	owner := createV0_1_10TestUser(t, client, ctx, "owner@example.com")

	// Create a channel with an expression field
	fields := []map[string]interface{}{
		{
			"key":          "usage_percent",
			"label":        "Usage %",
			"type":         "jsonpath",
			"format":       "percentage",
			"expression":   "${used}/${total}*100",
			"displayOrder": float64(0),
		},
	}
	ch := createV0_1_10TestMonitorChannel(t, client, ctx, owner.ID, "expression-channel", fields)

	err := datamigrate.NewV0_1_10().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.UsageMonitorChannel.Get(ctx, ch.ID)
	require.NoError(t, err)

	// Display field should have valueRef set to the expression
	assert.NotEmpty(t, got.DisplayFields)
	df := got.DisplayFields[0]
	assert.Equal(t, "${used}/${total}*100", df["valueRef"], "valueRef should be set to the expression")
}