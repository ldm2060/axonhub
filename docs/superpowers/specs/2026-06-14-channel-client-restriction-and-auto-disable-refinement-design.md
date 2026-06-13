# Channel Client Restriction and Auto-Disable Refinement Design

**Date:** 2026-06-14  
**Status:** Approved  
**Author:** AI Assistant (Claude Opus 4.8)

## Overview

This design introduces two enhancements to AxonHub's channel management system:

1. **Client Restriction**: Fine-grained control over which coding agent clients can access coding channels (Claude Code, Codex, GitHub Copilot, etc.)
2. **Auto-Disable Refinement**: Channel-level configuration for auto-disabling channels/API keys on repeated errors

Both features support global settings with channel-level overrides, providing flexibility while maintaining sensible defaults.

## Motivation

### Current Limitations

1. **No Client Verification**: Coding channels (claudecode, codex, etc.) accept requests from any client, even non-coding tools, potentially leading to unexpected behavior or quota abuse.

2. **Coarse Auto-Disable**: The auto-disable feature is global-only. Some channels may need stricter error thresholds, while others may need to disable the feature entirely.

### Goals

- Protect coding channels by restricting access to legitimate coding agent clients
- Provide three restriction levels: off, lenient (any coding client), strict (same-family only)
- Allow channels to inherit global settings or define custom configurations
- Maintain backward compatibility with existing deployments

## Architecture

### High-Level Flow

```
User Request
    ↓
[Extract User-Agent]
    ↓
[Load Balancer] ← Global Client Restriction Setting
    ↓              ↓
[Get Candidate Channels]
    ↓
[Filter by Client Restriction] ← Channel-level Override
    ↓
[Select Channel]
    ↓
[Execute Request]
    ↓
[Error Handling]
    ↓
[Auto-Disable Check] ← Global Policy + Channel Config
```

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| `ClientDetector` | Parse User-Agent and identify client type |
| `ClientRestrictionChecker` | Evaluate restriction rules and approve/reject |
| `ChannelService` | Apply filters during load balancing |
| `AutoDisableResolver` | Merge global and channel-level auto-disable configs |
| GraphQL Resolvers | Expose configuration API to frontend |
| Frontend Components | Provide configuration UI |

## Data Model

### 1. Client Restriction Enums

```go
// internal/server/biz/system.go
type ClientRestrictionLevel string

const (
    ClientRestrictionOff     ClientRestrictionLevel = "off"     // No restriction
    ClientRestrictionLenient ClientRestrictionLevel = "lenient" // Any coding agent
    ClientRestrictionStrict  ClientRestrictionLevel = "strict"  // Same-family only
)
```

### 2. Auto-Disable Configuration

```go
// internal/objects/channel.go
type AutoDisableMode string

const (
    AutoDisableModeInheritGlobal AutoDisableMode = "inherit_global"
    AutoDisableModeDisabled      AutoDisableMode = "disabled"
    AutoDisableModeCustom        AutoDisableMode = "custom"
)

type ChannelAutoDisableConfig struct {
    Mode     AutoDisableMode                `json:"mode"`
    Enabled  bool                           `json:"enabled,omitempty"`
    Statuses []biz.AutoDisableChannelStatus `json:"statuses,omitempty"`
}
```

### 3. Channel Schema Extensions

```go
// internal/ent/schema/channel.go
func (Channel) Fields() []ent.Field {
    return []ent.Field{
        // ... existing fields
        
        field.Enum("client_restriction").
            Values("off", "lenient", "strict").
            Optional().
            Nillable().
            Comment("Client access restriction level. nil = inherit global."),
        
        field.JSON("auto_disable_config", &objects.ChannelAutoDisableConfig{}).
            Optional().
            Nillable().
            Comment("Channel-level auto-disable configuration. nil = inherit global."),
    }
}
```

### 4. Global Configuration Extension

```go
// internal/server/biz/system.go
type RetryPolicy struct {
    // ... existing fields
    
    ClientRestriction ClientRestrictionLevel `json:"client_restriction"`
}
```

## Client Detection Logic

### Supported Clients

**Lenient Mode (any coding agent):**
- `claude-cli` (Claude Code)
- `codex-cli` (Codex)
- `cursor` (Cursor)
- `antigravity` (Google Antigravity)
- `opencode` (OpenCode)
- `aider` (Aider)
- `cline` (Cline)
- `continue` (Continue)
- `copilot`, `github-copilot` (GitHub Copilot)
- `windsurf` (Windsurf)
- `cody` (Sourcegraph Cody)

**Strict Mode (channel-to-client mapping):**
```go
var ChannelClientMapping = map[string][]string{
    "claudecode":             {"claude-cli"},
    "codex":                  {"codex-cli"},
    "github_copilot":         {"copilot", "github-copilot"},
    "antigravity":            {"antigravity"},
    "opencode_go":            {"opencode"},
    "opencode_go_anthropic":  {"opencode"},
    "moonshot_coding":        {"moonshot-cli"},
}
```

### Detection Algorithm

1. Extract `User-Agent` header from request
2. Convert to lowercase for case-insensitive matching
3. Check for substring matches against known client patterns
4. Return identified client or empty string if unknown

**Example:**
- `User-Agent: claude-cli/2.1.158 (external, cli)` → Detected: `claude-cli`
- `User-Agent: Mozilla/5.0 ... cursor/0.41.0` → Detected: `cursor`

### Coding Channel Identification

**Hardcoded list:**
```go
var CodingChannelTypes = map[string]bool{
    "claudecode":             true,
    "codex":                  true,
    "github_copilot":         true,
    "antigravity":            true,
    "opencode_go":            true,
    "opencode_go_anthropic":  true,
    "moonshot_coding":        true,
}
```

Non-coding channels (openai, anthropic, deepseek, etc.) are **not** subject to client restrictions.

## Load Balancer Integration

### Filtering Flow

```go
// Pseudocode
func SelectChannelForRequest(ctx, modelID, userAgent) (*Channel, error) {
    // 1. Get global client restriction setting
    retryPolicy := getRetryPolicy(ctx)
    
    // 2. Get candidate channels for the model
    candidates := getCandidateChannels(ctx, modelID)
    
    // 3. Apply client restriction filter
    candidates = filterChannelsByClientRestriction(
        ctx,
        candidates,
        userAgent,
        retryPolicy.ClientRestriction,
    )
    
    if len(candidates) == 0 {
        return nil, ErrNoChannelsAfterClientRestriction
    }
    
    // 4. Continue with other load balancing logic
    return selectBestChannel(candidates), nil
}
```

### Filter Implementation

```go
func filterChannelsByClientRestriction(
    ctx context.Context,
    candidates []*ent.Channel,
    userAgent string,
    globalRestriction ClientRestrictionLevel,
) []*ent.Channel {
    filtered := make([]*ent.Channel, 0, len(candidates))
    
    for _, channel := range candidates {
        // Determine effective restriction level
        effectiveRestriction := globalRestriction
        if channel.ClientRestriction != nil {
            effectiveRestriction = *channel.ClientRestriction
        }
        
        // Check if client is allowed
        allowed := checker.CheckClientRestriction(
            userAgent,
            channel.Type.String(),
            channel.ClientRestriction,
            globalRestriction,
        )
        
        if allowed {
            filtered = append(filtered, channel)
        } else {
            log.Debug(ctx, "Channel filtered by client restriction",
                log.Int("channel_id", channel.ID),
                log.String("user_agent", userAgent),
            )
        }
    }
    
    return filtered
}
```

### Error Handling

When all channels are filtered out:
- Return HTTP 403 Forbidden
- Error message: `"No channels available: all channels rejected due to client restrictions"`
- Error type: `client_restriction_error`

## Auto-Disable Configuration Resolution

### Resolution Logic

```go
func ResolveChannelAutoDisableConfig(
    channel *ent.Channel,
    globalPolicy *RetryPolicy,
) (enabled bool, statuses []AutoDisableChannelStatus) {
    // Channel has custom configuration
    if channel.AutoDisableConfig != nil {
        switch channel.AutoDisableConfig.Mode {
        case AutoDisableModeDisabled:
            return false, nil
        case AutoDisableModeCustom:
            return channel.AutoDisableConfig.Enabled, 
                   channel.AutoDisableConfig.Statuses
        case AutoDisableModeInheritGlobal:
            fallthrough
        default:
            // Fall through to global
        }
    }
    
    // Inherit global configuration
    return globalPolicy.AutoDisableChannel.Enabled,
           globalPolicy.AutoDisableChannel.Statuses
}
```

### Integration with Existing Auto-Disable Logic

**Modified `checkAndHandleChannelError`:**
```go
func (svc *ChannelService) checkAndHandleChannelError(
    ctx context.Context,
    perf *PerformanceRecord,
    channel *ent.Channel,
    globalPolicy *RetryPolicy,
) bool {
    // Resolve channel's auto-disable configuration
    enabled, statuses := ResolveChannelAutoDisableConfig(channel, globalPolicy)
    
    if !enabled {
        return false // Auto-disable disabled for this channel
    }
    
    // Continue with existing error counting and disabling logic
    for _, statusConfig := range statuses {
        if statusConfig.Status != perf.ResponseStatusCode {
            continue
        }
        
        // ... existing counter logic ...
    }
}
```

**Similar changes for `checkAndHandleAPIKeyError`.**

## GraphQL API

### Schema Additions

```graphql
enum ClientRestrictionLevel {
  OFF
  LENIENT
  STRICT
}

enum AutoDisableMode {
  INHERIT_GLOBAL
  DISABLED
  CUSTOM
}

type ChannelAutoDisableConfig {
  mode: AutoDisableMode!
  enabled: Boolean
  statuses: [AutoDisableStatus!]
}

type AutoDisableStatus {
  status: Int!
  times: Int!
}

extend type RetryPolicy {
  clientRestriction: ClientRestrictionLevel!
}

extend type Channel {
  clientRestriction: ClientRestrictionLevel
  autoDisableConfig: ChannelAutoDisableConfig
}

input ChannelAutoDisableConfigInput {
  mode: AutoDisableMode!
  enabled: Boolean
  statuses: [AutoDisableStatusInput!]
}

input AutoDisableStatusInput {
  status: Int!
  times: Int!
}

extend input UpdateChannelInput {
  clientRestriction: ClientRestrictionLevel
  clearClientRestriction: Boolean
  autoDisableConfig: ChannelAutoDisableConfigInput
  clearAutoDisableConfig: Boolean
}

extend input UpdateRetryPolicyInput {
  clientRestriction: ClientRestrictionLevel
}
```

### Mutations

**Update Global Client Restriction:**
```graphql
mutation UpdateRetryPolicy($input: UpdateRetryPolicyInput!) {
  updateRetryPolicy(input: $input) {
    clientRestriction
    autoDisableChannel {
      enabled
      statuses {
        status
        times
      }
    }
  }
}
```

**Update Channel Configuration:**
```graphql
mutation UpdateChannel($id: Int!, $input: UpdateChannelInput!) {
  updateChannel(id: $id, input: $input) {
    id
    clientRestriction
    autoDisableConfig {
      mode
      enabled
      statuses {
        status
        times
      }
    }
  }
}
```

## Frontend Implementation

### System Settings - Client Restriction

**Location:** Retry Policy settings section

**UI Component:**
```tsx
<div className="space-y-2">
  <Label>Client Restriction</Label>
  <Select value={clientRestriction} onValueChange={handleChange}>
    <SelectItem value="OFF">Off</SelectItem>
    <SelectItem value="LENIENT">Lenient</SelectItem>
    <SelectItem value="STRICT">Strict</SelectItem>
  </Select>
  <p className="text-sm text-muted-foreground">
    Control which clients can access coding channels
  </p>
</div>
```

### Channel Edit - Access Control Tab

**New Tab:** "Access Control" (alongside Basic, Settings, Policies)

**Components:**
1. **Client Restriction Selector**
   - Dropdown: `Inherit Global (current: Lenient)` / `Off` / `Lenient` / `Strict`
   - Only visible for coding channels
   - Shows current global setting when inheriting

2. **Auto-Disable Configuration**
   - Mode selector: `Inherit Global` / `Disabled` / `Custom`
   - When `Custom` selected:
     - Enable/disable toggle
     - Status code rules table (status code, times, delete button)
     - "Add Rule" button

**Example Layout:**
```
┌─ Access Control ─────────────────────────────┐
│                                               │
│ Client Restriction                            │
│ [Inherit Global (current: Lenient) ▾]        │
│ ℹ️ Controls which clients can use this channel│
│                                               │
│ ─────────────────────────────────────────────│
│                                               │
│ Auto-Disable Channel/API Key                  │
│ Mode: [Custom ▾]                              │
│                                               │
│ ☑ Enable Auto-Disable                        │
│                                               │
│ Trigger Rules:                                │
│ ┌───────┬───────┬────────┐                   │
│ │ 401   │ 3     │ [🗑️]    │                   │
│ │ 402   │ 5     │ [🗑️]    │                   │
│ └───────┴───────┴────────┘                   │
│ [+ Add Rule]                                  │
│                                               │
└───────────────────────────────────────────────┘
```

### Internationalization

**English (`en/system.json`, `en/channels.json`):**
- `client_restriction`: "Client Restriction"
- `client_restriction_off`: "Off"
- `client_restriction_lenient`: "Lenient"
- `client_restriction_strict`: "Strict"
- `client_restriction_lenient_hint`: "Accepts requests from any supported coding agent client"
- `client_restriction_strict_hint`: "Only accepts requests from clients matching the channel type"
- `auto_disable_config`: "Auto-Disable Channel/API Key"
- `inherit_global`: "Inherit Global"
- `auto_disable_custom`: "Custom"

**Chinese (`zh-CN/system.json`, `zh-CN/channels.json`):**
- `client_restriction`: "客户端限制"
- `client_restriction_off`: "关闭"
- `client_restriction_lenient`: "宽松"
- `client_restriction_strict`: "严格"
- `client_restriction_lenient_hint`: "接受任意受支持的 coding agent 客户端请求"
- `client_restriction_strict_hint`: "仅接受与渠道类型匹配的同家族客户端"
- `auto_disable_config`: "自动禁用渠道/API Key"
- `inherit_global`: "继承全局"
- `auto_disable_custom`: "自定义"

## Migration and Defaults

### Database Migration

1. **Add columns to `channels` table:**
   - `client_restriction` (enum, nullable)
   - `auto_disable_config` (JSON, nullable)

2. **Existing channels:** Both fields default to `NULL` (inherit global)

3. **Schema migration generated via:**
   ```bash
   go generate ./internal/ent
   ```

### Default Values

**Global Settings (new installations):**
- `ClientRestriction`: `OFF` (no disruption to existing workflows)

**Global Settings (existing installations):**
- `ClientRestriction`: `OFF` (backward compatible)

**Channel Settings:**
- All existing channels: `NULL` for both new fields (inherit global)

### Rollout Strategy

1. Deploy backend with new fields (null by default)
2. Deploy frontend with configuration UI
3. Announce feature in release notes
4. Users can opt-in by changing global or channel-level settings

## Testing Strategy

### Unit Tests

**`client_detector_test.go`:**
- User-Agent parsing for all supported clients
- Case-insensitive matching
- Substring detection
- Empty and unknown User-Agent handling

**`client_restriction_checker_test.go`:**
- Off mode allows all
- Lenient mode allows any coding client
- Strict mode enforces family matching
- Non-coding channels always allowed
- Channel-level override takes precedence

**`channel_auto_disable_config_test.go`:**
- Inherit global mode
- Disabled mode
- Custom mode with different statuses
- Nil config defaults to global

### Integration Tests

**Load Balancer Integration:**
- Verify client restriction filtering in candidate selection
- Verify all channels filtered returns 403
- Verify non-coding channels unaffected

**Auto-Disable Integration:**
- Verify channel-level config overrides global
- Verify disabled mode prevents auto-disable
- Verify custom statuses trigger correctly

### E2E Tests

**Frontend:**
- Change global client restriction in system settings
- Configure channel-level client restriction
- Configure channel-level auto-disable (all modes)
- Verify "inherit global" shows current global value

### Manual Testing Checklist

- [ ] Create claudecode channel, set strict restriction, verify only claude-cli requests succeed
- [ ] Set global lenient, verify cursor can access claudecode channel
- [ ] Set channel-level off, verify any client can access despite global strict
- [ ] Configure custom auto-disable rules, trigger with repeated errors
- [ ] Verify disabled auto-disable mode prevents channel disabling
- [ ] Test all UI components in both English and Chinese

## Error Messages

### Client Restriction Errors

**All channels filtered:**
```json
{
  "error": {
    "message": "No channels available: all channels rejected due to client restrictions",
    "type": "client_restriction_error",
    "code": "forbidden"
  }
}
```

**HTTP Status:** 403 Forbidden

### Logs

**Debug-level logs during filtering:**
```
Channel filtered out by client restriction [channel_id=123, channel_type=claudecode, user_agent=Mozilla/5.0...]
```

## Performance Considerations

### User-Agent Parsing

- **Cost:** O(n) substring matching per client pattern (~10 patterns)
- **Impact:** Negligible (<1ms per request)
- **Caching:** Not needed due to low cost

### Channel Filtering

- **Cost:** O(n) where n = number of candidate channels (typically 1-10)
- **Impact:** Negligible, part of existing load balancer loop
- **Placement:** Integrated into existing candidate filtering pipeline

### Auto-Disable Resolution

- **Cost:** Single config object read per error event
- **Impact:** Negligible, already fetching channel in error path
- **Caching:** Channel objects already cached in memory

## Security Considerations

### User-Agent Spoofing

**Risk:** Malicious clients could spoof User-Agent headers.

**Mitigation:**
- This feature is **not** intended as a security boundary
- Purpose: prevent accidental misuse and basic quota protection
- For production security, use proper authentication (API keys, OAuth)

### False Positives

**Risk:** Legitimate requests from unknown clients could be blocked.

**Mitigation:**
- Default global setting is `OFF` (no blocking)
- Lenient mode accepts many common clients
- Channel-level override allows fine-tuning per channel
- Clear error messages help users diagnose issues

## Future Enhancements

### Potential Additions (Out of Scope)

1. **Dynamic Client Registry:** Allow admins to add custom client patterns via UI
2. **Client Signature Verification:** Cryptographic verification of client identity
3. **IP Whitelisting:** Additional layer of access control
4. **Per-User Client Restrictions:** Different restrictions for different users
5. **Analytics Dashboard:** Track client types accessing each channel

## Summary

This design introduces two complementary features:

1. **Client Restriction** provides fine-grained control over which coding agent clients can access coding channels, with three levels (off, lenient, strict) and global + channel-level configuration.

2. **Auto-Disable Refinement** allows channels to inherit, disable, or customize auto-disable rules independently of the global configuration.

Both features follow the same configuration pattern:
- Global settings provide defaults
- Channel-level settings can inherit (nil), disable, or customize
- Frontend provides clear UI for both global and channel-level configuration
- Backward compatible with existing deployments

**Implementation Complexity:** Medium  
**Risk Level:** Low (filtering happens early, defaults maintain current behavior)  
**User Value:** High (flexibility + protection for coding channels)
