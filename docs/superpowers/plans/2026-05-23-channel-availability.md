# Channel Availability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add time-based cyclic availability rules to channels so they can be excluded from routing during specified time windows.

**Architecture:** Extend `objects.ChannelPolicies` with an `Availability` field (pointer + omitempty for upgrade compat). Add an `AvailabilitySelector` decorator to the candidate selection pipeline (after `StreamPolicySelector`). New GraphQL types and inputs for the availability rules. Frontend toggle + rule editor in the channel dialog.

**Tech Stack:** Go (Ent, gqlgen), React (TypeScript, TanStack, react-hook-form, zod, Tailwind, shadcn/ui)

---

### Task 1: Add Go data model objects

**Files:**
- Modify: `internal/objects/channel.go:326-328`

- [ ] **Step 1: Add `ChannelAvailability` and `ChannelAvailabilityRule` types to `internal/objects/channel.go`**

Add the following types after the `ChannelPolicies` struct (after line 328):

```go
type ChannelAvailabilityRuleType string

const (
	ChannelAvailabilityRuleTypeAvailable   ChannelAvailabilityRuleType = "available"
	ChannelAvailabilityRuleTypeUnavailable ChannelAvailabilityRuleType = "unavailable"
)

type ChannelAvailability struct {
	Rules []ChannelAvailabilityRule `json:"rules"`
}

type ChannelAvailabilityRule struct {
	Type      ChannelAvailabilityRuleType `json:"type"`
	Days      []int                       `json:"days,omitempty"`  // 1=Mon ... 7=Sun; nil/empty = every day
	StartTime string                      `json:"startTime"`      // "HH:MM" 24-hour
	EndTime   string                      `json:"endTime"`        // "HH:MM" 24-hour
	Enabled   bool                        `json:"enabled"`
}
```

- [ ] **Step 2: Add `Availability` field to `ChannelPolicies`**

Change:

```go
type ChannelPolicies struct {
	Stream CapabilityPolicy `json:"stream,omitempty"`
}
```

To:

```go
type ChannelPolicies struct {
	Stream       CapabilityPolicy      `json:"stream,omitempty"`
	Availability *ChannelAvailability  `json:"availability,omitempty"`
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/objects/channel.go
git commit -m "feat: add ChannelAvailability data model to ChannelPolicies"
```

---

### Task 2: Add availability matching logic

**Files:**
- Create: `internal/objects/channel_availability.go`
- Create: `internal/objects/channel_availability_test.go`

- [ ] **Step 1: Write failing tests for `IsChannelAvailable` and `MatchesTimeWindow`**

Create `internal/objects/channel_availability_test.go`:

```go
package objects

import (
	"testing"
	"time"
)

func TestIsChannelAvailable_NoRules(t *testing.T) {
	ch := ChannelPolicies{}
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC) // Monday
	if !IsChannelAvailable(ch, now) {
		t.Error("expected available when no rules")
	}
}

func TestIsChannelAvailable_NilAvailability(t *testing.T) {
	ch := ChannelPolicies{Availability: nil}
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	if !IsChannelAvailable(ch, now) {
		t.Error("expected available when availability is nil")
	}
}

func TestIsChannelAvailable_EmptyRules(t *testing.T) {
	ch := ChannelPolicies{Availability: &ChannelAvailability{Rules: nil}}
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	if !IsChannelAvailable(ch, now) {
		t.Error("expected available when rules are empty")
	}
}

func TestIsChannelAvailable_AvailableRule(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeAvailable, Days: []int{1, 2, 3, 4, 5}, StartTime: "09:00", EndTime: "18:00", Enabled: true},
			},
		},
	}
	// Monday 10:00 = available
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available on Mon 10:00")
	}
	// Monday 20:00 = not available (outside window)
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 20, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable on Mon 20:00")
	}
	// Saturday 10:00 = not available (not in days)
	if IsChannelAvailable(ch, time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable on Sat 10:00")
	}
}

func TestIsChannelAvailable_UnavailableRule(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeUnavailable, Days: nil, StartTime: "22:00", EndTime: "06:00", Enabled: true},
			},
		},
	}
	// 23:00 = unavailable
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 23, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable at 23:00")
	}
	// 05:00 = unavailable
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 5, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable at 05:00")
	}
	// 10:00 = available (outside window)
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available at 10:00")
	}
}

func TestIsChannelAvailable_DisabledRuleIgnored(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeUnavailable, StartTime: "00:00", EndTime: "23:59", Enabled: false},
			},
		},
	}
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available when rule is disabled")
	}
}

func TestIsChannelAvailable_LastMatchWins(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeAvailable, StartTime: "00:00", EndTime: "23:59", Enabled: true},
				{Type: ChannelAvailabilityRuleTypeUnavailable, StartTime: "12:00", EndTime: "13:00", Enabled: true},
			},
		},
	}
	// 10:00 matches first rule → available
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available at 10:00")
	}
	// 12:30 matches both, last wins → unavailable
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 12, 30, 0, 0, time.UTC)) {
		t.Error("expected unavailable at 12:30")
	}
}

func TestMatchesTimeWindow_SameDay(t *testing.T) {
	if !MatchesTimeWindow("10:00", "09:00", "18:00") {
		t.Error("10:00 in 09:00–18:00")
	}
	if MatchesTimeWindow("08:00", "09:00", "18:00") {
		t.Error("08:00 not in 09:00–18:00")
	}
	if MatchesTimeWindow("18:00", "09:00", "18:00") {
		t.Error("18:00 not in 09:00–18:00 (exclusive end)")
	}
}

func TestMatchesTimeWindow_CrossDay(t *testing.T) {
	if !MatchesTimeWindow("23:00", "22:00", "06:00") {
		t.Error("23:00 in 22:00–06:00 cross-day")
	}
	if !MatchesTimeWindow("03:00", "22:00", "06:00") {
		t.Error("03:00 in 22:00–06:00 cross-day")
	}
	if MatchesTimeWindow("10:00", "22:00", "06:00") {
		t.Error("10:00 not in 22:00–06:00 cross-day")
	}
}

func TestISOWeekday(t *testing.T) {
	// 2026-05-25 is Monday
	if w := ISOWeekday(time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)); w != 1 {
		t.Errorf("Monday = %d, want 1", w)
	}
	// 2026-05-31 is Sunday
	if w := ISOWeekday(time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)); w != 7 {
		t.Errorf("Sunday = %d, want 7", w)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd internal/objects && go test -run "TestIsChannelAvailable|TestMatchesTimeWindow|TestISOWeekday" -v`
Expected: FAIL — `IsChannelAvailable` undefined

- [ ] **Step 3: Implement `IsChannelAvailable`, `MatchesTimeWindow`, `ISOWeekday`**

Create `internal/objects/channel_availability.go`:

```go
package objects

import "time"

// IsChannelAvailable evaluates a channel's availability rules against the given time.
// Returns true if the channel should be considered available.
// No rules (nil Availability or empty Rules) → default available.
func IsChannelAvailable(policies ChannelPolicies, now time.Time) bool {
	if policies.Availability == nil || len(policies.Availability.Rules) == 0 {
		return true
	}

	available := true
	weekday := ISOWeekday(now)
	hhmm := now.Format("15:04")

	for _, rule := range policies.Availability.Rules {
		if !rule.Enabled {
			continue
		}
		if len(rule.Days) > 0 && !containsInt(rule.Days, weekday) {
			continue
		}
		if !MatchesTimeWindow(hhmm, rule.StartTime, rule.EndTime) {
			continue
		}
		available = (rule.Type == ChannelAvailabilityRuleTypeAvailable)
	}

	return available
}

// ISOWeekday returns the ISO 8601 weekday: 1=Monday ... 7=Sunday.
func ISOWeekday(t time.Time) int {
	w := int(t.Weekday())
	if w == 0 {
		return 7
	}
	return w
}

// MatchesTimeWindow checks whether hhmm ("HH:MM") falls within the time window [start, end).
// Supports cross-day windows where start > end (e.g. "22:00"–"06:00").
func MatchesTimeWindow(hhmm, start, end string) bool {
	if start <= end {
		return hhmm >= start && hhmm < end
	}
	// Cross-day: matches >= start OR < end
	return hhmm >= start || hhmm < end
}

func containsInt(slice []int, v int) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd internal/objects && go test -run "TestIsChannelAvailable|TestMatchesTimeWindow|TestISOWeekday" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/objects/channel_availability.go internal/objects/channel_availability_test.go
git commit -m "feat: add channel availability matching logic with tests"
```

---

### Task 3: Add AvailabilitySelector decorator

**Files:**
- Create: `internal/server/orchestrator/candidates_availability.go`
- Create: `internal/server/orchestrator/candidates_availability_test.go`
- Modify: `internal/server/orchestrator/select_candidates.go:36`

- [ ] **Step 1: Write failing test for AvailabilitySelector**

Create `internal/server/orchestrator/candidates_availability_test.go`:

```go
package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm"
)

func TestAvailabilitySelector_NoRules(t *testing.T) {
	inner := &mockSelector{candidates: []*ChannelModelsCandidate{
		{Channel: &mockBizChannel{id: 1}, Priority: 0},
	}}
	s := WithAvailabilitySelector(inner)
	result, err := s.Select(context.Background(), &llm.Request{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result))
	}
}

func TestAvailabilitySelector_FiltersUnavailable(t *testing.T) {
	ch := &mockBizChannel{id: 1}
	ch.policies.Availability = &objects.ChannelAvailability{
		Rules: []objects.ChannelAvailabilityRule{
			{Type: objects.ChannelAvailabilityRuleTypeUnavailable, StartTime: "00:00", EndTime: "23:59", Enabled: true},
		},
	}
	inner := &mockSelector{candidates: []*ChannelModelsCandidate{
		{Channel: ch, Priority: 0},
	}}
	s := WithAvailabilitySelector(inner)
	result, err := s.Select(context.Background(), &llm.Request{Model: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 candidates, got %d", len(result))
	}
}

type mockBizChannel struct {
	id       int
	policies objects.ChannelPolicies
}

func (m *mockBizChannel) GetPolicies() objects.ChannelPolicies { return m.policies }

// mockSelector implements CandidateSelector for testing
type mockSelector struct {
	candidates []*ChannelModelsCandidate
	err        error
}

func (m *mockSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	return m.candidates, m.err
}
```

Note: The test needs to access `Channel.Policies` on `biz.Channel`. Since `biz.Channel` embeds `*ent.Channel`, the `Policies` field is directly accessible. For the mock, we need to ensure the interface is compatible. We'll adapt this test based on how `biz.Channel` exposes policies.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd internal/server/orchestrator && go test -run "TestAvailabilitySelector" -v`
Expected: FAIL — `WithAvailabilitySelector` undefined

- [ ] **Step 3: Implement AvailabilitySelector**

Create `internal/server/orchestrator/candidates_availability.go`:

```go
package orchestrator

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm"
)

type AvailabilitySelector struct {
	wrapped CandidateSelector
}

func WithAvailabilitySelector(wrapped CandidateSelector) *AvailabilitySelector {
	return &AvailabilitySelector{wrapped: wrapped}
}

func (s *AvailabilitySelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		return objects.IsChannelAvailable(c.Channel.Policies, now)
	}), nil
}
```

- [ ] **Step 4: Wire into selectCandidates**

In `internal/server/orchestrator/select_candidates.go`, add after line 36 (`selector = WithStreamPolicySelector(selector)`):

```go
selector = WithAvailabilitySelector(selector)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd internal/server/orchestrator && go test -run "TestAvailabilitySelector" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/orchestrator/candidates_availability.go internal/server/orchestrator/candidates_availability_test.go internal/server/orchestrator/select_candidates.go
git commit -m "feat: add AvailabilitySelector to candidate selection pipeline"
```

---

### Task 4: Add GraphQL types and inputs

**Files:**
- Modify: `internal/server/gql/axonhub.graphql:119-126`

- [ ] **Step 1: Add availability enums and types to `axonhub.graphql`**

After the existing `ChannelPolicies` type (around line 119), replace:

```graphql
type ChannelPolicies {
  stream: CapabilityPolicy
}

input ChannelPoliciesInput {
  stream: CapabilityPolicy
}
```

With:

```graphql
enum ChannelAvailabilityRuleType {
  available
  unavailable
}

type ChannelAvailabilityRule {
  type: ChannelAvailabilityRuleType!
  days: [Int!]
  startTime: String!
  endTime: String!
  enabled: Boolean!
}

type ChannelAvailability {
  rules: [ChannelAvailabilityRule!]!
}

input ChannelAvailabilityRuleInput {
  type: ChannelAvailabilityRuleType!
  days: [Int!]
  startTime: String!
  endTime: String!
  enabled: Boolean!
}

input ChannelAvailabilityInput {
  rules: [ChannelAvailabilityRuleInput!]!
}

type ChannelPolicies {
  stream: CapabilityPolicy
  availability: ChannelAvailability
}

input ChannelPoliciesInput {
  stream: CapabilityPolicy
  availability: ChannelAvailabilityInput
}
```

- [ ] **Step 2: Update gqlgen.yml model mapping**

In `internal/server/gql/gqlgen.yml`, find the `ChannelPolicies` and `ChannelPoliciesInput` entries. Add mappings for the new types:

```yaml
  ChannelAvailabilityRuleType:
    model:
      - github.com/ldm2060/axonhub/internal/objects.ChannelAvailabilityRuleType
  ChannelAvailabilityRule:
    model:
      - github.com/ldm2060/axonhub/internal/objects.ChannelAvailabilityRule
  ChannelAvailability:
    model:
      - github.com/ldm2060/axonhub/internal/objects.ChannelAvailability
  ChannelAvailabilityRuleInput:
    model:
      - github.com/ldm2060/axonhub/internal/objects.ChannelAvailabilityRule
  ChannelAvailabilityInput:
    model:
      - github.com/ldm2060/axonhub/internal/objects.ChannelAvailability
```

Note: The input types reuse the same Go types since gqlgen maps input fields to the same struct fields via JSON tags.

- [ ] **Step 3: Run code generation**

Run: `cd internal/server/gql && go generate ./...`

This regenerates `generated.go` with the new types. Any compilation errors in the resolver stubs must be fixed.

- [ ] **Step 4: Commit**

```bash
git add internal/server/gql/axonhub.graphql internal/server/gql/gqlgen.yml internal/server/gql/generated.go
git commit -m "feat: add GraphQL types for channel availability"
```

---

### Task 5: Add frontend Zod schema and form types

**Files:**
- Modify: `frontend/src/features/channels/data/schema.ts`

- [ ] **Step 1: Add availability schemas to `schema.ts`**

After `channelPoliciesSchema` (around line 107), add:

```typescript
export const channelAvailabilityRuleTypeSchema = z.enum(['available', 'unavailable']);
export type ChannelAvailabilityRuleType = z.infer<typeof channelAvailabilityRuleTypeSchema>;

export const channelAvailabilityRuleSchema = z.object({
  type: channelAvailabilityRuleTypeSchema,
  days: z.array(z.number().int().min(1).max(7)).optional().nullable(),
  startTime: z.string().regex(/^\d{2}:\d{2}$/, 'Invalid time format'),
  endTime: z.string().regex(/^\d{2}:\d{2}$/, 'Invalid time format'),
  enabled: z.boolean(),
});
export type ChannelAvailabilityRule = z.infer<typeof channelAvailabilityRuleSchema>;

export const channelAvailabilitySchema = z.object({
  rules: z.array(channelAvailabilityRuleSchema),
});
export type ChannelAvailability = z.infer<typeof channelAvailabilitySchema>;
```

Update `channelPoliciesSchema` to include availability:

```typescript
export const channelPoliciesSchema = z.object({
  stream: capabilityPolicySchema.optional(),
  availability: channelAvailabilitySchema.optional().nullable(),
});
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/features/channels/data/schema.ts
git commit -m "feat: add Zod schemas for channel availability rules"
```

---

### Task 6: Add i18n keys for availability

**Files:**
- Modify: `frontend/src/locales/en/channels.json`
- Modify: `frontend/src/locales/zh-CN/channels.json`

- [ ] **Step 1: Add English locale keys**

Add the following keys to `frontend/src/locales/en/channels.json`:

```json
"channels.dialogs.fields.availability.label": "Availability Schedule",
"channels.dialogs.fields.availability.description": "Control when this channel is available for routing requests.",
"channels.dialogs.fields.availability.enable": "Enable availability schedule",
"channels.dialogs.fields.availability.rule.type.label": "Type",
"channels.dialogs.fields.availability.rule.type.available": "Available",
"channels.dialogs.fields.availability.rule.type.unavailable": "Unavailable",
"channels.dialogs.fields.availability.rule.days.label": "Days",
"channels.dialogs.fields.availability.rule.days.all": "Every day",
"channels.dialogs.fields.availability.rule.days.mon": "Mon",
"channels.dialogs.fields.availability.rule.days.tue": "Tue",
"channels.dialogs.fields.availability.rule.days.wed": "Wed",
"channels.dialogs.fields.availability.rule.days.thu": "Thu",
"channels.dialogs.fields.availability.rule.days.fri": "Fri",
"channels.dialogs.fields.availability.rule.days.sat": "Sat",
"channels.dialogs.fields.availability.rule.days.sun": "Sun",
"channels.dialogs.fields.availability.rule.startTime.label": "Start Time",
"channels.dialogs.fields.availability.rule.endTime.label": "End Time",
"channels.dialogs.fields.availability.rule.enabled.label": "Enabled",
"channels.dialogs.fields.availability.addRule": "Add Rule",
"channels.dialogs.fields.availability.deleteRule": "Delete",
"channels.columns.timeUnavailable": "Time Unavailable"
```

- [ ] **Step 2: Add Chinese locale keys**

Add the following keys to `frontend/src/locales/zh-CN/channels.json`:

```json
"channels.dialogs.fields.availability.label": "可用性计划",
"channels.dialogs.fields.availability.description": "控制该渠道何时可参与请求路由。",
"channels.dialogs.fields.availability.enable": "启用可用性计划",
"channels.dialogs.fields.availability.rule.type.label": "类型",
"channels.dialogs.fields.availability.rule.type.available": "可用",
"channels.dialogs.fields.availability.rule.type.unavailable": "不可用",
"channels.dialogs.fields.availability.rule.days.label": "星期",
"channels.dialogs.fields.availability.rule.days.all": "每天",
"channels.dialogs.fields.availability.rule.days.mon": "周一",
"channels.dialogs.fields.availability.rule.days.tue": "周二",
"channels.dialogs.fields.availability.rule.days.wed": "周三",
"channels.dialogs.fields.availability.rule.days.thu": "周四",
"channels.dialogs.fields.availability.rule.days.fri": "周五",
"channels.dialogs.fields.availability.rule.days.sat": "周六",
"channels.dialogs.fields.availability.rule.days.sun": "周日",
"channels.dialogs.fields.availability.rule.startTime.label": "开始时间",
"channels.dialogs.fields.availability.rule.endTime.label": "结束时间",
"channels.dialogs.fields.availability.rule.enabled.label": "启用",
"channels.dialogs.fields.availability.addRule": "添加规则",
"channels.dialogs.fields.availability.deleteRule": "删除",
"channels.columns.timeUnavailable": "时段不可用"
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/en/channels.json frontend/src/locales/zh-CN/channels.json
git commit -m "feat: add i18n keys for channel availability"
```

---

### Task 7: Add availability UI to channel edit dialog

**Files:**
- Modify: `frontend/src/features/channels/components/channels-action-dialog.tsx`

- [ ] **Step 1: Add availability toggle and rule editor after stream policy field**

In `channels-action-dialog.tsx`, after the `policies.stream` form field (around line 2076), add:

1. A `FormField` for `policies.availability` toggle (when off, sets availability to null; when on, initializes with empty rules array)
2. Conditional rendering of the rules list when availability is enabled
3. Each rule card with: type select, day checkboxes, time inputs, enabled toggle, delete button
4. An "Add Rule" button at the bottom

The form default values need updating. Where `policies: { stream: 'unlimited' }` is set (lines 527, 551, 574), keep as-is since `availability` defaults to `undefined` (null), matching the "disabled" state.

For the availability form state, use a `watch('policies.availability')` to toggle visibility and `field.onChange` to set rules.

Use the existing shadcn/ui components: `Select`/`SelectDropdown` for type, `Input` for times, `Checkbox` for days, `Switch` for enabled, `Button` for add/delete.

- [ ] **Step 2: Verify the UI in browser**

Start the dev server (already running). Open the channel edit dialog, toggle availability on, add rules, verify form state updates correctly.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/channels/components/channels-action-dialog.tsx
git commit -m "feat: add availability schedule UI to channel edit dialog"
```

---

### Task 8: Add time-unavailable badge to channel table

**Files:**
- Modify: `frontend/src/features/channels/components/channels-columns.tsx`

- [ ] **Step 1: Add time-unavailable indicator to status column**

In the channel table columns, where the status badge is rendered, add a secondary badge showing "时段不可用" / "Time Unavailable" when the channel is currently in an unavailable time window. This requires computing `isChannelAvailable` client-side using the channel's policies and current time.

Add a helper function in the same file:

```typescript
function isChannelCurrentlyAvailable(policies?: ChannelPolicies | null): boolean {
  if (!policies?.availability?.rules?.length) return true;
  const now = new Date();
  const weekday = now.getDay() === 0 ? 7 : now.getDay();
  const hhmm = now.toTimeString().slice(0, 5);
  let available = true;
  for (const rule of policies.availability.rules) {
    if (!rule.enabled) continue;
    if (rule.days?.length && !rule.days.includes(weekday)) continue;
    if (!matchesTimeWindow(hhmm, rule.startTime, rule.endTime)) continue;
    available = rule.type === 'available';
  }
  return available;
}

function matchesTimeWindow(hhmm: string, start: string, end: string): boolean {
  if (start <= end) return hhmm >= start && hhmm < end;
  return hhmm >= start || hhmm < end;
}
```

Then in the status column cell render, add:

```tsx
{!isChannelCurrentlyAvailable(row.original.policies) && (
  <Badge variant="outline" className="ml-1 text-xs">
    {t('channels.columns.timeUnavailable')}
  </Badge>
)}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/features/channels/components/channels-columns.tsx
git commit -m "feat: add time-unavailable badge to channel table status column"
```

---

### Task 9: Integration test

**Files:**
- Create: `internal/server/orchestrator/candidates_availability_int_test.go` (or add to existing test file)

- [ ] **Step 1: Write integration test verifying unavailable channels are excluded**

Test that a channel with an `unavailable` rule covering the current time is excluded from the candidate list after going through the full selection pipeline (DefaultSelector → StreamPolicySelector → AvailabilitySelector).

- [ ] **Step 2: Write integration test verifying test channel bypasses availability**

Test that `SpecifiedChannelSelector` (used for channel testing) returns the channel even when it has an `unavailable` rule.

- [ ] **Step 3: Run all orchestrator tests**

Run: `cd internal/server/orchestrator && go test ./... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/server/orchestrator/candidates_availability_int_test.go
git commit -m "test: add integration tests for channel availability selection"
```

---

### Task 10: Code generation and final build verification

**Files:**
- Various generated files

- [ ] **Step 1: Run full code generation**

Run: `cd internal/server/gql && go generate ./...`

- [ ] **Step 2: Run backend build check**

Run: `cd / && go build ./cmd/axonhub`
Expected: clean build

- [ ] **Step 3: Run full test suite**

Run: `cd / && go test ./internal/... ./llm/... -count=1`
Expected: PASS

- [ ] **Step 4: Final commit if any generated files changed**

```bash
git add -A
git commit -m "chore: regenerate code after channel availability feature"
```