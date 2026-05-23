# Channel Availability (Time-based Scheduling) Design

## Summary

Add time-based availability rules to channels so administrators can control when channels participate in request routing. This addresses scenarios where providers have peak-hour traffic surges or varying pricing across time windows.

## Requirements

1. **Cyclic time windows** — recurring daily/weekly schedules (e.g. "Mon–Fri 09:00–18:00 available")
2. **Multiple rules per channel** — a channel can have several rules; later rules override earlier matches
3. **Unified timezone** — all rules evaluated in the server's timezone
4. **Full exclusion** — unavailable channels are completely excluded from load-balancing candidates, not merely deprioritized
5. **Routing + UI + Test** — availability affects routing, UI display, and test requests
6. **Default available** — no rules = always available (safe default)
7. **Upgrade compatible** — old data without `availability` field works unchanged; new data is ignored by old versions

## Design

### 1. Data Model

Add `Availability` to `objects.ChannelPolicies`:

```go
type ChannelPolicies struct {
    Stream       CapabilityPolicy      `json:"stream,omitempty"`
    Availability *ChannelAvailability  `json:"availability,omitempty"`
}

type ChannelAvailability struct {
    Rules []ChannelAvailabilityRule `json:"rules"`
}

type ChannelAvailabilityRule struct {
    Type      string   `json:"type"`              // "available" | "unavailable"
    Days      []int    `json:"days,omitempty"`     // 1=Mon ... 7=Sun; empty/nil = every day
    StartTime string   `json:"startTime"`          // "HH:MM", 24-hour format
    EndTime   string   `json:"endTime"`            // "HH:MM", 24-hour format
    Enabled   bool     `json:"enabled"`            // can be toggled off without deleting
}
```

**Upgrade compatibility:**
- `Availability` is pointer + `omitempty`: old data deserializes to nil (→ default available)
- Old versions ignore unknown JSON fields in `ChannelPolicies` (Go standard behavior)
- New data includes `availability`; old versions skip it silently

**Matching logic:**
1. No rules → default available
2. Evaluate enabled rules in order
3. `available` rule matched → channel available in that window
4. `unavailable` rule matched → channel unavailable in that window
5. Conflicting rules: last match wins (list order = priority)
6. All times in server timezone

**Cross-day windows:** If `startTime > endTime` (e.g. 22:00–06:00), the window spans midnight: matches when `hhmm >= startTime OR hhmm < endTime`.

### 2. Backend Routing Integration

Add `AvailabilitySelector` decorator in the candidate selection pipeline.

**Location:** `internal/server/orchestrator/candidates_availability.go`

```go
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
        return isChannelAvailable(c.Channel, now)
    }), nil
}
```

**Integration point:** `selectCandidates.go`, after `WithStreamPolicySelector`:

```go
selector = WithStreamPolicySelector(selector)
selector = WithAvailabilitySelector(selector)  // NEW
```

**Cache impact:** The association resolution cache (5-minute TTL) is keyed by channelCacheVersion + channel update times. Since `availability` is stored in `policies` JSON, any update to a channel's policies changes `UpdatedAt`, automatically invalidating the cache. No extra cache logic needed.

**Test channel bypass:** `SpecifiedChannelSelector` (used for channel testing) must NOT apply availability rules. Administrators need to test channels even during unavailable periods. The `AvailabilitySelector` decorator only wraps the production selection path, not the test selector.

**`isChannelAvailable` implementation:**

```go
func isChannelAvailable(ch *biz.Channel, now time.Time) bool {
    policies := ch.Policies
    if policies == nil || policies.Availability == nil || len(policies.Availability.Rules) == 0 {
        return true
    }
    available := true // default
    weekday := isoWeekday(now)       // 1=Mon ... 7=Sun
    hhmm := now.Format("15:04")
    for _, rule := range policies.Availability.Rules {
        if !rule.Enabled {
            continue
        }
        if len(rule.Days) > 0 && !lo.Contains(rule.Days, weekday) {
            continue
        }
        if !matchesTimeWindow(hhmm, rule.StartTime, rule.EndTime) {
            continue
        }
        available = (rule.Type == "available")
    }
    return available
}

func isoWeekday(t time.Time) int {
    w := int(t.Weekday())
    if w == 0 { return 7 } // Sunday = 7
    return w
}

func matchesTimeWindow(hhmm, start, end string) bool {
    if start <= end {
        return hhmm >= start && hhmm < end
    }
    // Cross-day: 22:00–06:00 → matches >= 22:00 OR < 06:00
    return hhmm >= start || hhmm < end
}
```

### 3. GraphQL API

**New types in `axonhub.graphql`:**

```graphql
enum ChannelAvailabilityRuleType {
  AVAILABLE
  UNAVAILABLE
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
```

**Extend existing types:**

```graphql
type ChannelPolicies {
  stream: CapabilityPolicy!
  availability: ChannelAvailability   # NEW, nullable
}

input ChannelPoliciesInput {
  stream: CapabilityPolicyInput!
  availability: ChannelAvailabilityInput  # NEW, nullable
}
```

**Resolvers:**
- Query: `ChannelPolicies.availability` returns directly from `objects.ChannelPolicies.Availability`; no special resolver logic
- Mutation: `policies.availability` is serialized as part of the `policies` JSON field, following the same path as `stream`. No separate save needed.

### 4. Frontend UI

**Channel edit dialog — availability section:**

Below the existing "Stream Policy" control, add an "Availability" section:

1. **Toggle switch** — enable/disable availability rules (sets `availability` to nil when off)
2. **Rule list** — each rule rendered as a card row:
   - Type dropdown: Available / Unavailable
   - Day checkboxes: Mon–Sun (unchecked all = every day)
   - Time inputs: startTime and endTime (HH:MM 24-hour picker)
   - Enabled toggle per rule
   - Delete button
3. **Add rule button** — below the list

**Channel table — availability status indicator:**

When a channel is currently unavailable due to time rules, show a badge/icon next to the status column labeling it "时段不可用" / "Time Unavailable". This helps operators quickly see which channels are currently inactive.

**Test channel behavior:**

Test requests bypass availability rules. The test button remains enabled even during unavailable periods.

**i18n:**

Add keys in `frontend/src/locales/en/channels.json` and `frontend/src/locales/zh-CN/channels.json` for all availability-related UI strings.

### 5. Testing

- Unit tests for `isChannelAvailable` and `matchesTimeWindow` (cross-day edge cases)
- Unit tests for `AvailabilitySelector.Select`
- Integration test verifying unavailable channels are excluded from routing
- Integration test verifying test requests bypass availability
- Frontend test for availability rule CRUD in the channel dialog