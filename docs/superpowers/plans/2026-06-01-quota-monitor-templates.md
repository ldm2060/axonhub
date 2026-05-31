# Quota Monitor Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add provider template support to the Quota Monitor page so users can create monitor channels by selecting a provider and entering only an API key.

**Architecture:** Backend adds a `template` source type to UsageMonitorChannel with `provider_type` and `api_key` fields, plus a `quotaMonitorTemplates` query returning static provider templates. Frontend adds a third source option in AddChannelDialog that shows template selector + API key input only. Backend poll logic auto-assembles headers from `api_key` + `headerFormat`.

**Tech Stack:** Go (Ent ORM, gqlgen, Gin), React 19, TypeScript, TanStack Query, Zustand, Tailwind CSS

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/ent/schema/usage_monitor_channel.go` | Add `template` to source enum; add `provider_type` enum field; add `api_key` sensitive field |
| Create | `internal/server/biz/usage_monitor/templates.go` | Static template registry with all 8 provider templates |
| Modify | `internal/server/biz/usage_monitor/types.go` | Add `ProviderType`, `ApiKey` to input types |
| Modify | `internal/server/biz/usage_monitor.go` | Handle `source=template` in Create/Update; auto-assemble headers from apiKey; poll logic uses apiKey |
| Modify | `internal/server/gql/usage_monitor.graphql` | Add `QuotaMonitorTemplate` type, `quotaMonitorTemplates` query, extend channel/input types |
| Modify | `internal/server/gql/usage_monitor.resolvers.go` | Add template query resolver, apiKey field resolver, input resolvers for new fields |
| Modify | `internal/server/gql/gqlgen.yml` | Add `usagemonitorchannel` to autobind if needed |
| Modify | `frontend/src/locales/en/usage-monitor.json` | New i18n keys for template UI |
| Modify | `frontend/src/locales/zh-CN/usage-monitor.json` | New i18n keys for template UI |
| Create | `frontend/src/features/usage-monitor/data/templates.ts` | GraphQL query + React Query hook for templates |
| Modify | `frontend/src/features/usage-monitor/components/add-channel-dialog.tsx` | Add `template` source option with template selector + API key input |
| Modify | `frontend/src/features/usage-monitor/components/edit-channel-dialog.tsx` | Support editing apiKey for template channels |
| Modify | `frontend/src/features/usage-monitor/components/monitor-card.tsx` | Show provider type badge for template channels |

---

### Task 1: Add `template` source and new fields to Ent schema

**Files:**
- Modify: `internal/ent/schema/usage_monitor_channel.go`

- [ ] **Step 1: Update the schema**

Add `template` to the source enum, add `provider_type` enum field, add `api_key` sensitive field:

```go
// In Fields(), update the source enum:
field.Enum("source").
    Values("builtin", "custom", "template").
    Comment("builtin: linked to existing Channel; custom: fully manual; template: from provider template"),

// Add provider_type field after source:
field.Enum("provider_type").
    Values("claudecode", "codex", "github_copilot", "nanogpt", "wafer", "synthetic", "neuralwatt", "zhipu").
    Optional().
    Comment("Provider type for quota template (required when source=template)"),

// Add api_key field after api_body:
field.String("api_key").Optional().Sensitive().
    Comment("API key for authenticating with the provider (sensitive, hidden from GraphQL)"),
```

- [ ] **Step 2: Run code generation**

```bash
cd internal/server/gql && go generate
```

Expected: No errors. New Go constants generated in `internal/ent/usagemonitorchannel/`.

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/ent/schema/usage_monitor_channel.go internal/ent/ internal/server/gql/ent.graphql internal/server/gql/generated.go internal/server/gql/models_gen.go
git commit -m "feat(schema): add template source, provider_type and api_key to UsageMonitorChannel"
```

---

### Task 2: Create backend template registry

**Files:**
- Create: `internal/server/biz/usage_monitor/templates.go`

- [ ] **Step 1: Write the template registry**

Create `internal/server/biz/usage_monitor/templates.go` with the static template list. Each template defines the provider's quota API URL, method, header format, optional body, and field parsing configs:

```go
package usage_monitor

type QuotaMonitorTemplate struct {
	ProviderType string        `json:"providerType"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ApiURL       string        `json:"apiUrl"`
	ApiMethod    string        `json:"apiMethod"`
	HeaderFormat string        `json:"headerFormat"` // "bearer" or "x-api-key"
	ApiBody      string        `json:"apiBody,omitempty"`
	Fields       []FieldConfig `json:"fields"`
}

var quotaMonitorTemplates = []QuotaMonitorTemplate{
	{
		ProviderType: "claudecode",
		Name:         "Claude Code",
		Description:  "Monitor Claude Code rate limits and usage windows",
		ApiURL:       "https://api.anthropic.com/v1/messages",
		ApiMethod:    "POST",
		HeaderFormat: "bearer",
		ApiBody:      `{"model":"claude-haiku-4-5","messages":[{"role":"user","content":"limit"}],"max_tokens":1}`,
		Fields: []FieldConfig{
			{Key: "5h_utilization", Label: "5h Window Utilization", Path: "$.headers.anthropic-ratelimit-unified-5h-utilization", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "7d_utilization", Label: "7d Window Utilization", Path: "$.headers.anthropic-ratelimit-unified-7d-utilization", Type: "jsonpath", Format: "percentage", DisplayOrder: 1},
			{Key: "unified_status", Label: "Unified Status", Path: "$.headers.anthropic-ratelimit-unified-status", Type: "jsonpath", Format: "text", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "codex",
		Name:         "Codex / ChatGPT",
		Description:  "Monitor ChatGPT usage limits and rate windows",
		ApiURL:       "https://chatgpt.com/backend-api/wham/usage",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "primary_used_pct", Label: "Primary Window Used %", Path: "$.rate_limit.primary_window.used_percent", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "primary_reset", Label: "Primary Reset At", Path: "$.rate_limit.primary_window.reset_at", Type: "jsonpath", Format: "datetime", DisplayOrder: 1},
			{Key: "plan_type", Label: "Plan Type", Path: "$.plan_type", Type: "jsonpath", Format: "text", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "github_copilot",
		Name:         "GitHub Copilot",
		Description:  "Monitor GitHub Copilot quota limits and remaining usage",
		ApiURL:       "https://api.github.com/copilot_internal/user",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "plan", Label: "Plan", Path: "$.copilot_plan", Type: "jsonpath", Format: "text", DisplayOrder: 0},
			{Key: "access_type", Label: "Access Type", Path: "$.access_type_sku", Type: "jsonpath", Format: "text", DisplayOrder: 1},
		},
	},
	{
		ProviderType: "nanogpt",
		Name:         "NanoGPT",
		Description:  "Monitor NanoGPT subscription usage and token/image limits",
		ApiURL:       "https://nano-gpt.com/api/subscription/v1/usage",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "weekly_tokens_pct", Label: "Weekly Tokens Used %", Path: "$.weeklyInputTokens.percentUsed", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "daily_tokens_pct", Label: "Daily Tokens Used %", Path: "$.dailyInputTokens.percentUsed", Type: "jsonpath", Format: "percentage", DisplayOrder: 1},
			{Key: "daily_images_pct", Label: "Daily Images Used %", Path: "$.dailyImages.percentUsed", Type: "jsonpath", Format: "percentage", DisplayOrder: 2},
			{Key: "state", Label: "State", Path: "$.state", Type: "jsonpath", Format: "text", DisplayOrder: 3},
		},
	},
	{
		ProviderType: "wafer",
		Name:         "Wafer",
		Description:  "Monitor Wafer.ai inference quota and request limits",
		ApiURL:       "https://pass.wafer.ai/v1/inference/quota",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "used_pct", Label: "Period Used %", Path: "$.current_period_used_percent", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "remaining", Label: "Remaining Requests", Path: "$.remaining_included_requests", Type: "jsonpath", Format: "number", Unit: "requests", DisplayOrder: 1},
			{Key: "total", Label: "Total Requests", Path: "$.included_request_limit", Type: "jsonpath", Format: "number", Unit: "requests", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "synthetic",
		Name:         "Synthetic",
		Description:  "Monitor Synthetic.new weekly token and 5h rolling limits",
		ApiURL:       "https://api.synthetic.new/v2/quotas",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "weekly_remaining_pct", Label: "Weekly Tokens Remaining %", Path: "$.weeklyTokenLimit.percentRemaining", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "5h_remaining", Label: "5h Rolling Remaining", Path: "$.rollingFiveHourLimit.remaining", Type: "jsonpath", Format: "number", DisplayOrder: 1},
			{Key: "5h_max", Label: "5h Rolling Max", Path: "$.rollingFiveHourLimit.max", Type: "jsonpath", Format: "number", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "neuralwatt",
		Name:         "NeuralWatt",
		Description:  "Monitor NeuralWatt kWh subscription usage and credits",
		ApiURL:       "https://api.neuralwatt.com/v1/quota",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "kwh_used", Label: "kWh Used", Path: "$.subscription.kwh_used", Type: "jsonpath", Format: "number", Unit: "kWh", DisplayOrder: 0},
			{Key: "kwh_included", Label: "kWh Included", Path: "$.subscription.kwh_included", Type: "jsonpath", Format: "number", Unit: "kWh", DisplayOrder: 1},
			{Key: "credits_remaining", Label: "Credits Remaining", Path: "$.balance.credits_remaining_usd", Type: "jsonpath", Format: "number", Unit: "USD", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "zhipu",
		Name:         "Zhipu / Z.ai",
		Description:  "Monitor Zhipu AI quota limits for tokens and MCP usage",
		ApiURL:       "https://open.bigmodel.cn/api/monitor/usage/quota/limit",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "token_pct", Label: "Token Usage %", Path: "$.data.limits[?(@.type=='TOKENS_LIMIT')].percentage", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "time_pct", Label: "MCP Time Usage %", Path: "$.data.limits[?(@.type=='TIME_LIMIT')].percentage", Type: "jsonpath", Format: "percentage", DisplayOrder: 1},
		},
	},
}

func GetQuotaMonitorTemplates() []QuotaMonitorTemplate {
	return quotaMonitorTemplates
}

func GetQuotaMonitorTemplate(providerType string) *QuotaMonitorTemplate {
	for i := range quotaMonitorTemplates {
		if quotaMonitorTemplates[i].ProviderType == providerType {
			return &quotaMonitorTemplates[i]
		}
	}
	return nil
}
```

Note: Claude Code template uses `$.headers.xxx` JSONPath syntax — the generic checker needs a small enhancement to also capture response headers in the raw data. For now the fields parse from the response body. Claude Code is special because its quota data comes from response headers, not body. We'll note this in the resolver and handle it in Task 5.

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/usage_monitor/templates.go
git commit -m "feat(usage-monitor): add static provider template registry"
```

---

### Task 3: Update backend input types and create/update logic

**Files:**
- Modify: `internal/server/biz/usage_monitor/types.go`
- Modify: `internal/server/biz/usage_monitor.go`

- [ ] **Step 1: Add new fields to input types in `types.go`**

```go
// Add to CreateUsageMonitorChannelInput:
type CreateUsageMonitorChannelInput struct {
	Name         string        `json:"name"`
	Source       string        `json:"source"`
	ChannelID    *string       `json:"channelId,omitempty"`
	ProviderType *string       `json:"providerType,omitempty"` // required when source=template
	ApiKey       *string       `json:"apiKey,omitempty"`       // required when source=template
	ApiURL       string        `json:"apiUrl"`
	ApiMethod    string        `json:"apiMethod"`
	ApiHeaders   string        `json:"apiHeaders"`
	ApiBody      *string       `json:"apiBody,omitempty"`
	PollInterval int           `json:"pollInterval"`
	Fields       []FieldConfig `json:"fields"`
}

// Add to UpdateUsageMonitorChannelInput:
type UpdateUsageMonitorChannelInput struct {
	Name         *string        `json:"name,omitempty"`
	ApiURL       *string        `json:"apiUrl,omitempty"`
	ApiMethod    *string        `json:"apiMethod,omitempty"`
	ApiHeaders   *string        `json:"apiHeaders,omitempty"`
	ApiKey       *string        `json:"apiKey,omitempty"` // allow key rotation for template channels
	ApiBody      *string        `json:"apiBody,omitempty"`
	PollInterval *int           `json:"pollInterval,omitempty"`
	Fields       *[]FieldConfig `json:"fields,omitempty"`
	Status       *string        `json:"status,omitempty"`
}
```

- [ ] **Step 2: Add header assembly helper to `usage_monitor.go`**

Add a helper function that assembles headers from apiKey + headerFormat:

```go
func assembleHeadersFromAPIKey(apiKey string, headerFormat string) map[string]any {
	switch headerFormat {
	case "bearer":
		return map[string]any{"Authorization": "Bearer " + apiKey}
	case "x-api-key":
		return map[string]any{"x-api-key": apiKey}
	default:
		return map[string]any{"Authorization": "Bearer " + apiKey}
	}
}
```

- [ ] **Step 3: Update `CreateChannel` in `usage_monitor.go` to handle source=template**

After the existing `source` handling, add template logic before the `create :=` statement:

```go
// Handle source=template: auto-fill from template registry
var headerFormat string
if input.Source == "template" {
	if input.ProviderType == nil || *input.ProviderType == "" {
		return nil, fmt.Errorf("providerType is required when source=template")
	}
	if input.ApiKey == nil || *input.ApiKey == "" {
		return nil, fmt.Errorf("apiKey is required when source=template")
	}
	tmpl := usage_monitor.GetQuotaMonitorTemplate(*input.ProviderType)
	if tmpl == nil {
		return nil, fmt.Errorf("unknown provider template: %s", *input.ProviderType)
	}
	input.ApiURL = tmpl.ApiURL
	input.ApiMethod = tmpl.ApiMethod
	if tmpl.ApiBody != "" {
		input.ApiBody = &tmpl.ApiBody
	}
	input.Fields = tmpl.Fields
	headerFormat = tmpl.HeaderFormat

	// Assemble headers from apiKey
	apiHeaders := assembleHeadersFromAPIKey(*input.ApiKey, headerFormat)
	headersBytes, _ := json.Marshal(apiHeaders)
	input.ApiHeaders = string(headersBytes)
}
```

Also in the `create :=` builder, add:

```go
if input.ProviderType != nil {
	create.SetProviderType(usagemonitorchannel.ProviderType(*input.ProviderType))
}
if input.ApiKey != nil {
	create.SetAPIKey(*input.ApiKey)
}
```

- [ ] **Step 4: Update `UpdateChannel` to handle apiKey rotation**

In `UpdateChannel`, add:

```go
if input.ApiKey != nil {
	update.SetAPIKey(*input.ApiKey)
	// Re-assemble headers if this is a template channel
	ch, err := svc.GetChannel(ctx, id)
	if err == nil && ch.Source == usagemonitorchannel.SourceTemplate {
		tmpl := usage_monitor.GetQuotaMonitorTemplate(string(ch.ProviderType))
		if tmpl != nil {
			apiHeaders := assembleHeadersFromAPIKey(*input.ApiKey, tmpl.HeaderFormat)
			update.SetAPIHeaders(apiHeaders)
		}
	}
}
```

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/server/biz/usage_monitor/types.go internal/server/biz/usage_monitor.go
git commit -m "feat(usage-monitor): handle template source in create/update with apiKey assembly"
```

---

### Task 4: Add GraphQL schema and resolvers for templates

**Files:**
- Modify: `internal/server/gql/usage_monitor.graphql`
- Modify: `internal/server/gql/usage_monitor.resolvers.go`

- [ ] **Step 1: Update GraphQL schema**

Add to `usage_monitor.graphql`:

```graphql
type QuotaMonitorTemplate {
  providerType: String!
  name: String!
  description: String
  apiUrl: String!
  apiMethod: UsageMonitorChannelAPIMethod!
  headerFormat: String!
  apiBody: String
  fields: [FieldConfig!]!
}

extend type UsageMonitorChannel {
  providerType: UsageMonitorChannelProviderType
  apiKey: String @goField(forceResolver: true)
}

extend input CreateUsageMonitorChannelInput {
  providerType: UsageMonitorChannelProviderType
  apiKey: String
}

extend input UpdateUsageMonitorChannelInput {
  apiKey: String
}

extend type Query {
  quotaMonitorTemplates: [QuotaMonitorTemplate!]!
}
```

- [ ] **Step 2: Run code generation**

```bash
cd internal/server/gql && go generate
```

Expected: New resolver stubs generated. May need to add `usagemonitorchannel` to `gqlgen.yml` autobind section if type binding fails.

- [ ] **Step 3: Implement resolvers**

Add to `usage_monitor.resolvers.go`:

```go
// QuotaMonitorTemplates is the resolver for the quotaMonitorTemplates field.
func (r *queryResolver) QuotaMonitorTemplates(ctx context.Context) ([]usage_monitor.QuotaMonitorTemplate, error) {
	return usage_monitor.GetQuotaMonitorTemplates(), nil
}

// ProviderType is the resolver for the providerType field.
func (r *usageMonitorChannelResolver) ProviderType(ctx context.Context, obj *ent.UsageMonitorChannel) (*usagemonitorchannel.ProviderType, error) {
	if obj.ProviderType == "" {
		return nil, nil
	}
	pt := usagemonitorchannel.ProviderType(obj.ProviderType)
	return &pt, nil
}

// APIKey is the resolver for the apiKey field.
func (r *usageMonitorChannelResolver) APIKey(ctx context.Context, obj *ent.UsageMonitorChannel) (*string, error) {
	// Only return the key mask for users with write permission
	// Following the same pattern as Channel.credentials
	if obj.APIKey == "" {
		return nil, nil
	}
	masked := "••••••••"
	return &masked, nil
}
```

Also update the `CreateUsageMonitorChannelInput` resolver to handle new fields:

```go
// ProviderType resolver for CreateUsageMonitorChannelInput
func (r *createUsageMonitorChannelInputResolver) ProviderType(ctx context.Context, obj *usage_monitor.CreateUsageMonitorChannelInput, data *usagemonitorchannel.ProviderType) error {
	if data != nil {
		obj.ProviderType = lo.ToPtr(string(*data))
	}
	return nil
}
```

Add the `ProviderType` resolver registration to the Resolver struct if generated.

- [ ] **Step 4: Verify build**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/gql/usage_monitor.graphql internal/server/gql/usage_monitor.resolvers.go internal/server/gql/generated.go internal/server/gql/models_gen.go
git commit -m "feat(gql): add quotaMonitorTemplates query and template channel fields"
```

---

### Task 5: Enhance generic checker to capture response headers

**Files:**
- Modify: `internal/server/biz/usage_monitor/generic_checker.go`

Claude Code's quota data is in response headers (e.g. `anthropic-ratelimit-unified-status`), not in the body. The current generic checker only parses the body. We need to include response headers in the raw data so JSONPath can reference them.

- [ ] **Step 1: Modify `Poll` method to include response headers in poll data**

In `generic_checker.go`, after the HTTP response is received, capture response headers into the raw data map before field parsing. The headers should be nested under a `headers` key in the raw JSON so JSONPath like `$.headers.anthropic-ratelimit-unified-status` works:

```go
// After decoding the response body into rawData, also add response headers:
headerMap := make(map[string]string, len(resp.Header))
for k, v := range resp.Header {
	if len(v) > 0 {
		headerMap[strings.ToLower(k)] = v[0]
	}
}
// Merge headers into the raw data under a "headers" key
rawWithHeaders := map[string]any{}
rawWithHeaders["headers"] = headerMap
if rawMap, ok := rawData.(map[string]any); ok {
	for k, v := range rawMap {
		rawWithHeaders[k] = v
	}
}
// Use rawWithHeaders for field parsing instead of rawData
```

- [ ] **Step 2: Verify build and run tests**

```bash
go build ./... && go test ./internal/server/biz/usage_monitor/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/usage_monitor/generic_checker.go
git commit -m "feat(usage-monitor): include response headers in raw poll data for JSONPath parsing"
```

---

### Task 6: Add frontend i18n keys

**Files:**
- Modify: `frontend/src/locales/en/usage-monitor.json`
- Modify: `frontend/src/locales/zh-CN/usage-monitor.json`

- [ ] **Step 1: Add English keys**

```json
"usageMonitor.source.template": "From Template",
"usageMonitor.apiKey": "API Key",
"usageMonitor.apiKeyPlaceholder": "Enter your API key",
"usageMonitor.templateFields": "Parsed Fields",
"usageMonitor.templateFieldsHint": "These fields are pre-configured by the template",
"usageMonitor.selectTemplate": "Select Provider Template"
```

- [ ] **Step 2: Add Chinese keys**

```json
"usageMonitor.source.template": "从模板",
"usageMonitor.apiKey": "API 密钥",
"usageMonitor.apiKeyPlaceholder": "输入你的 API 密钥",
"usageMonitor.templateFields": "解析字段",
"usageMonitor.templateFieldsHint": "这些字段由模板预配置",
"usageMonitor.selectTemplate": "选择提供商模板"
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/en/usage-monitor.json frontend/src/locales/zh-CN/usage-monitor.json
git commit -m "feat(i18n): add quota monitor template keys"
```

---

### Task 7: Add frontend template data hook

**Files:**
- Create: `frontend/src/features/usage-monitor/data/templates.ts`

- [ ] **Step 1: Create the templates data hook**

```typescript
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import type { FieldConfig } from './schema';

const QUOTA_MONITOR_TEMPLATES_QUERY = `
  query QuotaMonitorTemplates {
    quotaMonitorTemplates {
      providerType
      name
      description
      apiUrl
      apiMethod
      headerFormat
      apiBody
      fields {
        key
        label
        path
        type
        format
        totalPath
        unit
        groupIndex
        displayOrder
      }
    }
  }
`;

export type QuotaMonitorTemplate = {
  providerType: string;
  name: string;
  description?: string;
  apiUrl: string;
  apiMethod: 'GET' | 'POST';
  headerFormat: string;
  apiBody?: string;
  fields: FieldConfig[];
};

export function useQuotaMonitorTemplates() {
  const { data } = useQuery({
    queryKey: ['quotaMonitorTemplates'],
    queryFn: async () => {
      const result = await graphqlRequest<{
        quotaMonitorTemplates: QuotaMonitorTemplate[];
      }>(QUOTA_MONITOR_TEMPLATES_QUERY);
      return result.quotaMonitorTemplates ?? [];
    },
    staleTime: Infinity,
  });
  return data ?? [];
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/features/usage-monitor/data/templates.ts
git commit -m "feat(usage-monitor): add template data hook"
```

---

### Task 8: Update AddChannelDialog with template source

**Files:**
- Modify: `frontend/src/features/usage-monitor/components/add-channel-dialog.tsx`

- [ ] **Step 1: Add template source to the dialog**

Key changes:
1. Change `SourceType` to include `'template'`
2. Add third source button
3. When `source=template`: show template selector + API key input, hide URL/headers/field config
4. Submit with `source=template`, `providerType`, `apiKey`

Update the component to add `useQuotaMonitorTemplates` hook, template state variables, and conditional rendering. The template source section shows:
- Template selector dropdown (from `useQuotaMonitorTemplates()`)
- After selecting: API Key input (password with toggle visibility)
- Read-only field preview showing template fields
- Poll interval input

The submit payload for template:
```typescript
{
  name,
  source: 'template',
  providerType: selectedTemplate.providerType,
  apiKey: apiKey,
  apiUrl: selectedTemplate.apiUrl,  // pre-filled, hidden
  apiMethod: selectedTemplate.apiMethod,
  apiHeaders: '',  // server assembles from apiKey
  pollInterval: pollIntervalMin * 60,
  fields: selectedTemplate.fields,  // pre-filled, hidden
}
```

- [ ] **Step 2: Verify in browser**

Navigate to `/admin/usage-monitor`, click "新增渠道", verify:
- Three source options visible: 从现有渠道 / 手动配置 / 从模板
- Selecting "从模板" shows template dropdown
- Selecting a template shows API Key input
- Field configuration is hidden (or shown read-only)
- Creating a monitor channel works

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/usage-monitor/components/add-channel-dialog.tsx
git commit -m "feat(usage-monitor): add template source to AddChannelDialog"
```

---

### Task 9: Update EditChannelDialog for template channels

**Files:**
- Modify: `frontend/src/features/usage-monitor/components/edit-channel-dialog.tsx`

- [ ] **Step 1: Support editing template channels**

For template channels (`source=template`), the edit dialog should:
- Show name (editable)
- Show API Key input (for key rotation) — pre-filled with masked value `••••••••`
- Show poll interval (editable)
- Show provider type badge (read-only)
- Hide URL, method, headers, field config (these are template-managed)

- [ ] **Step 2: Verify in browser**

Edit a template channel, update API key, verify it saves.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/usage-monitor/components/edit-channel-dialog.tsx
git commit -m "feat(usage-monitor): support editing template channel apiKey"
```

---

### Task 10: Update MonitorCard with provider badge

**Files:**
- Modify: `frontend/src/features/usage-monitor/components/monitor-card.tsx`

- [ ] **Step 1: Show provider type badge for template channels**

When `channel.source === 'template'`, show the provider name as a badge next to the existing source badge. The provider name can be inferred from the `channel.apiUrl` or we can add `providerType` to the GraphQL query response.

Update the monitor card query in `usage-monitor.ts` to include `providerType` field, then in MonitorCard:

```tsx
{channel.source === 'template' && channel.providerType && (
  <Badge variant="outline" className="text-xs">
    {channel.providerType}
  </Badge>
)}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/features/usage-monitor/components/monitor-card.tsx frontend/src/features/usage-monitor/data/usage-monitor.ts frontend/src/features/usage-monitor/data/schema.ts
git commit -m "feat(usage-monitor): show provider type badge on template monitor cards"
```

---

### Task 11: End-to-end verification

- [ ] **Step 1: Run full verification suite**

```bash
go build ./...
cd llm && go build ./...
golangci-lint run --timeout 10m --max-same-issues 50 ./...
cd llm && golangci-lint run --timeout 10m --max-same-issues 50 ./...
go test ./...
cd llm && go test ./...
```

- [ ] **Step 2: Rebuild and restart backend, verify in browser**

```bash
go build -o axonhub.exe ./cmd/axonhub/
```

Restart backend, then in browser:
1. Navigate to `/admin/usage-monitor`
2. Click "新增渠道", select "从模板"
3. Choose a provider template, enter API key
4. Create the channel
5. Verify the monitor card appears with provider badge
6. Edit the channel, update API key
7. Delete the channel

- [ ] **Step 3: Final commit**

```bash
git commit -m "feat(quota-monitor): add provider template support for quota-only monitoring"
```
