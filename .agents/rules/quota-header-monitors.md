---
alwaysApply: false
globs: "frontend/src/components/quota-badges.tsx, frontend/src/features/usage-monitor/**/*.ts, frontend/src/features/usage-monitor/**/*.tsx, internal/server/biz/usage_monitor*.go, internal/server/biz/usage_monitor/**/*.go, internal/server/biz/provider_quota*.go, internal/server/gql/ent.resolvers.go"
---

# Header Quota Popover and Usage Monitors

This fork surfaces **user-added quota monitors** (custom and template) in the header battery popover. Upstream only lists built-in `provider_quota_status` checkers. Do not drop this bridge on merge.

## Two stores, one popover

| Store | Source | Header row |
|---|---|---|
| `provider_quota_status` | Built-in checkers (`channel.providerQuotaStatus`) | `QuotaRow` |
| `usage_monitor_channels` | `/admin/usage-monitor` (builtin / custom / template) | `MonitorQuotaRow` |

`QuotaBadges` always queries `useUsageMonitorChannels` and lists every monitor. **Do not gate monitors on a bound channel, credentials, `quotaReady`, or any other "activation" prerequisite.** A monitor the user added must appear even when polling is in error.

## Template badges

Monitor cards on `/admin/usage-monitor` render parsed fields with `SharedFieldRenderer` + `BadgeDisplay` (`displayFields.badge` / `badgePresets`). The header popover **must reuse that renderer**. Do not reimplement text fields as plain `{String(f.value)}` — that drops Plan / Access Type / Chat TRUE gradient badges.

## Period quota estimates on templates

`fillMonitorPeriodQuotaEstimates` prices each derived limit window the same way built-in checkers do (`periodCost` / `periodQuota`). Cost attribution:

1. Bound `channelID` when present.
2. Otherwise **every enabled channel whose `channelProviderType` matches the monitor's `providerType`** (a template describes that provider's account-level pool).
3. Providers with no AxonHub channels keep their limits without an estimate.

`DeriveQuotaStatus` must stamp `Window` + `PeriodStart` on Codex / MiniMax / OpenCode Go / xAI / Cline (and any similar windowed template) so the estimator has a period to query. Iterate windows with a **slice**, never `for range` over a map — map order scrambles popover labels.

The GraphQL `Map` scalar for `quotaLimits` is `{ items: [...] }`. The `QuotaLimits` resolver returns that wrapper; it must not panic. Frontend `parseQuotaLimits` reads `quotaLimits.items`.

## Embed order

The production UI is `go:embed` of `frontend/dist`. After frontend changes: stop `axonhub.exe` if it holds `dist`, build the frontend, then `go build -o axonhub.exe ./cmd/axonhub/`. Building the binary first embeds a stale UI.
