# Admin Dashboard and Quota Monitor Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the admin dashboard empty/infinite scroll and add channel-managed quota monitor bindings that temporarily exclude channels from routing when user-defined status or field conditions match.

**Architecture:** Add a dedicated `ChannelUsageMonitorBinding` Ent model that links channels to usage monitors and stores structured trigger rules. Backend evaluation updates the existing `Channel.quotaBindingReady` field, GraphQL exposes save/list/summary operations, and frontend channel edit UI owns configuration while the usage monitor page displays read-only summaries.

**Tech Stack:** Go 1.26+, Ent ORM, gqlgen, Gin/Fx service layer, React 19, TypeScript, TanStack Query, react-hook-form, Zod, Tailwind, SQLite migrations.

---

## Scope Check

The spec covers two pieces: the admin dashboard scroll fix and quota monitor binding. They are independent, but both are small enough to implement in one branch because the scroll fix is a single frontend layout correction and the quota feature is a cohesive backend/frontend slice. Keep commits scoped by task.

## File Structure

### Backend model and migrations

- Create `internal/objects/quota_monitor_binding.go`
  - Owns JSON-serializable binding condition types shared by Ent JSON fields and service code.
- Create `internal/ent/schema/channel_usage_monitor_binding.go`
  - Dedicated binding table between `Channel` and `UsageMonitorChannel`.
- Modify `internal/ent/schema/channel.go`
  - Add edge to quota monitor bindings.
- Modify `internal/ent/schema/usage_monitor_channel.go`
  - Add edge to quota monitor bindings.
- Modify generated Ent files via `go generate ./internal/server/gql`
  - Do not hand-edit generated Ent/gqlgen output.
- Create `internal/ent/migrate/datamigrate/v0.1.35.go`
  - Data migration for old `usage_monitor_channel.channel_id` bindings. Use the next `v0.1.x` version after existing `v0.1.34`; do not use `v0.2.0+`.
- Create `internal/ent/migrate/datamigrate/v0.1.35_test.go`
  - Local upgrade/data migration tests.
- Modify `internal/ent/migrate/datamigrate/migrator.go`
  - Register `NewV0_1_35()` after `NewV0_1_34()`.

### Backend business logic and GraphQL

- Create `internal/server/biz/quota_monitor_binding_eval.go`
  - Pure condition evaluation: status rules, field values, virtual fields, operators, any/all aggregation.
- Create `internal/server/biz/quota_monitor_binding_eval_test.go`
  - Unit tests for pure evaluation.
- Create `internal/server/biz/channel_quota_monitor_binding.go`
  - Service methods to list/save channel bindings, list monitor summaries, and re-evaluate affected channels.
- Create `internal/server/biz/channel_quota_monitor_binding_test.go`
  - Integration-style service tests using Ent test client.
- Modify `internal/server/biz/usage_monitor_internal.go`
  - Replace existing threshold-only aggregation with binding-table evaluation while keeping `updateChannelQuotaBindingReady` behavior.
- Modify `internal/server/biz/usage_monitor.go`
  - Re-evaluate affected channels after monitor refresh/update paths.
- Modify `internal/server/gql/usage_monitor.graphql`
  - Add GraphQL types, inputs, queries, and mutation for binding config and summaries.
- Modify `internal/server/gql/usage_monitor.resolvers.go`
  - Add resolver methods that call the service layer.

### Frontend data and UI

- Modify `frontend/src/features/channels/data/schema.ts`
  - Add Zod schemas/types for quota binding config.
- Modify `frontend/src/features/channels/data/channels.ts`
  - Add GraphQL fragments/queries/mutations and TanStack hooks for binding config.
- Create `frontend/src/features/channels/components/channel-quota-monitor-binding.tsx`
  - Focused editor component for channel quota monitor binding settings.
- Modify `frontend/src/features/channels/components/channels-action-dialog.tsx`
  - Load binding config, render editor, and save binding config after channel save.
- Modify `frontend/src/features/usage-monitor/data/schema.ts`
  - Add summary types for read-only monitor binding display.
- Modify `frontend/src/features/usage-monitor/data/usage-monitor.ts`
  - Query monitor binding summaries.
- Create `frontend/src/features/usage-monitor/components/monitor-binding-summary.tsx`
  - Read-only summary component.
- Modify `frontend/src/features/usage-monitor/components/monitor-card.tsx`
  - Render the summary component.
- Modify locale files:
  - `frontend/src/locales/en/channels.json`
  - `frontend/src/locales/zh-CN/channels.json`
  - `frontend/src/locales/en/usage-monitor.json`
  - `frontend/src/locales/zh-CN/usage-monitor.json`
- Modify `frontend/src/features/dashboard/index.tsx` or the admin dashboard route wrapper if inspection shows the route wrapper owns the extra scroll.

---

## Task 1: Add Backend Binding Types and Ent Schema

**Files:**
- Create: `internal/objects/quota_monitor_binding.go`
- Create: `internal/ent/schema/channel_usage_monitor_binding.go`
- Modify: `internal/ent/schema/channel.go`
- Modify: `internal/ent/schema/usage_monitor_channel.go`
- Generated by command: `internal/ent/**`, `internal/server/gql/generated.go`, `internal/server/gql/ent.graphql`, `internal/server/gql/models_gen.go` as produced by generation

- [ ] **Step 1: Create shared condition types**

Create `internal/objects/quota_monitor_binding.go` with this exact content:

```go
package objects

// QuotaMonitorConditionOperator is a controlled comparison operator for
// quota-monitor field conditions. It is stored as JSON and never executed as code.
type QuotaMonitorConditionOperator string

const (
	QuotaMonitorOperatorLT          QuotaMonitorConditionOperator = "<"
	QuotaMonitorOperatorLTE         QuotaMonitorConditionOperator = "<="
	QuotaMonitorOperatorEQ          QuotaMonitorConditionOperator = "="
	QuotaMonitorOperatorNEQ         QuotaMonitorConditionOperator = "!="
	QuotaMonitorOperatorGTE         QuotaMonitorConditionOperator = ">="
	QuotaMonitorOperatorGT          QuotaMonitorConditionOperator = ">"
	QuotaMonitorOperatorContains    QuotaMonitorConditionOperator = "contains"
	QuotaMonitorOperatorNotContains QuotaMonitorConditionOperator = "not_contains"
)

// QuotaMonitorBindingCondition describes one structured condition such as
// remaining <= 0. Field can reference parsedData keys, lastPollData keys, or
// the virtual field maxUsageRatio.
type QuotaMonitorBindingCondition struct {
	Field    string                        `json:"field"`
	Operator QuotaMonitorConditionOperator `json:"operator"`
	Value    string                        `json:"value"`
}
```

- [ ] **Step 2: Add Ent schema for the dedicated binding table**

Create `internal/ent/schema/channel_usage_monitor_binding.go` with this exact content:

```go
package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/objects"
)

// ChannelUsageMonitorBinding stores user-defined quota monitor rules for one channel.
type ChannelUsageMonitorBinding struct {
	ent.Schema
}

func (ChannelUsageMonitorBinding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (ChannelUsageMonitorBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "usage_monitor_channel_id", "deleted_at").
			Unique().
			StorageKey("channel_usage_monitor_bindings_unique_active"),
		index.Fields("channel_id", "deleted_at").StorageKey("channel_usage_monitor_bindings_by_channel"),
		index.Fields("usage_monitor_channel_id", "deleted_at").StorageKey("channel_usage_monitor_bindings_by_monitor"),
	}
}

func (ChannelUsageMonitorBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Comment("Channel affected by this quota monitor binding"),
		field.Int("usage_monitor_channel_id").Comment("Usage monitor that provides quota state"),
		field.Bool("enabled").Default(true).Comment("Whether this binding participates in quota-ready evaluation"),
		field.Strings("trigger_statuses").Optional().Default([]string{}).Comment("Quota statuses that trigger this binding"),
		field.JSON("conditions", []objects.QuotaMonitorBindingCondition{}).
			Optional().
			Default([]objects.QuotaMonitorBindingCondition{}).
			Annotations(entgql.Type("Map")).
			Comment("Structured OR conditions evaluated against monitor fields"),
		field.Time("last_triggered_at").Optional().Nillable().Comment("Last time this binding matched"),
		field.String("last_trigger_reason").Optional().Nillable().Comment("Last human-readable match reason"),
	}
}

func (ChannelUsageMonitorBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("quota_monitor_bindings").
			Field("channel_id").
			Unique().
			Required(),
		edge.From("usage_monitor_channel", UsageMonitorChannel.Type).
			Ref("channel_bindings").
			Field("usage_monitor_channel_id").
			Unique().
			Required(),
	}
}
```

- [ ] **Step 3: Add Channel edge**

In `internal/ent/schema/channel.go`, add this edge inside `func (Channel) Edges() []ent.Edge` after the existing `edge.To("usage_monitor_channels", UsageMonitorChannel.Type),` line:

```go
			edge.To("quota_monitor_bindings", ChannelUsageMonitorBinding.Type),
```

- [ ] **Step 4: Add UsageMonitorChannel edge**

In `internal/ent/schema/usage_monitor_channel.go`, add this edge inside `func (UsageMonitorChannel) Edges() []ent.Edge` after the existing channel edge:

```go
			edge.To("channel_bindings", ChannelUsageMonitorBinding.Type),
```

- [ ] **Step 5: Generate Ent and gqlgen output**

Run:

```powershell
go generate ./internal/server/gql
```

Expected: command exits 0 and generated files include `channelusagemonitorbinding` Ent package plus GraphQL model changes.

- [ ] **Step 6: Run focused backend build**

Run:

```powershell
go build ./internal/ent ./internal/server/gql ./internal/objects
```

Expected: exits 0.

- [ ] **Step 7: Commit schema foundation**

Run:

```powershell
git status --short
git add internal/objects/quota_monitor_binding.go internal/ent/schema/channel_usage_monitor_binding.go internal/ent/schema/channel.go internal/ent/schema/usage_monitor_channel.go internal/ent internal/server/gql
git commit -m "feat(quota): add monitor binding schema"
```

Expected: commit succeeds. Do not add `.exe` files.

---

## Task 2: Add Data Migration and Upgrade Tests

**Files:**
- Create: `internal/ent/migrate/datamigrate/v0.1.35.go`
- Create: `internal/ent/migrate/datamigrate/v0.1.35_test.go`
- Modify: `internal/ent/migrate/datamigrate/migrator.go`

- [ ] **Step 1: Write failing migration tests**

Create `internal/ent/migrate/datamigrate/v0.1.35_test.go` with this content. If generated package names differ after Task 1, use the generated import paths shown by the compiler and keep the test logic unchanged.

```go
package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/migrate/datamigrate"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
)

func newV0_1_35TestContext(t *testing.T) (*ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:v0135?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	return client, ctx
}

func createV0_1_35User(t *testing.T, client *ent.Client, ctx context.Context) *ent.User {
	t.Helper()

	u, err := client.User.Create().
		SetEmail("quota-upgrade@example.com").
		SetPassword("hashedpassword").
		SetFirstName("Quota").
		SetLastName("Upgrade").
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)
	return u
}

func createV0_1_35Channel(t *testing.T, client *ent.Client, ctx context.Context, name string, ownerID int) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://example.com/v1").
		SetCredentials(map[string]any{"apiKeys": []string{"sk-test"}}).
		SetSupportedModels([]string{"gpt-test"}).
		SetDefaultTestModel("gpt-test").
		SetOwnerID(ownerID).
		SetQuotaBindingReady(false).
		Save(ctx)
	require.NoError(t, err)
	return ch
}

func createV0_1_35Monitor(t *testing.T, client *ent.Client, ctx context.Context, ownerID int, channelID int, autoDisable bool) *ent.UsageMonitorChannel {
	t.Helper()

	create := client.UsageMonitorChannel.Create().
		SetName("old-monitor").
		SetSource(usagemonitorchannel.SourceCustom).
		SetChannelID(channelID).
		SetAPIURL("https://quota.example.com").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetPollInterval(300).
		SetOwnerID(ownerID).
		SetAutoDisableEnabled(autoDisable)

	if autoDisable {
		create = create.SetAutoDisableThreshold(0.8).SetAutoEnableThreshold(0.7)
	}

	monitor, err := create.Save(ctx)
	require.NoError(t, err)
	return monitor
}

func TestV0_1_35_MigratesOldChannelIDRelationship(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)
	owner := createV0_1_35User(t, client, ctx)
	ch := createV0_1_35Channel(t, client, ctx, "bound-channel", owner.ID)
	monitor := createV0_1_35Monitor(t, client, ctx, owner.ID, ch.ID, false)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.ChannelID(ch.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, monitor.ID, bindings[0].UsageMonitorChannelID)
	assert.False(t, bindings[0].Enabled, "old relationships without active auto-disable are preserved but ineffective")
	assert.Empty(t, bindings[0].TriggerStatuses)
	assert.Empty(t, bindings[0].Conditions)

	gotChannel, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, gotChannel.QuotaBindingReady, "migration must not leave existing channels excluded from routing")
}

func TestV0_1_35_MigratesOldAutoDisableThresholdAsVirtualCondition(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)
	owner := createV0_1_35User(t, client, ctx)
	ch := createV0_1_35Channel(t, client, ctx, "auto-disable-channel", owner.ID)
	createV0_1_35Monitor(t, client, ctx, owner.ID, ch.ID, true)

	err := datamigrate.NewV0_1_35().Migrate(ctx, client)
	require.NoError(t, err)

	binding, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.ChannelID(ch.ID)).
		Only(ctx)
	require.NoError(t, err)
	assert.True(t, binding.Enabled)
	require.Len(t, binding.Conditions, 1)
	assert.Equal(t, "maxUsageRatio", binding.Conditions[0].Field)
	assert.Equal(t, ">=", string(binding.Conditions[0].Operator))
	assert.Equal(t, "0.8", binding.Conditions[0].Value)
}

func TestV0_1_35_IsIdempotent(t *testing.T) {
	client, ctx := newV0_1_35TestContext(t)
	owner := createV0_1_35User(t, client, ctx)
	ch := createV0_1_35Channel(t, client, ctx, "idempotent-channel", owner.ID)
	createV0_1_35Monitor(t, client, ctx, owner.ID, ch.ID, false)

	require.NoError(t, datamigrate.NewV0_1_35().Migrate(ctx, client))
	require.NoError(t, datamigrate.NewV0_1_35().Migrate(ctx, client))

	count, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.ChannelID(ch.ID)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
```

- [ ] **Step 2: Run migration test to verify it fails**

Run:

```powershell
go test ./internal/ent/migrate/datamigrate -run V0_1_35 -count=1
```

Expected: FAIL because `NewV0_1_35` and migration code do not exist.

- [ ] **Step 3: Implement migration**

Create `internal/ent/migrate/datamigrate/v0.1.35.go` with this content:

```go
package datamigrate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/objects"
)

type V0_1_35 struct{}

func NewV0_1_35() DataMigrator {
	return &V0_1_35{}
}

func (v *V0_1_35) Version() string {
	return "v0.1.35"
}

func (v *V0_1_35) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	monitors, err := client.UsageMonitorChannel.Query().
		Where(
			usagemonitorchannel.ChannelIDNotNil(),
			usagemonitorchannel.DeletedAtEQ(0),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query existing usage monitor channel bindings: %w", err)
	}

	created := 0
	for _, monitor := range monitors {
		if monitor.ChannelID == nil {
			continue
		}

		exists, err := client.ChannelUsageMonitorBinding.Query().
			Where(
				channelusagemonitorbinding.ChannelID(*monitor.ChannelID),
				channelusagemonitorbinding.UsageMonitorChannelID(monitor.ID),
				channelusagemonitorbinding.DeletedAtEQ(0),
			).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("check quota monitor binding for channel %d monitor %d: %w", *monitor.ChannelID, monitor.ID, err)
		}
		if exists {
			continue
		}

		conditions := []objects.QuotaMonitorBindingCondition{}
		enabled := false
		if monitor.AutoDisableEnabled {
			enabled = true
			threshold := monitor.AutoDisableThreshold
			if threshold <= 0 {
				threshold = 1.0
			}
			conditions = append(conditions, objects.QuotaMonitorBindingCondition{
				Field:    "maxUsageRatio",
				Operator: objects.QuotaMonitorOperatorGTE,
				Value:    strconv.FormatFloat(threshold, 'f', -1, 64),
			})
		}

		_, err = client.ChannelUsageMonitorBinding.Create().
			SetChannelID(*monitor.ChannelID).
			SetUsageMonitorChannelID(monitor.ID).
			SetEnabled(enabled).
			SetTriggerStatuses([]string{}).
			SetConditions(conditions).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create quota monitor binding for channel %d monitor %d: %w", *monitor.ChannelID, monitor.ID, err)
		}
		created++
	}

	updatedChannels, err := client.Channel.Update().SetQuotaBindingReady(true).Save(ctx)
	if err != nil {
		return fmt.Errorf("reset quota binding ready during migration: %w", err)
	}

	log.Info(ctx, "migrated quota monitor bindings",
		log.Int("created", created),
		log.Int("channels_ready", updatedChannels),
	)
	return nil
}
```

- [ ] **Step 4: Register migration**

In `internal/ent/migrate/datamigrate/migrator.go`, change `NewMigrator` to include `NewV0_1_35()` after `NewV0_1_34()`:

```go
func NewMigrator(client *ent.Client) *Migrator {
	migrator := NewMigratorWithoutRegistrations(client)
	migrator.Register(NewV0_1_10())
	migrator.Register(NewV0_1_34())
	migrator.Register(NewV0_1_35())

	return migrator
}
```

- [ ] **Step 5: Run migration tests**

Run:

```powershell
go test ./internal/ent/migrate/datamigrate -run "V0_1_35|Migrator" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit migration**

Run:

```powershell
git add internal/ent/migrate/datamigrate/v0.1.35.go internal/ent/migrate/datamigrate/v0.1.35_test.go internal/ent/migrate/datamigrate/migrator.go
git commit -m "feat(quota): migrate monitor bindings"
```

Expected: commit succeeds.

---

## Task 3: Implement Pure Quota Binding Evaluation

**Files:**
- Create: `internal/server/biz/quota_monitor_binding_eval.go`
- Create: `internal/server/biz/quota_monitor_binding_eval_test.go`

- [ ] **Step 1: Write failing pure evaluation tests**

Create `internal/server/biz/quota_monitor_binding_eval_test.go` with this content:

```go
package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/objects"
)

func TestEvaluateBinding_StatusMatch(t *testing.T) {
	result := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
		MonitorName:     "monitor-a",
		QuotaStatus:     "exhausted",
		TriggerStatuses: []string{"warning", "exhausted"},
	})

	assert.True(t, result.Effective)
	assert.True(t, result.Matched)
	assert.Contains(t, result.Reason, "status exhausted")
}

func TestEvaluateBinding_NumericConditionMatch(t *testing.T) {
	result := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
		MonitorName: "monitor-a",
		ParsedFields: map[string]any{
			"remaining": "0",
		},
		Conditions: []objects.QuotaMonitorBindingCondition{{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"}},
	})

	assert.True(t, result.Effective)
	assert.True(t, result.Matched)
	assert.Contains(t, result.Reason, "remaining <= 0")
}

func TestEvaluateBinding_TextConditionMatch(t *testing.T) {
	result := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
		MonitorName: "monitor-a",
		ParsedFields: map[string]any{
			"plan": "monthly exhausted plan",
		},
		Conditions: []objects.QuotaMonitorBindingCondition{{Field: "plan", Operator: objects.QuotaMonitorOperatorContains, Value: "exhausted"}},
	})

	assert.True(t, result.Effective)
	assert.True(t, result.Matched)
}

func TestEvaluateBinding_InvalidNumericDoesNotMatch(t *testing.T) {
	result := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
		MonitorName: "monitor-a",
		ParsedFields: map[string]any{
			"remaining": "not-a-number",
		},
		Conditions: []objects.QuotaMonitorBindingCondition{{Field: "remaining", Operator: objects.QuotaMonitorOperatorLTE, Value: "0"}},
	})

	assert.True(t, result.Effective)
	assert.False(t, result.Matched)
	assert.Contains(t, result.Diagnostics, "remaining")
}

func TestEvaluateBinding_EmptyRulesIneffective(t *testing.T) {
	result := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{MonitorName: "monitor-a"})

	assert.False(t, result.Effective)
	assert.False(t, result.Matched)
}

func TestAggregateBindingResultsAny(t *testing.T) {
	ready, reason := aggregateQuotaMonitorBindingResults("any", []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: false},
		{Effective: true, Matched: true, Reason: "monitor-b: exhausted"},
	})

	assert.False(t, ready)
	assert.Equal(t, "monitor-b: exhausted", reason)
}

func TestAggregateBindingResultsAll(t *testing.T) {
	ready, reason := aggregateQuotaMonitorBindingResults("all", []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: true, Reason: "monitor-a: exhausted"},
		{Effective: true, Matched: false},
	})

	assert.True(t, ready)
	assert.Empty(t, reason)

	ready, reason = aggregateQuotaMonitorBindingResults("all", []quotaMonitorBindingRuleResult{
		{Effective: true, Matched: true, Reason: "monitor-a: exhausted"},
		{Effective: true, Matched: true, Reason: "monitor-b: remaining <= 0"},
	})

	assert.False(t, ready)
	require.NotEmpty(t, reason)
	assert.Contains(t, reason, "monitor-a")
	assert.Contains(t, reason, "monitor-b")
}

func TestMaxUsageRatioVirtualField(t *testing.T) {
	fields := flattenQuotaMonitorFields(map[string]any{}, []map[string]any{
		{"usageRatio": 0.25},
		{"usageRatio": 0.91},
	})

	assert.Equal(t, 0.91, fields["maxUsageRatio"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/server/biz -run "EvaluateBinding|AggregateBinding|MaxUsageRatio" -count=1
```

Expected: FAIL because evaluation functions are missing.

- [ ] **Step 3: Implement evaluation helpers**

Create `internal/server/biz/quota_monitor_binding_eval.go` with this content:

```go
package biz

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ldm2060/axonhub/internal/objects"
)

type quotaMonitorBindingRuleInput struct {
	MonitorName     string
	QuotaStatus     string
	TriggerStatuses []string
	Conditions      []objects.QuotaMonitorBindingCondition
	ParsedFields    map[string]any
	LastPollData    map[string]any
	QuotaLimits     []map[string]any
}

type quotaMonitorBindingRuleResult struct {
	Effective    bool
	Matched      bool
	Reason       string
	Diagnostics  string
	MatchedField string
}

func evaluateQuotaMonitorBindingRule(input quotaMonitorBindingRuleInput) quotaMonitorBindingRuleResult {
	statusSet := make(map[string]struct{}, len(input.TriggerStatuses))
	for _, status := range input.TriggerStatuses {
		status = strings.TrimSpace(status)
		if status != "" {
			statusSet[status] = struct{}{}
		}
	}

	effective := len(statusSet) > 0 || len(input.Conditions) > 0
	if !effective {
		return quotaMonitorBindingRuleResult{Effective: false}
	}

	if _, ok := statusSet[input.QuotaStatus]; ok {
		return quotaMonitorBindingRuleResult{
			Effective: true,
			Matched:   true,
			Reason:    fmt.Sprintf("%s: status %s", input.MonitorName, input.QuotaStatus),
		}
	}

	fields := flattenQuotaMonitorFields(input.ParsedFields, input.QuotaLimits)
	for key, value := range input.LastPollData {
		if _, exists := fields[key]; !exists {
			fields[key] = value
		}
	}

	diagnostics := []string{}
	for _, condition := range input.Conditions {
		field := strings.TrimSpace(condition.Field)
		if field == "" {
			diagnostics = append(diagnostics, "empty field")
			continue
		}
		actual, ok := fields[field]
		if !ok {
			diagnostics = append(diagnostics, fmt.Sprintf("%s missing", field))
			continue
		}
		matched, diagnostic := compareQuotaBindingCondition(actual, condition.Operator, condition.Value)
		if diagnostic != "" {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: %s", field, diagnostic))
		}
		if matched {
			return quotaMonitorBindingRuleResult{
				Effective:    true,
				Matched:      true,
				Reason:       fmt.Sprintf("%s: %s %s %s", input.MonitorName, field, condition.Operator, condition.Value),
				Diagnostics:  strings.Join(diagnostics, "; "),
				MatchedField: field,
			}
		}
	}

	return quotaMonitorBindingRuleResult{Effective: true, Diagnostics: strings.Join(diagnostics, "; ")}
}

func flattenQuotaMonitorFields(parsedFields map[string]any, quotaLimits []map[string]any) map[string]any {
	fields := make(map[string]any, len(parsedFields)+1)
	for key, value := range parsedFields {
		fields[key] = value
	}

	maxUsageRatio := 0.0
	for _, limit := range quotaLimits {
		ratio, ok := numberFromAny(limit["usageRatio"])
		if ok && ratio > maxUsageRatio {
			maxUsageRatio = ratio
		}
	}
	fields["maxUsageRatio"] = maxUsageRatio
	return fields
}

func compareQuotaBindingCondition(actual any, operator objects.QuotaMonitorConditionOperator, expected string) (bool, string) {
	switch operator {
	case objects.QuotaMonitorOperatorLT, objects.QuotaMonitorOperatorLTE, objects.QuotaMonitorOperatorGTE, objects.QuotaMonitorOperatorGT:
		actualNum, ok := numberFromAny(actual)
		if !ok {
			return false, "actual value is not numeric"
		}
		expectedNum, err := strconv.ParseFloat(strings.TrimSpace(expected), 64)
		if err != nil {
			return false, "expected value is not numeric"
		}
		switch operator {
		case objects.QuotaMonitorOperatorLT:
			return actualNum < expectedNum, ""
		case objects.QuotaMonitorOperatorLTE:
			return actualNum <= expectedNum, ""
		case objects.QuotaMonitorOperatorGTE:
			return actualNum >= expectedNum, ""
		case objects.QuotaMonitorOperatorGT:
			return actualNum > expectedNum, ""
		}
	case objects.QuotaMonitorOperatorEQ:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(actual)), strings.TrimSpace(expected)), ""
	case objects.QuotaMonitorOperatorNEQ:
		return !strings.EqualFold(strings.TrimSpace(fmt.Sprint(actual)), strings.TrimSpace(expected)), ""
	case objects.QuotaMonitorOperatorContains:
		return strings.Contains(fmt.Sprint(actual), expected), ""
	case objects.QuotaMonitorOperatorNotContains:
		return !strings.Contains(fmt.Sprint(actual), expected), ""
	}
	return false, fmt.Sprintf("unsupported operator %q", operator)
}

func numberFromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case jsonNumber:
		f, err := strconv.ParseFloat(string(v), 64)
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(v, "%")), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

type jsonNumber string

func aggregateQuotaMonitorBindingResults(strategy string, results []quotaMonitorBindingRuleResult) (bool, string) {
	effective := make([]quotaMonitorBindingRuleResult, 0, len(results))
	for _, result := range results {
		if result.Effective {
			effective = append(effective, result)
		}
	}
	if len(effective) == 0 {
		return true, ""
	}

	if strategy == "all" {
		reasons := []string{}
		for _, result := range effective {
			if !result.Matched {
				return true, ""
			}
			if result.Reason != "" {
				reasons = append(reasons, result.Reason)
			}
		}
		return false, strings.Join(reasons, "; ")
	}

	for _, result := range effective {
		if result.Matched {
			return false, result.Reason
		}
	}
	return true, ""
}
```

- [ ] **Step 4: Fix `json.Number` support**

Replace the custom `jsonNumber` branch by importing `encoding/json` and using `json.Number`. Change the import block to:

```go
import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ldm2060/axonhub/internal/objects"
)
```

Then replace the `jsonNumber` branch and remove `type jsonNumber string`:

```go
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
```

- [ ] **Step 5: Run pure evaluation tests**

Run:

```powershell
go test ./internal/server/biz -run "EvaluateBinding|AggregateBinding|MaxUsageRatio" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit pure evaluator**

Run:

```powershell
git add internal/server/biz/quota_monitor_binding_eval.go internal/server/biz/quota_monitor_binding_eval_test.go
git commit -m "feat(quota): evaluate monitor binding rules"
```

Expected: commit succeeds.

---

## Task 4: Add Binding Service Methods and Backend Integration Tests

**Files:**
- Create: `internal/server/biz/channel_quota_monitor_binding.go`
- Create: `internal/server/biz/channel_quota_monitor_binding_test.go`
- Modify: `internal/server/biz/usage_monitor_internal.go`
- Modify: `internal/server/biz/usage_monitor.go`

- [ ] **Step 1: Write service integration tests**

Create `internal/server/biz/channel_quota_monitor_binding_test.go` with this content. After Task 1 generation, use generated enum constants if compiler names differ.

```go
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
)

func newQuotaBindingServiceTest(t *testing.T) (*ent.Client, context.Context, *UsageMonitorService) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:quota_binding_service?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewUsageMonitorService(UsageMonitorServiceParams{Ent: client})
	return client, ctx, svc
}

func createQuotaBindingServiceUser(t *testing.T, client *ent.Client, ctx context.Context) *ent.User {
	t.Helper()
	u, err := client.User.Create().
		SetEmail("quota-binding-service@example.com").
		SetPassword("hashedpassword").
		SetFirstName("Quota").
		SetLastName("Binding").
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)
	return u
}

func createQuotaBindingServiceChannel(t *testing.T, client *ent.Client, ctx context.Context, ownerID int) *ent.Channel {
	t.Helper()
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("quota-bound-channel").
		SetBaseURL("https://example.com/v1").
		SetCredentials(map[string]any{"apiKeys": []string{"sk-test"}}).
		SetSupportedModels([]string{"gpt-test"}).
		SetDefaultTestModel("gpt-test").
		SetStatus(channel.StatusEnabled).
		SetOwnerID(ownerID).
		Save(ctx)
	require.NoError(t, err)
	return ch
}

func createQuotaBindingServiceMonitor(t *testing.T, client *ent.Client, ctx context.Context, ownerID int, name string, quotaStatus usagemonitorchannel.QuotaStatus, quotaLimits []map[string]any) *ent.UsageMonitorChannel {
	t.Helper()
	m, err := client.UsageMonitorChannel.Create().
		SetName(name).
		SetSource(usagemonitorchannel.SourceCustom).
		SetAPIURL("https://quota.example.com").
		SetAPIMethod(usagemonitorchannel.APIMethodGET).
		SetAPIHeaders(map[string]any{}).
		SetPollInterval(300).
		SetOwnerID(ownerID).
		SetStatus(usagemonitorchannel.StatusActive).
		SetQuotaStatus(quotaStatus).
		SetQuotaLimits(quotaLimits).
		SetLastPollData(map[string]any{"remaining": 0}).
		Save(ctx)
	require.NoError(t, err)
	return m
}

func TestSaveChannelQuotaMonitorBindings_ReplacesBindingsAndEvaluates(t *testing.T) {
	client, ctx, svc := newQuotaBindingServiceTest(t)
	owner := createQuotaBindingServiceUser(t, client, ctx)
	ch := createQuotaBindingServiceChannel(t, client, ctx, owner.ID)
	monitor := createQuotaBindingServiceMonitor(t, client, ctx, owner.ID, "monitor-a", usagemonitorchannel.QuotaStatusExhausted, nil)

	err := svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{{
			UsageMonitorChannelID: monitor.ID,
			Enabled:               true,
			TriggerStatuses:       []string{"exhausted"},
			Conditions:            []objects.QuotaMonitorBindingCondition{},
		}},
	})
	require.NoError(t, err)

	got, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, got.QuotaBindingReady)
	assert.Contains(t, *got.ErrorMessage, "monitor-a")

	bindings, err := svc.ListChannelQuotaMonitorBindings(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, monitor.ID, bindings[0].UsageMonitorChannelID)
}

func TestEvaluateAndUpdateChannelQuotaReady_AllStrategyRequiresAllMatches(t *testing.T) {
	client, ctx, svc := newQuotaBindingServiceTest(t)
	owner := createQuotaBindingServiceUser(t, client, ctx)
	ch := createQuotaBindingServiceChannel(t, client, ctx, owner.ID)
	_, err := client.Channel.UpdateOneID(ch.ID).SetQuotaMultiMonitorStrategy(channel.QuotaMultiMonitorStrategyAll).Save(ctx)
	require.NoError(t, err)
	monitorA := createQuotaBindingServiceMonitor(t, client, ctx, owner.ID, "monitor-a", usagemonitorchannel.QuotaStatusExhausted, nil)
	monitorB := createQuotaBindingServiceMonitor(t, client, ctx, owner.ID, "monitor-b", usagemonitorchannel.QuotaStatusAvailable, nil)

	require.NoError(t, svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "all",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{UsageMonitorChannelID: monitorA.ID, Enabled: true, TriggerStatuses: []string{"exhausted"}},
			{UsageMonitorChannelID: monitorB.ID, Enabled: true, TriggerStatuses: []string{"exhausted"}},
		},
	}))

	got, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, got.QuotaBindingReady)

	_, err = client.UsageMonitorChannel.UpdateOneID(monitorB.ID).SetQuotaStatus(usagemonitorchannel.QuotaStatusExhausted).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID))

	got, err = client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, got.QuotaBindingReady)
}

func TestEvaluateAndUpdateChannelQuotaReady_FieldConditionRecovery(t *testing.T) {
	client, ctx, svc := newQuotaBindingServiceTest(t)
	owner := createQuotaBindingServiceUser(t, client, ctx)
	ch := createQuotaBindingServiceChannel(t, client, ctx, owner.ID)
	monitor := createQuotaBindingServiceMonitor(t, client, ctx, owner.ID, "monitor-a", usagemonitorchannel.QuotaStatusAvailable, []map[string]any{{"usageRatio": 1.0}})

	require.NoError(t, svc.SaveChannelQuotaMonitorBindings(ctx, ch.ID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{{
			UsageMonitorChannelID: monitor.ID,
			Enabled:               true,
			Conditions: []objects.QuotaMonitorBindingCondition{{
				Field:    "maxUsageRatio",
				Operator: objects.QuotaMonitorOperatorGTE,
				Value:    "1",
			}},
		}},
	}))

	got, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, got.QuotaBindingReady)

	_, err = client.UsageMonitorChannel.UpdateOneID(monitor.ID).SetQuotaLimits([]map[string]any{{"usageRatio": 0.5}}).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.evaluateAndUpdateChannelQuotaReady(ctx, ch.ID))

	got, err = client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, got.QuotaBindingReady)
	assert.Nil(t, got.ErrorMessage)
}
```

- [ ] **Step 2: Run service tests to verify they fail**

Run:

```powershell
go test ./internal/server/biz -run "SaveChannelQuotaMonitorBindings|EvaluateAndUpdateChannelQuotaReady" -count=1
```

Expected: FAIL because service methods are missing or still threshold-only.

- [ ] **Step 3: Implement service input/output types and save/list methods**

Create `internal/server/biz/channel_quota_monitor_binding.go` with this content:

```go
package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/objects"
)

type SaveChannelQuotaMonitorBindingInput struct {
	UsageMonitorChannelID int
	Enabled               bool
	TriggerStatuses       []string
	Conditions            []objects.QuotaMonitorBindingCondition
}

type SaveChannelQuotaMonitorBindingsInput struct {
	Strategy string
	Bindings []SaveChannelQuotaMonitorBindingInput
}

type ChannelQuotaMonitorBindingView struct {
	ID                    int
	ChannelID             int
	UsageMonitorChannelID int
	UsageMonitorName      string
	Enabled               bool
	TriggerStatuses       []string
	Conditions            []objects.QuotaMonitorBindingCondition
	LastTriggeredAt       *time.Time
	LastTriggerReason     *string
}

type UsageMonitorBindingSummary struct {
	ChannelID             int
	ChannelName           string
	UsageMonitorChannelID int
	Strategy              string
	Enabled               bool
	TriggerStatuses       []string
	Conditions            []objects.QuotaMonitorBindingCondition
	Matched               bool
	Reason                string
}

func (svc *UsageMonitorService) SaveChannelQuotaMonitorBindings(ctx context.Context, channelID int, input SaveChannelQuotaMonitorBindingsInput) error {
	ctx = authz.WithSystemBypass(ctx, "quota-monitor-binding")
	client := svc.entFromContext(ctx)

	strategy := input.Strategy
	if strategy != string(channel.QuotaMultiMonitorStrategyAll) {
		strategy = string(channel.QuotaMultiMonitorStrategyAny)
	}
	strategyEnum := channel.QuotaMultiMonitorStrategy(strategy)

	err := RunInTransaction(ctx, client, func(tx *ent.Tx) error {
		if _, err := tx.Channel.UpdateOneID(channelID).SetQuotaMultiMonitorStrategy(strategyEnum).Save(ctx); err != nil {
			return fmt.Errorf("update channel quota monitor strategy: %w", err)
		}

		if _, err := tx.ChannelUsageMonitorBinding.Delete().Where(channelusagemonitorbinding.ChannelID(channelID)).Exec(ctx); err != nil {
			return fmt.Errorf("replace quota monitor bindings: %w", err)
		}

		for _, binding := range input.Bindings {
			if binding.UsageMonitorChannelID == 0 {
				continue
			}
			_, err := tx.ChannelUsageMonitorBinding.Create().
				SetChannelID(channelID).
				SetUsageMonitorChannelID(binding.UsageMonitorChannelID).
				SetEnabled(binding.Enabled).
				SetTriggerStatuses(cleanQuotaStatuses(binding.TriggerStatuses)).
				SetConditions(cleanQuotaConditions(binding.Conditions)).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("create quota monitor binding: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return svc.evaluateAndUpdateChannelQuotaReady(ctx, channelID)
}

func (svc *UsageMonitorService) ListChannelQuotaMonitorBindings(ctx context.Context, channelID int) ([]ChannelQuotaMonitorBindingView, error) {
	client := svc.entFromContext(ctx)
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.ChannelID(channelID)).
		WithUsageMonitorChannel().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channel quota monitor bindings: %w", err)
	}

	views := make([]ChannelQuotaMonitorBindingView, 0, len(bindings))
	for _, binding := range bindings {
		view := ChannelQuotaMonitorBindingView{
			ID:                    binding.ID,
			ChannelID:             binding.ChannelID,
			UsageMonitorChannelID: binding.UsageMonitorChannelID,
			Enabled:               binding.Enabled,
			TriggerStatuses:       binding.TriggerStatuses,
			Conditions:            binding.Conditions,
			LastTriggeredAt:       binding.LastTriggeredAt,
			LastTriggerReason:     binding.LastTriggerReason,
		}
		if binding.Edges.UsageMonitorChannel != nil {
			view.UsageMonitorName = binding.Edges.UsageMonitorChannel.Name
		}
		views = append(views, view)
	}
	return views, nil
}

func cleanQuotaStatuses(statuses []string) []string {
	allowed := map[string]struct{}{"available": {}, "warning": {}, "exhausted": {}, "unknown": {}}
	seen := map[string]struct{}{}
	out := []string{}
	for _, status := range statuses {
		status = strings.TrimSpace(status)
		if _, ok := allowed[status]; !ok {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		out = append(out, status)
	}
	return out
}

func cleanQuotaConditions(conditions []objects.QuotaMonitorBindingCondition) []objects.QuotaMonitorBindingCondition {
	out := []objects.QuotaMonitorBindingCondition{}
	for _, condition := range conditions {
		condition.Field = strings.TrimSpace(condition.Field)
		condition.Value = strings.TrimSpace(condition.Value)
		if condition.Field == "" || condition.Value == "" {
			continue
		}
		switch condition.Operator {
		case objects.QuotaMonitorOperatorLT, objects.QuotaMonitorOperatorLTE, objects.QuotaMonitorOperatorEQ, objects.QuotaMonitorOperatorNEQ, objects.QuotaMonitorOperatorGTE, objects.QuotaMonitorOperatorGT, objects.QuotaMonitorOperatorContains, objects.QuotaMonitorOperatorNotContains:
			out = append(out, condition)
		}
	}
	return out
}

func monitorQuotaStatusString(monitor *ent.UsageMonitorChannel) string {
	if monitor.QuotaStatus == "" {
		return string(usagemonitorchannel.QuotaStatusUnknown)
	}
	return string(monitor.QuotaStatus)
}
```

- [ ] **Step 4: Replace aggregation in `usage_monitor_internal.go`**

Modify `evaluateAndUpdateChannelQuotaReady` in `internal/server/biz/usage_monitor_internal.go` so it queries `ChannelUsageMonitorBinding` instead of `UsageMonitorChannel.AutoDisableEnabled`. The core of the function should be:

```go
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(
			channelusagemonitorbinding.ChannelID(channelID),
			channelusagemonitorbinding.Enabled(true),
			channelusagemonitorbinding.DeletedAtEQ(0),
		).
		WithUsageMonitorChannel().
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query quota monitor bindings for channel %d: %w", channelID, err)
	}

	if len(bindings) == 0 {
		return svc.updateChannelQuotaBindingReady(ctx, channelID, true, "")
	}

	ch, err := client.Channel.Query().Where(channel.ID(channelID)).Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to get channel %d: %w", channelID, err)
	}

	strategy := string(channel.QuotaMultiMonitorStrategyAny)
	if ch.QuotaMultiMonitorStrategy != nil && *ch.QuotaMultiMonitorStrategy != "" {
		strategy = string(*ch.QuotaMultiMonitorStrategy)
	}

	results := make([]quotaMonitorBindingRuleResult, 0, len(bindings))
	for _, binding := range bindings {
		monitor := binding.Edges.UsageMonitorChannel
		if monitor == nil || monitor.DeletedAt != 0 || monitor.Status != usagemonitorchannel.StatusActive {
			results = append(results, quotaMonitorBindingRuleResult{Effective: false})
			continue
		}
		results = append(results, evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
			MonitorName:     monitor.Name,
			QuotaStatus:     monitorQuotaStatusString(monitor),
			TriggerStatuses: binding.TriggerStatuses,
			Conditions:      binding.Conditions,
			ParsedFields:    parsedFieldsMapFromMonitor(monitor),
			LastPollData:    monitor.LastPollData,
			QuotaLimits:     monitor.QuotaLimits,
		}))
	}

	ready, errorMsg := aggregateQuotaMonitorBindingResults(strategy, results)
	return svc.updateChannelQuotaBindingReady(ctx, channelID, ready, errorMsg)
```

Add required imports:

```go
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
```

Keep the existing `updateChannelQuotaBindingReady` function and its error-message ownership behavior.

- [ ] **Step 5: Add parsed field extraction helper**

In `internal/server/biz/channel_quota_monitor_binding.go`, append:

```go
func parsedFieldsMapFromMonitor(monitor *ent.UsageMonitorChannel) map[string]any {
	fields := map[string]any{}
	if monitor == nil {
		return fields
	}
	parsed, err := parseMonitorFieldsForBinding(monitor)
	if err == nil {
		for _, item := range parsed {
			fields[item.Key] = item.Value
			if item.Percent != nil {
				fields[item.Key+".percent"] = *item.Percent
			}
			if item.Total != nil {
				fields[item.Key+".total"] = *item.Total
			}
		}
	}
	return fields
}

func parseMonitorFieldsForBinding(monitor *ent.UsageMonitorChannel) ([]usage_monitor.ParsedFieldValue, error) {
	if monitor.LastPollData == nil {
		return nil, nil
	}
	variables := convertMapSliceToVariables(monitor.Variables)
	displayFields := convertMapSliceToDisplayFields(monitor.DisplayFields)
	enrichDisplayFieldsFromTemplate(monitor, displayFields)
	return usage_monitor.ParseDisplayFields(monitor.LastPollData, variables, displayFields)
}
```

Add import to `channel_quota_monitor_binding.go`:

```go
	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
```

If `usage_monitor.ParseDisplayFields` has a different exported name, use the existing parser function that backs the current `parsedData` resolver and keep the helper output map behavior identical.

- [ ] **Step 6: Re-evaluate affected channels after monitor changes**

In `internal/server/biz/usage_monitor.go`, find places that save or update a `UsageMonitorChannel` after refresh/update/delete/status changes. After a monitor is saved, call this helper:

```go
func (svc *UsageMonitorService) evaluateChannelsForMonitor(ctx context.Context, monitorID int) {
	client := svc.entFromContext(ctx)
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.UsageMonitorChannelID(monitorID)).
		All(ctx)
	if err != nil {
		log.Warn(ctx, "failed to query quota bindings for monitor", log.Int("monitor_id", monitorID), log.Cause(err))
		return
	}
	for _, binding := range bindings {
		if err := svc.evaluateAndUpdateChannelQuotaReady(ctx, binding.ChannelID); err != nil {
			log.Warn(ctx, "failed to evaluate quota binding for channel", log.Int("channel_id", binding.ChannelID), log.Cause(err))
		}
	}
}
```

Add the import for `channelusagemonitorbinding` in `usage_monitor.go`. Call `svc.evaluateChannelsForMonitor(ctx, saved.ID)` after refresh/update paths and before returning the updated monitor.

- [ ] **Step 7: Run service tests**

Run:

```powershell
go test ./internal/server/biz -run "QuotaBinding|SaveChannelQuotaMonitorBindings|EvaluateAndUpdateChannelQuotaReady" -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit backend service integration**

Run:

```powershell
git add internal/server/biz/channel_quota_monitor_binding.go internal/server/biz/channel_quota_monitor_binding_test.go internal/server/biz/usage_monitor_internal.go internal/server/biz/usage_monitor.go
git commit -m "feat(quota): save and evaluate monitor bindings"
```

Expected: commit succeeds.

---

## Task 5: Add GraphQL API for Binding Config and Summaries

**Files:**
- Modify: `internal/server/gql/usage_monitor.graphql`
- Modify: `internal/server/gql/usage_monitor.resolvers.go`
- Generated by command: `internal/server/gql/generated.go`, `internal/server/gql/models_gen.go`

- [ ] **Step 1: Extend GraphQL schema**

Append this block to `internal/server/gql/usage_monitor.graphql` before the final mutation extension or at the end of the file:

```graphql
enum QuotaMonitorConditionOperator {
  LT
  LTE
  EQ
  NEQ
  GTE
  GT
  CONTAINS
  NOT_CONTAINS
}

type QuotaMonitorBindingCondition {
  field: String!
  operator: String!
  value: String!
}

input QuotaMonitorBindingConditionInput {
  field: String!
  operator: String!
  value: String!
}

type ChannelQuotaMonitorBinding {
  id: ID!
  channelID: ID!
  usageMonitorChannelID: ID!
  usageMonitorName: String!
  enabled: Boolean!
  triggerStatuses: [String!]!
  conditions: [QuotaMonitorBindingCondition!]!
  lastTriggeredAt: Time
  lastTriggerReason: String
}

input SaveChannelQuotaMonitorBindingInput {
  usageMonitorChannelID: ID!
  enabled: Boolean!
  triggerStatuses: [String!]!
  conditions: [QuotaMonitorBindingConditionInput!]!
}

input SaveChannelQuotaMonitorBindingsInput {
  strategy: String!
  bindings: [SaveChannelQuotaMonitorBindingInput!]!
}

type UsageMonitorBindingSummary {
  channelID: ID!
  channelName: String!
  usageMonitorChannelID: ID!
  strategy: String!
  enabled: Boolean!
  triggerStatuses: [String!]!
  conditions: [QuotaMonitorBindingCondition!]!
  matched: Boolean!
  reason: String
}

extend type Query {
  channelQuotaMonitorBindings(channelID: ID!): [ChannelQuotaMonitorBinding!]!
  usageMonitorBindingSummaries: [UsageMonitorBindingSummary!]!
}

extend type Mutation {
  saveChannelQuotaMonitorBindings(channelID: ID!, input: SaveChannelQuotaMonitorBindingsInput!): [ChannelQuotaMonitorBinding!]!
}
```

- [ ] **Step 2: Generate gqlgen output**

Run:

```powershell
go generate ./internal/server/gql
```

Expected: generated resolver stubs appear in `internal/server/gql/usage_monitor.resolvers.go` or compiler points to missing methods.

- [ ] **Step 3: Implement resolver conversions**

In `internal/server/gql/usage_monitor.resolvers.go`, add helper conversions near existing usage monitor input resolvers:

```go
func graphQLQuotaConditionToObject(in *QuotaMonitorBindingConditionInput) objects.QuotaMonitorBindingCondition {
	if in == nil {
		return objects.QuotaMonitorBindingCondition{}
	}
	return objects.QuotaMonitorBindingCondition{
		Field:    in.Field,
		Operator: objects.QuotaMonitorConditionOperator(in.Operator),
		Value:    in.Value,
	}
}

func objectQuotaConditionToGraphQL(in objects.QuotaMonitorBindingCondition) *QuotaMonitorBindingCondition {
	return &QuotaMonitorBindingCondition{
		Field:    in.Field,
		Operator: string(in.Operator),
		Value:    in.Value,
	}
}

func channelQuotaBindingViewToGraphQL(view biz.ChannelQuotaMonitorBindingView) *ChannelQuotaMonitorBinding {
	conditions := make([]*QuotaMonitorBindingCondition, 0, len(view.Conditions))
	for _, condition := range view.Conditions {
		conditions = append(conditions, objectQuotaConditionToGraphQL(condition))
	}
	return &ChannelQuotaMonitorBinding{
		ID:                    objects.NewGUID(objects.ChannelUsageMonitorBindingObject, view.ID),
		ChannelID:             objects.NewGUID(objects.ChannelObject, view.ChannelID),
		UsageMonitorChannelID: objects.NewGUID(objects.UsageMonitorChannelObject, view.UsageMonitorChannelID),
		UsageMonitorName:      view.UsageMonitorName,
		Enabled:               view.Enabled,
		TriggerStatuses:       view.TriggerStatuses,
		Conditions:            conditions,
		LastTriggeredAt:       view.LastTriggeredAt,
		LastTriggerReason:     view.LastTriggerReason,
	}
}
```

If `objects.ChannelUsageMonitorBindingObject` is not defined, add a GUID object constant following the existing pattern in `internal/objects/guid.go` before compiling.

- [ ] **Step 4: Implement query and mutation resolvers**

Add these resolver bodies to `internal/server/gql/usage_monitor.resolvers.go` where gqlgen expects them:

```go
func (r *queryResolver) ChannelQuotaMonitorBindings(ctx context.Context, channelID objects.GUID) ([]*ChannelQuotaMonitorBinding, error) {
	views, err := r.usageMonitorService.ListChannelQuotaMonitorBindings(ctx, channelID.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*ChannelQuotaMonitorBinding, 0, len(views))
	for _, view := range views {
		out = append(out, channelQuotaBindingViewToGraphQL(view))
	}
	return out, nil
}

func (r *mutationResolver) SaveChannelQuotaMonitorBindings(ctx context.Context, channelID objects.GUID, input SaveChannelQuotaMonitorBindingsInput) ([]*ChannelQuotaMonitorBinding, error) {
	bindings := make([]biz.SaveChannelQuotaMonitorBindingInput, 0, len(input.Bindings))
	for _, item := range input.Bindings {
		conditions := make([]objects.QuotaMonitorBindingCondition, 0, len(item.Conditions))
		for _, condition := range item.Conditions {
			conditions = append(conditions, graphQLQuotaConditionToObject(condition))
		}
		bindings = append(bindings, biz.SaveChannelQuotaMonitorBindingInput{
			UsageMonitorChannelID: item.UsageMonitorChannelID.ID,
			Enabled:               item.Enabled,
			TriggerStatuses:       item.TriggerStatuses,
			Conditions:            conditions,
		})
	}
	if err := r.usageMonitorService.SaveChannelQuotaMonitorBindings(ctx, channelID.ID, biz.SaveChannelQuotaMonitorBindingsInput{Strategy: input.Strategy, Bindings: bindings}); err != nil {
		return nil, err
	}
	views, err := r.usageMonitorService.ListChannelQuotaMonitorBindings(ctx, channelID.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*ChannelQuotaMonitorBinding, 0, len(views))
	for _, view := range views {
		out = append(out, channelQuotaBindingViewToGraphQL(view))
	}
	return out, nil
}
```

- [ ] **Step 5: Implement usage monitor summaries**

Add `ListUsageMonitorBindingSummaries` in `internal/server/biz/channel_quota_monitor_binding.go`:

```go
func (svc *UsageMonitorService) ListUsageMonitorBindingSummaries(ctx context.Context) ([]UsageMonitorBindingSummary, error) {
	client := svc.entFromContext(ctx)
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.DeletedAtEQ(0)).
		WithChannel().
		WithUsageMonitorChannel().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list usage monitor binding summaries: %w", err)
	}

	summaries := make([]UsageMonitorBindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		ch := binding.Edges.Channel
		monitor := binding.Edges.UsageMonitorChannel
		if ch == nil || monitor == nil {
			continue
		}
		strategy := string(channel.QuotaMultiMonitorStrategyAny)
		if ch.QuotaMultiMonitorStrategy != nil && *ch.QuotaMultiMonitorStrategy != "" {
			strategy = string(*ch.QuotaMultiMonitorStrategy)
		}
		result := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
			MonitorName:     monitor.Name,
			QuotaStatus:     monitorQuotaStatusString(monitor),
			TriggerStatuses: binding.TriggerStatuses,
			Conditions:      binding.Conditions,
			ParsedFields:    parsedFieldsMapFromMonitor(monitor),
			LastPollData:    monitor.LastPollData,
			QuotaLimits:     monitor.QuotaLimits,
		})
		summaries = append(summaries, UsageMonitorBindingSummary{
			ChannelID:             ch.ID,
			ChannelName:           ch.Name,
			UsageMonitorChannelID: monitor.ID,
			Strategy:              strategy,
			Enabled:               binding.Enabled,
			TriggerStatuses:       binding.TriggerStatuses,
			Conditions:            binding.Conditions,
			Matched:               result.Matched,
			Reason:                result.Reason,
		})
	}
	return summaries, nil
}
```

Add GraphQL resolver:

```go
func (r *queryResolver) UsageMonitorBindingSummaries(ctx context.Context) ([]*UsageMonitorBindingSummary, error) {
	items, err := r.usageMonitorService.ListUsageMonitorBindingSummaries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*UsageMonitorBindingSummary, 0, len(items))
	for _, item := range items {
		conditions := make([]*QuotaMonitorBindingCondition, 0, len(item.Conditions))
		for _, condition := range item.Conditions {
			conditions = append(conditions, objectQuotaConditionToGraphQL(condition))
		}
		reason := item.Reason
		out = append(out, &UsageMonitorBindingSummary{
			ChannelID:             objects.NewGUID(objects.ChannelObject, item.ChannelID),
			ChannelName:           item.ChannelName,
			UsageMonitorChannelID: objects.NewGUID(objects.UsageMonitorChannelObject, item.UsageMonitorChannelID),
			Strategy:              item.Strategy,
			Enabled:               item.Enabled,
			TriggerStatuses:       item.TriggerStatuses,
			Conditions:            conditions,
			Matched:               item.Matched,
			Reason:                &reason,
		})
	}
	return out, nil
}
```

- [ ] **Step 6: Run gql/backend tests**

Run:

```powershell
go test ./internal/server/gql ./internal/server/biz -run "QuotaBinding|UsageMonitorBinding|ChannelQuotaMonitor" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit GraphQL API**

Run:

```powershell
git add internal/server/gql/usage_monitor.graphql internal/server/gql internal/server/biz/channel_quota_monitor_binding.go internal/objects
git commit -m "feat(quota): expose monitor binding api"
```

Expected: commit succeeds.

---

## Task 6: Add Frontend Data Types and Hooks

**Files:**
- Modify: `frontend/src/features/channels/data/schema.ts`
- Modify: `frontend/src/features/channels/data/channels.ts`
- Modify: `frontend/src/features/usage-monitor/data/schema.ts`
- Modify: `frontend/src/features/usage-monitor/data/usage-monitor.ts`

- [ ] **Step 1: Add channel-side Zod schemas**

In `frontend/src/features/channels/data/schema.ts`, after `retryableErrorPatternSchema`, add:

```ts
export const quotaMonitorConditionOperatorSchema = z.enum(['<', '<=', '=', '!=', '>=', '>', 'contains', 'not_contains']);
export type QuotaMonitorConditionOperator = z.infer<typeof quotaMonitorConditionOperatorSchema>;

export const quotaMonitorBindingConditionSchema = z.object({
  field: z.string().min(1),
  operator: quotaMonitorConditionOperatorSchema,
  value: z.string().min(1),
});
export type QuotaMonitorBindingCondition = z.infer<typeof quotaMonitorBindingConditionSchema>;

export const channelQuotaMonitorBindingSchema = z.object({
  id: z.string(),
  channelID: z.string(),
  usageMonitorChannelID: z.string(),
  usageMonitorName: z.string(),
  enabled: z.boolean(),
  triggerStatuses: z.array(z.enum(['available', 'warning', 'exhausted', 'unknown'])),
  conditions: z.array(quotaMonitorBindingConditionSchema),
  lastTriggeredAt: z.string().optional().nullable(),
  lastTriggerReason: z.string().optional().nullable(),
});
export type ChannelQuotaMonitorBinding = z.infer<typeof channelQuotaMonitorBindingSchema>;

export const saveChannelQuotaMonitorBindingInputSchema = z.object({
  usageMonitorChannelID: z.string().min(1),
  enabled: z.boolean(),
  triggerStatuses: z.array(z.enum(['available', 'warning', 'exhausted', 'unknown'])),
  conditions: z.array(quotaMonitorBindingConditionSchema),
});
export type SaveChannelQuotaMonitorBindingInput = z.infer<typeof saveChannelQuotaMonitorBindingInputSchema>;
```

- [ ] **Step 2: Include quota readiness fields in channel schema and queries**

In `channelSchema`, add:

```ts
  quotaBindingReady: z.boolean().optional().default(true),
  quotaMultiMonitorStrategy: z.enum(['any', 'all']).optional().nullable(),
```

In all channel GraphQL selections that feed `channelSchema` for edit/list echo, include:

```graphql
      quotaBindingReady
      quotaMultiMonitorStrategy
```

Specifically update `CREATE_CHANNEL_MUTATION`, `DUPLICATE_CHANNEL_MUTATION`, `UPDATE_CHANNEL_MUTATION`, and `QUERY_CHANNELS_QUERY` in `frontend/src/features/channels/data/channels.ts`.

- [ ] **Step 3: Add channel binding queries and hooks**

In `frontend/src/features/channels/data/channels.ts`, import the new types/schemas and add:

```ts
const CHANNEL_QUOTA_MONITOR_BINDINGS_QUERY = `
  query ChannelQuotaMonitorBindings($channelID: ID!) {
    channelQuotaMonitorBindings(channelID: $channelID) {
      id
      channelID
      usageMonitorChannelID
      usageMonitorName
      enabled
      triggerStatuses
      conditions {
        field
        operator
        value
      }
      lastTriggeredAt
      lastTriggerReason
    }
  }
`;

const SAVE_CHANNEL_QUOTA_MONITOR_BINDINGS_MUTATION = `
  mutation SaveChannelQuotaMonitorBindings($channelID: ID!, $input: SaveChannelQuotaMonitorBindingsInput!) {
    saveChannelQuotaMonitorBindings(channelID: $channelID, input: $input) {
      id
      channelID
      usageMonitorChannelID
      usageMonitorName
      enabled
      triggerStatuses
      conditions {
        field
        operator
        value
      }
      lastTriggeredAt
      lastTriggerReason
    }
  }
`;

const channelQuotaMonitorBindingsSchema = z.array(channelQuotaMonitorBindingSchema);

export function useChannelQuotaMonitorBindings(channelID: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ['channelQuotaMonitorBindings', channelID],
    enabled: !!channelID && (options?.enabled ?? true),
    queryFn: async () => {
      const data = await graphqlRequest<{ channelQuotaMonitorBindings: unknown }>(CHANNEL_QUOTA_MONITOR_BINDINGS_QUERY, { channelID });
      return channelQuotaMonitorBindingsSchema.parse(data.channelQuotaMonitorBindings);
    },
  });
}

export function useSaveChannelQuotaMonitorBindings() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const handleError = useErrorHandler();

  return useMutation({
    mutationFn: async ({
      channelID,
      input,
    }: {
      channelID: string;
      input: { strategy: 'any' | 'all'; bindings: SaveChannelQuotaMonitorBindingInput[] };
    }) => {
      const data = await graphqlRequest<{ saveChannelQuotaMonitorBindings: unknown }>(SAVE_CHANNEL_QUOTA_MONITOR_BINDINGS_MUTATION, {
        channelID,
        input,
      });
      return channelQuotaMonitorBindingsSchema.parse(data.saveChannelQuotaMonitorBindings);
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channelQuotaMonitorBindings', variables.channelID] });
      queryClient.invalidateQueries({ queryKey: ['channels'] });
      queryClient.invalidateQueries({ queryKey: ['usageMonitorBindingSummaries'] });
      toast.success(t('channels.quotaMonitorBinding.messages.saved'));
    },
    onError: (error) => handleError(error, t('common.errors.internalServerError')),
  });
}
```

Use the existing channels query key if this file uses a constant instead of `['channels']`; keep invalidation aligned with existing hooks.

- [ ] **Step 4: Add usage-monitor summary schemas**

In `frontend/src/features/usage-monitor/data/schema.ts`, append:

```ts
export const usageMonitorBindingSummarySchema = z.object({
  channelID: z.string(),
  channelName: z.string(),
  usageMonitorChannelID: z.string(),
  strategy: z.enum(['any', 'all']),
  enabled: z.boolean(),
  triggerStatuses: z.array(z.enum(['available', 'warning', 'exhausted', 'unknown'])),
  conditions: z.array(
    z.object({
      field: z.string(),
      operator: z.string(),
      value: z.string(),
    })
  ),
  matched: z.boolean(),
  reason: z.string().optional().nullable(),
});
export type UsageMonitorBindingSummary = z.infer<typeof usageMonitorBindingSummarySchema>;
```

- [ ] **Step 5: Add usage-monitor summary hook**

In `frontend/src/features/usage-monitor/data/usage-monitor.ts`, add imports and hook:

```ts
const USAGE_MONITOR_BINDING_SUMMARIES_QUERY = `
  query UsageMonitorBindingSummaries {
    usageMonitorBindingSummaries {
      channelID
      channelName
      usageMonitorChannelID
      strategy
      enabled
      triggerStatuses
      conditions {
        field
        operator
        value
      }
      matched
      reason
    }
  }
`;

export function useUsageMonitorBindingSummaries() {
  return useQuery({
    queryKey: ['usageMonitorBindingSummaries'],
    queryFn: async () => {
      const data = await graphqlRequest<{ usageMonitorBindingSummaries: unknown }>(USAGE_MONITOR_BINDING_SUMMARIES_QUERY);
      return z.array(usageMonitorBindingSummarySchema).parse(data.usageMonitorBindingSummaries);
    },
  });
}
```

- [ ] **Step 6: Run TypeScript type check through editor diagnostics**

Do not run frontend build/lint unless the user explicitly asks. Use TypeScript diagnostics in the IDE if available; otherwise rely on the browser/dev server compilation in later browser verification.

- [ ] **Step 7: Commit frontend data layer**

Run:

```powershell
git add frontend/src/features/channels/data/schema.ts frontend/src/features/channels/data/channels.ts frontend/src/features/usage-monitor/data/schema.ts frontend/src/features/usage-monitor/data/usage-monitor.ts
git commit -m "feat(quota): add frontend binding data hooks"
```

Expected: commit succeeds.

---

## Task 7: Add Channel Binding Editor UI

**Files:**
- Create: `frontend/src/features/channels/components/channel-quota-monitor-binding.tsx`
- Modify: `frontend/src/features/channels/components/channels-action-dialog.tsx`
- Modify: locale files under `frontend/src/locales/en/channels.json` and `frontend/src/locales/zh-CN/channels.json`

- [ ] **Step 1: Add i18n keys**

Add these keys under a new `quotaMonitorBinding` object in both `frontend/src/locales/en/channels.json` and `frontend/src/locales/zh-CN/channels.json`.

English values:

```json
{
  "quotaMonitorBinding": {
    "title": "Quota monitor binding",
    "description": "Temporarily exclude this channel from routing when bound quota monitors match exhaustion rules.",
    "enabled": "Enable quota monitor binding",
    "strategy": "Multiple monitor strategy",
    "strategyAny": "Disable when any monitor matches",
    "strategyAll": "Disable only when all monitors match",
    "addBinding": "Add monitor binding",
    "monitor": "Quota monitor",
    "triggerStatuses": "Trigger statuses",
    "fieldConditions": "Field conditions",
    "addCondition": "Add condition",
    "field": "Field",
    "operator": "Operator",
    "value": "Value",
    "remove": "Remove",
    "empty": "No quota monitor bindings configured.",
    "summaryStatus": "Status: {{statuses}}",
    "summaryCondition": "{{field}} {{operator}} {{value}}",
    "messages": {
      "saved": "Quota monitor binding saved"
    }
  }
}
```

Chinese values:

```json
{
  "quotaMonitorBinding": {
    "title": "配额监控绑定",
    "description": "当绑定的配额监控命中耗尽规则时，暂时将此渠道排除出路由。",
    "enabled": "启用配额监控绑定",
    "strategy": "多个监控的聚合策略",
    "strategyAny": "任一监控命中即暂时禁用",
    "strategyAll": "全部监控命中才暂时禁用",
    "addBinding": "添加监控绑定",
    "monitor": "配额监控",
    "triggerStatuses": "触发状态",
    "fieldConditions": "字段条件",
    "addCondition": "添加条件",
    "field": "字段",
    "operator": "操作符",
    "value": "值",
    "remove": "移除",
    "empty": "尚未配置配额监控绑定。",
    "summaryStatus": "状态：{{statuses}}",
    "summaryCondition": "{{field}} {{operator}} {{value}}",
    "messages": {
      "saved": "配额监控绑定已保存"
    }
  }
}
```

Preserve the existing JSON structure; merge these keys into the current object rather than replacing the file.

- [ ] **Step 2: Create editor component**

Create `frontend/src/features/channels/components/channel-quota-monitor-binding.tsx` with this content:

```tsx
import { Plus, Trash2 } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { useUsageMonitorChannels } from '@/features/usage-monitor/data/usage-monitor';
import type { SaveChannelQuotaMonitorBindingInput, QuotaMonitorBindingCondition } from '../data/schema';

const QUOTA_STATUSES = ['available', 'warning', 'exhausted', 'unknown'] as const;
const OPERATORS = ['<', '<=', '=', '!=', '>=', '>', 'contains', 'not_contains'] as const;

type Strategy = 'any' | 'all';

interface Props {
  enabled: boolean;
  strategy: Strategy;
  bindings: SaveChannelQuotaMonitorBindingInput[];
  onEnabledChange: (enabled: boolean) => void;
  onStrategyChange: (strategy: Strategy) => void;
  onBindingsChange: (bindings: SaveChannelQuotaMonitorBindingInput[]) => void;
}

export function ChannelQuotaMonitorBinding({
  enabled,
  strategy,
  bindings,
  onEnabledChange,
  onStrategyChange,
  onBindingsChange,
}: Props) {
  const { t } = useTranslation();
  const { data: monitors = [] } = useUsageMonitorChannels();

  const monitorOptions = useMemo(
    () => monitors.map((monitor) => ({ value: monitor.id, label: monitor.name, fields: monitor.displayFields.map((field) => field.key) })),
    [monitors]
  );

  function updateBinding(index: number, next: SaveChannelQuotaMonitorBindingInput) {
    onBindingsChange(bindings.map((binding, i) => (i === index ? next : binding)));
  }

  function addBinding() {
    const firstMonitor = monitorOptions[0]?.value ?? '';
    onBindingsChange([
      ...bindings,
      { usageMonitorChannelID: firstMonitor, enabled: true, triggerStatuses: ['exhausted'], conditions: [] },
    ]);
  }

  function removeBinding(index: number) {
    onBindingsChange(bindings.filter((_, i) => i !== index));
  }

  function addCondition(binding: SaveChannelQuotaMonitorBindingInput) {
    return { ...binding, conditions: [...binding.conditions, { field: 'remaining', operator: '<=', value: '0' }] };
  }

  function updateCondition(binding: SaveChannelQuotaMonitorBindingInput, conditionIndex: number, condition: QuotaMonitorBindingCondition) {
    return { ...binding, conditions: binding.conditions.map((item, i) => (i === conditionIndex ? condition : item)) };
  }

  function removeCondition(binding: SaveChannelQuotaMonitorBindingInput, conditionIndex: number) {
    return { ...binding, conditions: binding.conditions.filter((_, i) => i !== conditionIndex) };
  }

  return (
    <Card>
      <CardHeader className='space-y-2'>
        <div className='flex items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>{t('channels.quotaMonitorBinding.title')}</CardTitle>
            <CardDescription>{t('channels.quotaMonitorBinding.description')}</CardDescription>
          </div>
          <Switch checked={enabled} onCheckedChange={onEnabledChange} />
        </div>
      </CardHeader>
      {enabled && (
        <CardContent className='space-y-4'>
          <div className='space-y-1.5'>
            <Label>{t('channels.quotaMonitorBinding.strategy')}</Label>
            <Select value={strategy} onValueChange={(value) => onStrategyChange(value as Strategy)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='any'>{t('channels.quotaMonitorBinding.strategyAny')}</SelectItem>
                <SelectItem value='all'>{t('channels.quotaMonitorBinding.strategyAll')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {bindings.length === 0 && <p className='text-sm text-muted-foreground'>{t('channels.quotaMonitorBinding.empty')}</p>}

          <div className='space-y-3'>
            {bindings.map((binding, index) => {
              const monitor = monitorOptions.find((option) => option.value === binding.usageMonitorChannelID);
              const fields = monitor?.fields ?? [];
              return (
                <div key={`${binding.usageMonitorChannelID}-${index}`} className='space-y-3 rounded-md border p-3'>
                  <div className='flex items-center justify-between gap-2'>
                    <div className='flex items-center gap-2'>
                      <Checkbox checked={binding.enabled} onCheckedChange={(checked) => updateBinding(index, { ...binding, enabled: !!checked })} />
                      <Label>{t('channels.quotaMonitorBinding.monitor')}</Label>
                    </div>
                    <Button type='button' variant='ghost' size='sm' onClick={() => removeBinding(index)}>
                      <Trash2 className='h-4 w-4' />
                      {t('channels.quotaMonitorBinding.remove')}
                    </Button>
                  </div>

                  <Select value={binding.usageMonitorChannelID} onValueChange={(value) => updateBinding(index, { ...binding, usageMonitorChannelID: value })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {monitorOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>

                  <div className='space-y-2'>
                    <Label>{t('channels.quotaMonitorBinding.triggerStatuses')}</Label>
                    <div className='flex flex-wrap gap-2'>
                      {QUOTA_STATUSES.map((status) => {
                        const selected = binding.triggerStatuses.includes(status);
                        return (
                          <Badge
                            key={status}
                            variant={selected ? 'default' : 'outline'}
                            className='cursor-pointer'
                            onClick={() =>
                              updateBinding(index, {
                                ...binding,
                                triggerStatuses: selected
                                  ? binding.triggerStatuses.filter((item) => item !== status)
                                  : [...binding.triggerStatuses, status],
                              })
                            }
                          >
                            {status}
                          </Badge>
                        );
                      })}
                    </div>
                  </div>

                  <div className='space-y-2'>
                    <div className='flex items-center justify-between'>
                      <Label>{t('channels.quotaMonitorBinding.fieldConditions')}</Label>
                      <Button type='button' variant='outline' size='sm' onClick={() => updateBinding(index, addCondition(binding))}>
                        <Plus className='h-4 w-4' />
                        {t('channels.quotaMonitorBinding.addCondition')}
                      </Button>
                    </div>

                    {binding.conditions.map((condition, conditionIndex) => (
                      <div key={conditionIndex} className='grid grid-cols-12 gap-2'>
                        <Input
                          className='col-span-5'
                          list={`quota-fields-${index}`}
                          value={condition.field}
                          placeholder={t('channels.quotaMonitorBinding.field')}
                          onChange={(event) => updateBinding(index, updateCondition(binding, conditionIndex, { ...condition, field: event.target.value }))}
                        />
                        <datalist id={`quota-fields-${index}`}>
                          {fields.map((field) => (
                            <option key={field} value={field} />
                          ))}
                          <option value='maxUsageRatio' />
                        </datalist>
                        <Select
                          value={condition.operator}
                          onValueChange={(value) => updateBinding(index, updateCondition(binding, conditionIndex, { ...condition, operator: value as QuotaMonitorBindingCondition['operator'] }))}
                        >
                          <SelectTrigger className='col-span-3'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {OPERATORS.map((operator) => (
                              <SelectItem key={operator} value={operator}>
                                {operator}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Input
                          className='col-span-3'
                          value={condition.value}
                          placeholder={t('channels.quotaMonitorBinding.value')}
                          onChange={(event) => updateBinding(index, updateCondition(binding, conditionIndex, { ...condition, value: event.target.value }))}
                        />
                        <Button type='button' variant='ghost' size='icon' onClick={() => updateBinding(index, removeCondition(binding, conditionIndex))}>
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>

          <Button type='button' variant='outline' onClick={addBinding} disabled={monitorOptions.length === 0}>
            <Plus className='h-4 w-4' />
            {t('channels.quotaMonitorBinding.addBinding')}
          </Button>
        </CardContent>
      )}
    </Card>
  );
}
```

- [ ] **Step 3: Integrate editor state into channel dialog**

In `frontend/src/features/channels/components/channels-action-dialog.tsx`, add imports:

```ts
import { useChannelQuotaMonitorBindings, useSaveChannelQuotaMonitorBindings } from '../data/channels';
import type { SaveChannelQuotaMonitorBindingInput } from '../data/schema';
import { ChannelQuotaMonitorBinding } from './channel-quota-monitor-binding';
```

Add state near other `useState` calls:

```ts
const [quotaBindingEnabled, setQuotaBindingEnabled] = useState(false);
const [quotaBindingStrategy, setQuotaBindingStrategy] = useState<'any' | 'all'>('any');
const [quotaBindings, setQuotaBindings] = useState<SaveChannelQuotaMonitorBindingInput[]>([]);
```

Add hooks after `currentRow` is available:

```ts
const quotaBindingQuery = useChannelQuotaMonitorBindings(currentRow?.id || '', { enabled: isEdit && !!currentRow?.id && open });
const saveQuotaBindings = useSaveChannelQuotaMonitorBindings();
```

Add effect:

```ts
useEffect(() => {
  if (!isEdit || !currentRow) {
    setQuotaBindingEnabled(false);
    setQuotaBindingStrategy('any');
    setQuotaBindings([]);
    return;
  }
  setQuotaBindingStrategy((currentRow.quotaMultiMonitorStrategy ?? 'any') as 'any' | 'all');
  const bindings = quotaBindingQuery.data ?? [];
  setQuotaBindingEnabled(bindings.some((binding) => binding.enabled));
  setQuotaBindings(
    bindings.map((binding) => ({
      usageMonitorChannelID: binding.usageMonitorChannelID,
      enabled: binding.enabled,
      triggerStatuses: binding.triggerStatuses,
      conditions: binding.conditions,
    }))
  );
}, [isEdit, currentRow, quotaBindingQuery.data]);
```

- [ ] **Step 4: Save bindings during channel submit**

Find `onSubmit` in `channels-action-dialog.tsx`. After the channel create/update mutation resolves for edit mode, call:

```ts
if (isEdit && currentRow?.id) {
  await saveQuotaBindings.mutateAsync({
    channelID: currentRow.id,
    input: {
      strategy: quotaBindingStrategy,
      bindings: quotaBindingEnabled ? quotaBindings : quotaBindings.map((binding) => ({ ...binding, enabled: false })),
    },
  });
}
```

Keep the existing channel update payload unchanged except for `quotaMultiMonitorStrategy` if the generated `UpdateChannelInput` accepts it. If it accepts it, also include:

```ts
quotaMultiMonitorStrategy: quotaBindingStrategy,
```

If GraphQL rejects `quotaMultiMonitorStrategy` in `UpdateChannelInput`, rely on `saveChannelQuotaMonitorBindings` to update strategy and do not send it in `updateChannel`.

- [ ] **Step 5: Render editor in advanced section**

In `channels-action-dialog.tsx`, after the `ChannelAutoDisableConfig` block, add:

```tsx
{/* Quota Monitor Binding */}
{isEdit && (
  <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
    <div className='pt-2 md:col-span-2' />
    <div className='md:col-span-6'>
      <ChannelQuotaMonitorBinding
        enabled={quotaBindingEnabled}
        strategy={quotaBindingStrategy}
        bindings={quotaBindings}
        onEnabledChange={setQuotaBindingEnabled}
        onStrategyChange={setQuotaBindingStrategy}
        onBindingsChange={setQuotaBindings}
      />
    </div>
  </div>
)}
```

- [ ] **Step 6: Browser-compile through dev server**

Open the channel edit dialog in the existing frontend dev server. Expected: no red Vite overlay and the quota monitor binding card appears only in edit mode.

- [ ] **Step 7: Commit channel editor UI**

Run:

```powershell
git add frontend/src/features/channels/components/channel-quota-monitor-binding.tsx frontend/src/features/channels/components/channels-action-dialog.tsx frontend/src/locales/en/channels.json frontend/src/locales/zh-CN/channels.json
git commit -m "feat(channels): edit quota monitor bindings"
```

Expected: commit succeeds.

---

## Task 8: Add Usage Monitor Binding Summary UI

**Files:**
- Create: `frontend/src/features/usage-monitor/components/monitor-binding-summary.tsx`
- Modify: `frontend/src/features/usage-monitor/components/monitor-card.tsx`
- Modify: locale files under `frontend/src/locales/en/usage-monitor.json` and `frontend/src/locales/zh-CN/usage-monitor.json`

- [ ] **Step 1: Add i18n keys**

Add these keys under `bindingSummary` in both usage monitor locale files.

English:

```json
{
  "bindingSummary": {
    "title": "Affected channels",
    "none": "No channel bindings",
    "strategyAny": "Any monitor",
    "strategyAll": "All monitors",
    "matched": "Triggered",
    "ready": "Ready",
    "statusRule": "Statuses: {{statuses}}",
    "conditionRule": "{{field}} {{operator}} {{value}}"
  }
}
```

Chinese:

```json
{
  "bindingSummary": {
    "title": "影响渠道",
    "none": "未绑定渠道",
    "strategyAny": "任一监控",
    "strategyAll": "全部监控",
    "matched": "已触发",
    "ready": "可用",
    "statusRule": "状态：{{statuses}}",
    "conditionRule": "{{field}} {{operator}} {{value}}"
  }
}
```

- [ ] **Step 2: Create summary component**

Create `frontend/src/features/usage-monitor/components/monitor-binding-summary.tsx`:

```tsx
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { useUsageMonitorBindingSummaries } from '../data/usage-monitor';

export function MonitorBindingSummary({ monitorID }: { monitorID: string }) {
  const { t } = useTranslation();
  const { data: summaries = [] } = useUsageMonitorBindingSummaries();
  const items = useMemo(() => summaries.filter((summary) => summary.usageMonitorChannelID === monitorID), [summaries, monitorID]);

  if (items.length === 0) {
    return <div className='text-xs text-muted-foreground'>{t('usageMonitor.bindingSummary.none')}</div>;
  }

  return (
    <div className='space-y-2 rounded-md bg-muted/40 p-2'>
      <div className='text-xs font-medium'>{t('usageMonitor.bindingSummary.title')}</div>
      {items.map((item) => (
        <div key={`${item.channelID}-${item.usageMonitorChannelID}`} className='space-y-1 rounded border bg-background p-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-xs font-medium'>{item.channelName}</span>
            <Badge variant='outline' className='text-[10px]'>
              {item.strategy === 'all' ? t('usageMonitor.bindingSummary.strategyAll') : t('usageMonitor.bindingSummary.strategyAny')}
            </Badge>
            <Badge variant={item.matched ? 'destructive' : 'secondary'} className='text-[10px]'>
              {item.matched ? t('usageMonitor.bindingSummary.matched') : t('usageMonitor.bindingSummary.ready')}
            </Badge>
          </div>
          {item.triggerStatuses.length > 0 && (
            <div className='text-[11px] text-muted-foreground'>
              {t('usageMonitor.bindingSummary.statusRule', { statuses: item.triggerStatuses.join(', ') })}
            </div>
          )}
          {item.conditions.map((condition, index) => (
            <div key={`${condition.field}-${condition.operator}-${index}`} className='text-[11px] text-muted-foreground'>
              {t('usageMonitor.bindingSummary.conditionRule', condition)}
            </div>
          ))}
          {item.reason && <div className='text-[11px] text-destructive'>{item.reason}</div>}
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Render summary in monitor cards**

In `frontend/src/features/usage-monitor/components/monitor-card.tsx`, add import:

```ts
import { MonitorBindingSummary } from './monitor-binding-summary';
```

After parsed fields block, add:

```tsx
<MonitorBindingSummary monitorID={channel.id} />
```

- [ ] **Step 4: Browser verify usage monitor page**

Open `/admin/usage-monitor`. Expected: every monitor card shows either “No channel bindings” or affected channel summaries. No Vite overlay appears.

- [ ] **Step 5: Commit monitor summary UI**

Run:

```powershell
git add frontend/src/features/usage-monitor/components/monitor-binding-summary.tsx frontend/src/features/usage-monitor/components/monitor-card.tsx frontend/src/locales/en/usage-monitor.json frontend/src/locales/zh-CN/usage-monitor.json
git commit -m "feat(usage-monitor): show channel binding summaries"
```

Expected: commit succeeds.

---

## Task 9: Fix Admin Dashboard Empty Scroll

**Files:**
- Modify: `frontend/src/features/dashboard/index.tsx`
- Modify only if inspection proves parent route owns the issue: `frontend/src/routes/_authenticated/admin/index.tsx` or layout component under `frontend/src/components/layout/`

- [ ] **Step 1: Reproduce in browser**

Open `/admin` in the browser. Scroll to the bottom. Confirm current behavior has blank space or continued empty scroll.

- [ ] **Step 2: Apply the focused layout fix**

In `frontend/src/features/dashboard/index.tsx`, change the outer wrapper from:

```tsx
<div className='flex-1 space-y-6 p-8 pt-6'>
```

to:

```tsx
<div className='w-full space-y-6 p-8 pt-6'>
```

Also change the loading and error wrappers from:

```tsx
<div className='flex-1 space-y-4 p-8 pt-6'>
```

to:

```tsx
<div className='w-full space-y-4 p-8 pt-6'>
```

This removes dashboard-level flex growth that can create blank scroll area while preserving natural content height.

- [ ] **Step 3: Browser verify `/admin` and `/`**

Open `/admin`, scroll to bottom. Expected: bottom stops at real dashboard content. Open `/`, scroll to bottom. Expected: personal dashboard layout still displays all cards/charts without truncation.

- [ ] **Step 4: Commit dashboard fix**

Run:

```powershell
git add frontend/src/features/dashboard/index.tsx
git commit -m "fix(dashboard): remove empty admin scroll space"
```

Expected: commit succeeds.

---

## Task 10: Local Upgrade Verification Script and Manual Validation

**Files:**
- Create: `.agent/summary/quota-monitor-binding-upgrade-verification.md`
- No commit until verification results are filled in.

- [ ] **Step 1: Create verification summary file**

Create `.agent/summary/quota-monitor-binding-upgrade-verification.md` with:

```markdown
# Quota Monitor Binding Upgrade Verification

## Database migration

Command:

```powershell
go test ./internal/ent/migrate/datamigrate -run "V0_1_35|Migrator" -count=1
```

Result: not run yet

## Local backend checks

Command:

```powershell
go test ./internal/server/biz -run "QuotaBinding|SaveChannelQuotaMonitorBindings|EvaluateAndUpdateChannelQuotaReady|EvaluateBinding|AggregateBinding|MaxUsageRatio" -count=1
```

Result: not run yet

## Browser checks

- `/admin` no empty infinite scroll: not checked yet
- Channel edit binding save and reopen: not checked yet
- Usage monitor binding summary: not checked yet
- Trigger condition updates `quotaBindingReady`: not checked yet
```

- [ ] **Step 2: Run migration upgrade verification tests**

Run:

```powershell
go test ./internal/ent/migrate/datamigrate -run "V0_1_35|Migrator" -count=1
```

Expected: PASS. Update the summary file result line to `PASS`.

- [ ] **Step 3: Run quota binding backend tests**

Run:

```powershell
go test ./internal/server/biz -run "QuotaBinding|SaveChannelQuotaMonitorBindings|EvaluateAndUpdateChannelQuotaReady|EvaluateBinding|AggregateBinding|MaxUsageRatio" -count=1
```

Expected: PASS. Update the summary file result line to `PASS`.

- [ ] **Step 4: Rebuild backend binary after Go changes**

Project root command:

```powershell
go build -o axonhub.exe ./cmd/axonhub/
```

Expected: exits 0. Do not commit `axonhub.exe`.

- [ ] **Step 5: Restart backend server**

Stop the running `axonhub.exe` process and start the new binary using the project’s usual local server workflow. If the server is managed by `air` in the current session, let `air` pick up changes instead of manually fighting it. Record the actual method in `.agent/summary/quota-monitor-binding-upgrade-verification.md`.

- [ ] **Step 6: Browser verify channel binding**

In the browser:

1. Open channel management.
2. Edit an existing channel.
3. Enable quota monitor binding.
4. Select strategy `any`.
5. Add one monitor binding.
6. Select status `exhausted`.
7. Add field condition `maxUsageRatio >= 1`.
8. Save.
9. Reopen the channel edit dialog.

Expected: binding is still present with selected monitor, `exhausted`, and `maxUsageRatio >= 1`. Update summary file with PASS or the observed failure.

- [ ] **Step 7: Browser verify usage monitor summary**

Open `/admin/usage-monitor`.

Expected: the monitor used in Step 6 shows the affected channel and the configured trigger rules. Update summary file.

- [ ] **Step 8: Browser verify admin dashboard scroll**

Open `/admin` and `/`.

Expected: `/admin` does not scroll into blank space; `/` still displays normally. Update summary file.

- [ ] **Step 9: Commit verification summary**

Run:

```powershell
git add .agent/summary/quota-monitor-binding-upgrade-verification.md
git commit -m "test(quota): document upgrade verification"
```

Expected: commit succeeds.

---

## Task 11: Full Required Verification and Final Commit Check

**Files:**
- No planned source changes unless verification reveals a bug.

- [ ] **Step 1: Run root build**

Run:

```powershell
go build ./...
```

Expected: PASS.

- [ ] **Step 2: Run llm build**

Run:

```powershell
Push-Location llm; go build ./...; Pop-Location
```

Expected: PASS.

- [ ] **Step 3: Run root lint**

Run:

```powershell
golangci-lint run --timeout 10m --max-same-issues 50 ./...
```

Expected: PASS with `0 issues`.

- [ ] **Step 4: Run llm lint**

Run:

```powershell
Push-Location llm; golangci-lint run --timeout 10m --max-same-issues 50 ./...; Pop-Location
```

Expected: PASS with `0 issues`.

- [ ] **Step 5: Run root tests**

Run:

```powershell
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Run llm tests**

Run:

```powershell
Push-Location llm; go test ./...; Pop-Location
```

Expected: PASS.

- [ ] **Step 7: Check git status for forbidden binaries**

Run:

```powershell
git status --short
```

Expected: no `axonhub.exe`, `.exe`, or `.exe~` files are staged or untracked for commit.

- [ ] **Step 8: Final report**

Report these exact items to the user:

```markdown
Implemented:
- Admin dashboard scroll fix
- Channel quota monitor binding editor
- Usage monitor binding summary
- Binding evaluator and GraphQL API
- Migration from existing monitor channel_id relationships

Verification:
- go build ./...: PASS
- cd llm && go build ./...: PASS
- golangci-lint root: PASS
- golangci-lint llm: PASS
- go test ./...: PASS
- cd llm && go test ./...: PASS
- local upgrade verification: PASS
- browser /admin scroll: PASS
- browser channel binding save/reopen: PASS
- browser usage monitor summary: PASS

Migration:
- internal/ent/migrate/datamigrate/v0.1.35.go
```

If any item fails, replace `PASS` with the failing command and exact error summary, then fix before claiming completion.

---

## Self-Review

### Spec coverage

- Admin dashboard natural-height scroll: Task 9.
- Channel-side binding editor: Tasks 6 and 7.
- Usage monitor read-only summary: Task 8.
- Status rules and field conditions: Tasks 3, 4, 5, 6, 7.
- Any/all strategy: Tasks 3, 4, 5, 7.
- No arbitrary code execution: Task 3 stores and evaluates structured conditions only.
- `quotaBindingReady` enforcement without channel status mutation: Task 4.
- Database migration: Task 2.
- Local upgrade verification: Task 10.
- Required build/lint/test verification: Task 11.

### Placeholder scan

The plan avoids open-ended implementation markers and provides concrete code or commands for each source-changing step. Generated files are intentionally produced by commands rather than hand-written.

### Type consistency

- Backend condition type is `objects.QuotaMonitorBindingCondition`.
- Frontend condition type is `QuotaMonitorBindingCondition` with matching `field`, `operator`, `value` fields.
- Backend save input is `SaveChannelQuotaMonitorBindingsInput`; GraphQL input uses the same name.
- Aggregation strategy values are consistently `any` and `all`.
