# Quota-Channel Binding Design

**Date**: 2026-06-15  
**Status**: Draft  
**Author**: Claude Code

## Overview

This design introduces automatic channel availability management based on quota monitoring. When a UsageMonitorChannel detects that its quota is exhausted, the associated Channel will be temporarily removed from routing selection until the quota recovers.

## Goals

1. **Automatic quota-based channel management**: Temporarily disable channels when quotas are exhausted, automatically re-enable when quotas recover
2. **Flexible threshold configuration**: Support both global defaults and per-monitor custom thresholds
3. **Multi-monitor aggregation**: Support multiple monitors on the same channel with configurable aggregation strategies
4. **Non-invasive design**: Keep Channel status field unchanged, use a separate flag for routing control
5. **Observable**: Log status changes and display quota-disabled state in UI

## Non-Goals

- Webhook notifications for quota events (future enhancement)
- Historical audit trail of quota state changes (future enhancement)
- Manual override/recovery controls (future enhancement)
- Integration with external alerting systems (future enhancement)

## Background

Currently, AxonHub has:
- **UsageMonitorChannel**: Polls provider quota APIs and derives `quota_status` (available/warning/exhausted/unknown)
- **ProviderQuotaSelector**: Filters channels based on `ProviderQuotaStatus` in orchestrator
- **Channel**: Has a `status` field (enabled/disabled/archived) controlling overall availability

The system lacks automatic binding between quota monitoring and channel availability. When a quota is exhausted, the channel continues to participate in routing, leading to failed requests.

## Requirements Summary

- **R1**: Preserve Channel status field, use separate `quota_binding_ready` flag for routing control
- **R2**: Support global default thresholds with per-monitor overrides
- **R3**: Automatically re-enable channels when quotas recover below threshold
- **R4**: Support multiple monitors per channel with "any" or "all" aggregation strategies
- **R5**: Log state changes and update Channel.error_message for visibility

## Design

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     UsageMonitor Polling                        │
│                                                                 │
│  1. Poll API                                                    │
│  2. Derive quota_status (available/warning/exhausted)           │
│  3. Calculate max_usage_ratio                                   │
│  4. Evaluate auto-disable conditions                            │
│  5. Update UsageMonitorChannel.quota_ready                      │
│  6. If builtin source → Evaluate Channel quota_binding_ready    │
│  7. Update Channel.quota_binding_ready + error_message          │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    Orchestrator (Load Balancer)                 │
│                                                                 │
│  Filter candidates:                                             │
│  - Channel.status = enabled                                     │
│  - Channel.quota_binding_ready = true  ← NEW                    │
│  - ProviderQuotaStatus check (existing)                         │
└─────────────────────────────────────────────────────────────────┘
```

### Data Model

#### 1. UsageMonitorChannel Schema Changes

Add the following fields to `internal/ent/schema/usage_monitor_channel.go`:

```go
field.Bool("auto_disable_enabled").
    Default(false).
    Comment("Enable automatic channel disabling based on quota status"),

field.Float("auto_disable_threshold").
    Default(1.0).
    Optional().
    Comment("Disable channel when max usage ratio >= this threshold (0.0-1.0). Only used when auto_disable_enabled=true"),

field.Float("auto_enable_threshold").
    Default(0.95).
    Optional().
    Comment("Re-enable channel when max usage ratio < this threshold (0.0-1.0). Only used when auto_disable_enabled=true"),

field.Enum("multi_monitor_strategy").
    Values("any", "all").
    Default("any").
    Optional().
    Comment("Strategy when multiple monitors bind to the same channel: 'any'=disable if any exhausted, 'all'=disable if all exhausted"),
```

**Field Semantics**:
- `auto_disable_enabled`: Master switch for this feature. When false, all auto-disable logic is skipped for this monitor.
- `auto_disable_threshold`: Usage ratio threshold (0.0-1.0) for triggering channel disable. When `max_usage_ratio >= threshold`, set `quota_ready=false`.
- `auto_enable_threshold`: Usage ratio threshold for re-enabling. When `max_usage_ratio < threshold`, set `quota_ready=true`. Must be <= `auto_disable_threshold`.
- `multi_monitor_strategy`: How to aggregate status when multiple monitors bind to the same channel.

#### 2. Channel Schema Changes

Add the following field to `internal/ent/schema/channel.go`:

```go
field.Bool("quota_binding_ready").
    Default(true).
    Comment("Aggregated quota-ready status from all bound UsageMonitorChannels. When false, channel is excluded from routing."),
```

**Why a separate field?**
- Decouples quota management from manual channel control (`status` field)
- Allows independent states: user can manually disable a channel, and quota auto-disable won't interfere
- Simplifies orchestrator logic: single boolean check instead of querying related monitors

#### 3. Global Configuration

Add to `conf/conf.go` and `conf/conf.yaml`:

```go
type QuotaChannelBindingConfig struct {
    DefaultDisableThreshold     float64 `yaml:"default_disable_threshold" json:"default_disable_threshold"`
    DefaultEnableThreshold      float64 `yaml:"default_enable_threshold" json:"default_enable_threshold"`
    DefaultMultiMonitorStrategy string  `yaml:"default_multi_monitor_strategy" json:"default_multi_monitor_strategy"`
}

// Add to Conf struct
QuotaChannelBinding QuotaChannelBindingConfig `yaml:"quota_channel_binding" json:"quota_channel_binding"`
```

```yaml
# conf/conf.yaml
quota_channel_binding:
  default_disable_threshold: 1.0      # 100% usage triggers disable
  default_enable_threshold: 0.95      # 95% usage allows re-enable
  default_multi_monitor_strategy: "any"  # "any" or "all"
```

### Core Logic

#### 1. UsageMonitor Polling Extension

**File**: `internal/server/biz/usage_monitor.go` → `pollUsageMonitorChannel()`

**Pseudocode**:

```go
func (s *UsageMonitorService) pollUsageMonitorChannel(ctx context.Context, monitor *ent.UsageMonitorChannel) error {
    // ... existing polling logic ...
    
    // Derive quota status (existing)
    derivedStatus := usage_monitor.DeriveQuotaStatus(monitor.ProviderType, parsedFields)
    
    // Calculate max usage ratio from limits
    maxUsageRatio := calculateMaxUsageRatio(derivedStatus.Limits)
    
    // Evaluate auto-disable conditions
    if monitor.AutoDisableEnabled {
        newQuotaReady := evaluateQuotaReady(
            monitor.QuotaReady,
            maxUsageRatio,
            getDisableThreshold(monitor, globalConfig),
            getEnableThreshold(monitor, globalConfig),
        )
        
        if newQuotaReady != monitor.QuotaReady {
            // Update monitor.quota_ready
            // Log state change
        }
    }
    
    // Update UsageMonitorChannel in database
    // ...
    
    // If source=builtin and channel_id is set, evaluate channel status
    if monitor.Source == usagemonitorchannel.SourceBuiltin && monitor.ChannelID != nil {
        s.evaluateAndUpdateChannelQuotaReady(ctx, *monitor.ChannelID)
    }
    
    return nil
}

func evaluateQuotaReady(currentReady bool, ratio float64, disableThreshold float64, enableThreshold float64) bool {
    if currentReady {
        // Currently ready → check if should disable
        if ratio >= disableThreshold {
            return false  // Disable
        }
    } else {
        // Currently not ready → check if should enable
        if ratio < enableThreshold {
            return true  // Enable
        }
    }
    return currentReady  // No change
}

func calculateMaxUsageRatio(limits []provider_quota.QuotaLimitStatus) float64 {
    maxRatio := 0.0
    for _, limit := range limits {
        if limit.UsageRatio > maxRatio {
            maxRatio = limit.UsageRatio
        }
    }
    return maxRatio
}

func getDisableThreshold(monitor *ent.UsageMonitorChannel, globalConfig *conf.QuotaChannelBindingConfig) float64 {
    if monitor.AutoDisableThreshold != nil && *monitor.AutoDisableThreshold > 0 {
        return *monitor.AutoDisableThreshold
    }
    return globalConfig.DefaultDisableThreshold
}

func getEnableThreshold(monitor *ent.UsageMonitorChannel, globalConfig *conf.QuotaChannelBindingConfig) float64 {
    if monitor.AutoEnableThreshold != nil && *monitor.AutoEnableThreshold > 0 {
        return *monitor.AutoEnableThreshold
    }
    return globalConfig.DefaultEnableThreshold
}
```

#### 2. Channel Quota Ready Evaluation

**File**: `internal/server/biz/usage_monitor_internal.go`

**Function**: `evaluateAndUpdateChannelQuotaReady(ctx, channelID)`

**Pseudocode**:

```go
func (s *UsageMonitorService) evaluateAndUpdateChannelQuotaReady(ctx context.Context, channelID int) error {
    // Query all active monitors with auto_disable_enabled=true for this channel
    monitors, err := s.entClient.UsageMonitorChannel.Query().
        Where(
            usagemonitorchannel.ChannelID(channelID),
            usagemonitorchannel.StatusEQ(usagemonitorchannel.StatusActive),
            usagemonitorchannel.AutoDisableEnabled(true),
            usagemonitorchannel.DeletedAtIsNil(),
        ).
        All(ctx)
    
    if err != nil {
        return err
    }
    
    // If no monitors with auto-disable, set ready=true
    if len(monitors) == 0 {
        return s.updateChannelQuotaBindingReady(ctx, channelID, true, "")
    }
    
    // Aggregate based on strategy
    strategy := monitors[0].MultiMonitorStrategy  // Assume all monitors use same strategy
    
    var ready bool
    var errorMsg string
    
    switch strategy {
    case "any":
        // Disable if ANY monitor is not ready
        ready = true
        for _, m := range monitors {
            if !m.QuotaReady {
                ready = false
                errorMsg = buildErrorMessage(m)
                break
            }
        }
    
    case "all":
        // Disable if ALL monitors are not ready
        allNotReady := true
        for _, m := range monitors {
            if m.QuotaReady {
                allNotReady = false
                break
            }
        }
        if allNotReady {
            ready = false
            errorMsg = buildErrorMessage(monitors[0])
        } else {
            ready = true
        }
    
    default:
        ready = true
    }
    
    return s.updateChannelQuotaBindingReady(ctx, channelID, ready, errorMsg)
}

func (s *UsageMonitorService) updateChannelQuotaBindingReady(ctx context.Context, channelID int, ready bool, errorMsg string) error {
    update := s.entClient.Channel.UpdateOneID(channelID).
        SetQuotaBindingReady(ready)
    
    if errorMsg != "" {
        update.SetErrorMessage(errorMsg)
    } else {
        update.ClearErrorMessage()
    }
    
    _, err := update.Save(ctx)
    
    if err == nil {
        // Log state change
        log.Info(ctx, "Channel quota binding status updated",
            log.Int("channel_id", channelID),
            log.Bool("ready", ready),
            log.String("error_msg", errorMsg),
        )
    }
    
    return err
}

func buildErrorMessage(monitor *ent.UsageMonitorChannel) string {
    // Extract usage ratio from last_poll_data if available
    var usageStr string
    if monitor.LastPollData != nil {
        // Parse max ratio from data (implementation details omitted)
        usageStr = "N/A"
    }
    
    return fmt.Sprintf(
        "Channel temporarily disabled due to quota exhaustion (monitor: %s, usage: %s)",
        monitor.Name,
        usageStr,
    )
}
```

#### 3. Orchestrator Integration

**File**: `internal/server/orchestrator/candidates_quota.go` → `ProviderQuotaSelector.Select()`

**Change**: Add quota_binding_ready check when filtering candidates.

**Pseudocode**:

```go
func (s *ProviderQuotaSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
    candidates, err := s.wrapped.Select(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // NEW: Filter out channels with quota_binding_ready=false
    filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
        if !c.Channel.QuotaBindingReady {
            return false  // Exclude from routing
        }
        
        // Existing quota status check
        quotaStatus := s.provider.GetQuotaStatus(c.Channel.ID)
        // ... existing logic ...
        
        return true
    })
    
    return filtered, nil
}
```

### Frontend UI

#### 1. UsageMonitor Form

**File**: `frontend/src/features/usage-monitor/components/UsageMonitorForm.tsx`

**New Section**: "Automatic Channel Management"

- Toggle switch: "Enable Automatic Channel Disabling"
- Number input: "Disable Threshold (%)" with hint showing global default
- Number input: "Re-enable Threshold (%)" with hint showing global default
- Select: "Multiple Monitors Strategy" (any/all)
- Validation: Ensure `disable_threshold >= enable_threshold`

#### 2. Channel List

**File**: `frontend/src/features/channels/components/ChannelList.tsx`

**Enhancement**: Add badge for quota-disabled channels

```tsx
{channel.status === 'enabled' && !channel.quotaBindingReady && (
  <Badge variant="warning" className="ml-2">
    <AlertTriangle className="h-3 w-3 mr-1" />
    Quota Disabled
  </Badge>
)}
```

#### 3. Channel Detail Page

**File**: `frontend/src/features/channels/components/ChannelDetail.tsx`

**New Card**: "Bound Usage Monitors"

- Table showing all bound monitors with columns:
  - Monitor Name
  - Quota Status (badge)
  - Usage Ratio (%)
  - Auto-Disable Enabled (Yes/No)
  - Threshold (%)
- Alert box showing `error_message` if channel is quota-disabled

#### 4. GraphQL Schema

**File**: `internal/server/gql/schema/*.graphql`

```graphql
extend type UsageMonitorChannel {
  autoDisableEnabled: Boolean!
  autoDisableThreshold: Float
  autoEnableThreshold: Float
  multiMonitorStrategy: String
}

extend type Channel {
  quotaBindingReady: Boolean!
}
```

### Error Handling and Edge Cases

#### Edge Case 1: Monitor Deleted/Paused

**Scenario**: A UsageMonitorChannel is deleted or set to `status=paused`.

**Solution**: When monitor status changes, trigger `evaluateAndUpdateChannelQuotaReady` for the associated channel. If all monitors are gone/paused, `quota_binding_ready` reverts to `true`.

#### Edge Case 2: All Monitors Unbound

**Scenario**: All monitors are removed from a channel.

**Solution**: `evaluateAndUpdateChannelQuotaReady` detects zero monitors and sets `quota_binding_ready=true`, clearing `error_message`.

#### Edge Case 3: Manual Channel Disable

**Scenario**: User manually sets `Channel.status=disabled` while `quota_binding_ready=false`.

**Solution**: The two flags are independent. Orchestrator checks both: `status=enabled AND quota_binding_ready=true` for routing eligibility. When quota recovers, only `quota_binding_ready` changes; `status` remains user-controlled.

#### Edge Case 4: Polling Failure (quota_status=unknown)

**Scenario**: Monitor polling fails, `quota_status` becomes `unknown`.

**Solution**: `quota_status=unknown` does NOT trigger auto-disable. Only explicit `exhausted` status (ratio >= threshold) triggers disable. Unknown status leaves `quota_ready` unchanged.

#### Edge Case 5: Invalid Threshold Configuration

**Scenario**: User sets `auto_disable_threshold < auto_enable_threshold`.

**Solution**: Frontend validates this constraint. Backend gracefully falls back to global defaults if invalid. No hard constraint at DB level to allow manual fixes.

#### Edge Case 6: Concurrent Polling

**Scenario**: Multiple monitors for the same channel poll simultaneously.

**Solution**: Each monitor updates its own `quota_ready` independently. The final `evaluateAndUpdateChannelQuotaReady` is called after each poll, idempotently aggregating the latest state. Use database transactions to prevent race conditions.

### Logging

Log at the following points:

```go
// When disabling a channel
log.Info(ctx, "Channel auto-disabled due to quota exhaustion",
    log.Int("channel_id", channelID),
    log.Int("monitor_id", monitorID),
    log.String("monitor_name", monitorName),
    log.Float64("usage_ratio", ratio),
    log.Float64("threshold", threshold),
)

// When re-enabling a channel
log.Info(ctx, "Channel auto-enabled after quota recovery",
    log.Int("channel_id", channelID),
    log.Int("monitor_id", monitorID),
    log.Float64("usage_ratio", ratio),
)

// When evaluation fails
log.Error(ctx, "Failed to evaluate channel quota status",
    log.Int("channel_id", channelID),
    log.Error(err),
)
```

### Data Migration

**Migration**: v0.1.10

**SQL**:

```sql
-- Add new fields to usage_monitor_channels
ALTER TABLE usage_monitor_channels ADD COLUMN auto_disable_enabled BOOLEAN DEFAULT false;
ALTER TABLE usage_monitor_channels ADD COLUMN auto_disable_threshold REAL DEFAULT 1.0;
ALTER TABLE usage_monitor_channels ADD COLUMN auto_enable_threshold REAL DEFAULT 0.95;
ALTER TABLE usage_monitor_channels ADD COLUMN multi_monitor_strategy TEXT DEFAULT 'any';

-- Add new field to channels
ALTER TABLE channels ADD COLUMN quota_binding_ready BOOLEAN DEFAULT true;

-- Initialize quota_binding_ready for existing channels
UPDATE channels SET quota_binding_ready = true WHERE quota_binding_ready IS NULL;
```

**Backward Compatibility**:
- All new fields have safe defaults
- `auto_disable_enabled=false` means existing monitors won't suddenly start disabling channels
- `quota_binding_ready=true` means existing channels remain fully available

### Testing Strategy

#### Unit Tests

**File**: `internal/server/biz/usage_monitor_test.go`

1. `TestAutoDisable_ThresholdExceeded`: Verify `quota_ready` becomes false when ratio exceeds threshold
2. `TestAutoEnable_UsageDrops`: Verify `quota_ready` becomes true when ratio drops below enable threshold
3. `TestMultiMonitorStrategy_Any`: Verify "any" strategy disables channel if any monitor is exhausted
4. `TestMultiMonitorStrategy_All`: Verify "all" strategy disables channel only if all monitors are exhausted
5. `TestAutoDisable_Disabled`: Verify channels are not affected when `auto_disable_enabled=false`
6. `TestAutoDisable_UseGlobalDefault`: Verify global defaults are used when monitor thresholds are nil
7. `TestMonitorStatusChange_TriggerReEvaluation`: Verify channel is re-evaluated when monitor status changes

#### Integration Tests

**File**: `internal/server/biz/usage_monitor_integration_test.go`

- End-to-end test: Create monitor with auto-disable, trigger polling with high usage, verify channel is filtered by orchestrator, then trigger polling with low usage, verify channel is included again

#### Orchestrator Tests

**File**: `internal/server/orchestrator/candidates_quota_test.go`

- Test that `ProviderQuotaSelector.Select()` correctly filters channels with `quota_binding_ready=false`

#### Frontend Tests

**File**: `frontend/src/features/usage-monitor/components/UsageMonitorForm.test.tsx`

1. Validate threshold constraints (`disable_threshold >= enable_threshold`)
2. Verify default values are populated from global config
3. Verify threshold fields are shown/hidden based on `auto_disable_enabled` toggle

## Performance Considerations

### Concern: N+1 Query Problem

**Scenario**: A channel has 100 bound monitors. Each polling triggers `evaluateAndUpdateChannelQuotaReady`, which queries all monitors.

**Mitigation**:
- Only call `evaluateAndUpdateChannelQuotaReady` when `quota_ready` **changes**, not on every poll
- Use database transactions to batch updates
- Consider caching the monitor list for a channel if polling frequency is very high

### Concern: Concurrent Updates

**Scenario**: Multiple monitors for the same channel poll simultaneously.

**Mitigation**:
- Each monitor updates its own row independently (no conflict)
- The final `evaluateAndUpdateChannelQuotaReady` is idempotent
- Use row-level locking on Channel table update to prevent race conditions

## Future Enhancements

- **Webhook notifications**: Trigger webhooks when channels are auto-disabled/enabled
- **Audit log**: Record historical state changes for analytics and debugging
- **Manual override**: Allow admins to temporarily override auto-disable (e.g., "keep enabled despite quota")
- **Smart re-enable delay**: Add a cooldown period before re-enabling to prevent flapping
- **Alerting integration**: Integrate with external alerting systems (PagerDuty, Slack, etc.)

## Implementation Phases

### Phase 1: Backend Core (Priority: High)
1. Schema changes (UsageMonitorChannel + Channel)
2. Global config structure
3. Data migration script
4. Core logic in `pollUsageMonitorChannel`
5. `evaluateAndUpdateChannelQuotaReady` function
6. Orchestrator integration
7. Unit tests

### Phase 2: Frontend UI (Priority: High)
1. GraphQL schema updates
2. UsageMonitor form enhancements
3. Channel list badge display
4. Channel detail page "Bound Monitors" card
5. Form validation
6. Frontend tests

### Phase 3: Polish and Testing (Priority: Medium)
1. Integration tests
2. E2E tests
3. Documentation updates
4. Performance profiling
5. Edge case handling verification

## Success Criteria

- Channels are automatically excluded from routing when quota is exhausted
- Channels are automatically re-included when quota recovers
- Global defaults can be overridden per-monitor
- Multiple monitors on the same channel work correctly with both "any" and "all" strategies
- All tests pass (unit, integration, E2E)
- No performance degradation in orchestrator selection
- UI clearly displays quota-disabled state

## Open Questions

None at this time. All requirements have been clarified with the user.
