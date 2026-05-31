# Quota Monitor Templates

## Problem

The Quota Monitor page currently supports two source types:
- **builtin**: Links to an existing Channel, auto-fills baseURL and API key from its credentials
- **custom**: Fully manual — user must fill API URL, method, headers, and field parsing rules

Neither supports the common use case: **"I just want to monitor my provider's quota with an API key, without creating an actual call channel."** Users must manually figure out the provider's quota API endpoint, request format, and field parsing rules — information that already exists in `ProviderQuotaService`'s hardcoded checkers.

## Solution

Add a third source type `template` that lets users pick a provider template (e.g. "Claude Code", "NanoGPT"), enter only their API key, and get a fully configured monitor channel. The template pre-fills API URL, request method, headers format, and field parsing rules from server-side static configuration that mirrors the existing provider quota checkers.

## Data Model

### Ent Schema Changes (`usage_monitor_channel.go`)

- `source` enum: add `template` value (existing: `builtin`, `custom`)
- New field: `provider_type` (string, optional) — records which provider template this channel was created from (e.g. "claudecode", "nanogpt")
- New field: `api_key` (string, optional, sensitive) — stores the raw API key separately from headers, so key rotation only requires updating this field

### GraphQL Schema Changes (`usage_monitor.graphql`)

New type:

```graphql
type QuotaMonitorTemplate {
  providerType: String!
  name: String!
  description: String
  apiUrl: String!
  apiMethod: UsageMonitorChannelAPIMethod!
  headerFormat: String!       # "bearer" | "x-api-key" — how to place the key
  apiBody: String
  fields: [FieldConfig!]!
}
```

New query:

```graphql
extend type Query {
  quotaMonitorTemplates: [QuotaMonitorTemplate!]!
}
```

Extend `UsageMonitorChannel`:

```graphql
extend type UsageMonitorChannel {
  providerType: String
  # api_key is intentionally NOT exposed via GraphQL
}
```

Extend `CreateUsageMonitorChannelInput`:

```graphql
extend input CreateUsageMonitorChannelInput {
  providerType: String        # required when source=template
  apiKey: String              # required when source=template
}
```

Extend `UpdateUsageMonitorChannelInput`:

```graphql
extend input UpdateUsageMonitorChannelInput {
  apiKey: String              # allow key rotation
}
```

## Backend

### Template Registry

Add `internal/server/biz/usage_monitor/templates.go` with a static slice of `QuotaMonitorTemplate` values. Each template's `apiUrl`, `apiMethod`, `headerFormat`, `apiBody`, and `fields` are extracted from the corresponding provider quota checker's known configuration.

Initial templates (8 providers):

| providerType | name | apiUrl | headerFormat | apiMethod |
|---|---|---|---|---|
| claudecode | Claude Code | `https://api.anthropic.com/v1/messages` | bearer | POST |
| codex | Codex / ChatGPT | `https://chatgpt.com/backend-api/wham/usage` | bearer | GET |
| github_copilot | GitHub Copilot | `https://api.githubcopilot.com/quotas` | bearer | GET |
| nanogpt | NanoGPT | `https://app.nanogpt.ai/api/subscription/v1/usage` | bearer | GET |
| wafer | Wafer | `https://api.wafer.ai/v1/inference/quota` | bearer | GET |
| synthetic | Synthetic | `https://api.synthetic.new/v2/quotas` | bearer | GET |
| neuralwatt | NeuralWatt | `https://api.neuralwatt.com/v1/quota` | bearer | GET |
| zhipu | Zhipu / Z.ai | `https://open.bigmodel.cn/api/paas/v4/quota` | bearer | GET |

Each template also includes the `fields` array with JSONPath/regex parsing rules matching what the corresponding checker uses internally.

### Resolver

`quotaMonitorTemplates` query returns the static template list.

### Header Assembly

When `source=template`, the poll logic assembles headers from `api_key` + `headerFormat`:
- `bearer`: `{"Authorization": "Bearer <api_key>"}`
- `x-api-key`: `{"x-api-key": "<api_key>"}`

This replaces the previous behavior of reading `api_headers` directly. The `api_headers` field is still stored (as the assembled result) for backward compatibility with the generic checker, but the source of truth for template channels is `api_key` + `headerFormat`.

### Create/Update Flow

On `createUsageMonitorChannel` with `source=template`:
1. Validate `providerType` exists in template registry
2. Look up template, auto-fill `apiUrl`, `apiMethod`, `fields`, `apiBody`
3. Assemble `api_headers` from `apiKey` + template's `headerFormat`
4. Store `provider_type` and `api_key` (encrypted or plaintext per existing pattern)

On `updateUsageMonitorChannel` with new `apiKey`:
1. Re-assemble `api_headers` from new key + stored `headerFormat`

## Frontend

### AddChannelDialog Changes

Add third source option `template` alongside `builtin` and `custom`.

When `source=template` is selected:

1. Show provider template selector (dropdown populated from `quotaMonitorTemplates` query)
2. After selecting a template, show:
   - Template name and description (read-only)
   - API Key input (single text input, type=password with toggle)
   - Poll interval input
   - Read-only field configuration preview (shows what fields will be monitored, but no editing)
3. Hide: API URL, method, headers editor, field config form, request body

The submit payload sets `source=template`, `providerType`, `apiKey`, and `name`. The server fills in the rest from the template.

### MonitorCard Changes

For template channels, show the provider type as a badge/tag (e.g. "Claude Code") in addition to the existing source badge.

### i18n

New keys needed:
- `usageMonitor.source.template` — "From Template" / "从模板"
- `usageMonitor.apiKey` — "API Key" / "API 密钥"
- `usageMonitor.templateFields` — "Parsed Fields" / "解析字段"
- `usageMonitor.templateFieldsHint` — "These fields are pre-configured by the template" / "这些字段由模板预配置"

## Security

- `api_key` field must NOT be returned by any GraphQL query (similar to how Channel credentials are handled)
- `api_key` is stored in the database — follow the same encryption/at-rest pattern as Channel API keys if applicable

## Testing

- Backend: unit test for template registry completeness (all 8 providers present)
- Backend: unit test for header assembly logic (bearer vs x-api-key)
- Backend: integration test for create + poll with template source
- Frontend: verify template selector populates, API key input works, field preview shows correctly
