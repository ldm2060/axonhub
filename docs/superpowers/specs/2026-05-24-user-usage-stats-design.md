# Admin Dashboard — Per-User Statistics

## Problem

The admin dashboard currently shows statistics grouped by channel, model, and API key, but has no view into per-user usage and activity. Administrators need to see which users are consuming resources, who is active, and identify top consumers.

## Design Decision

**Approach: Cached aggregation table (materialized view pattern)**

A `user_usage_stats` table stores pre-aggregated per-user statistics, refreshed every 5 minutes by a scheduled task. Queries read from this table through the existing `live.IndexedCache` pattern.

**Why not alternatives:**
- Pure SQL aggregation (Approach A): Two-step join through API Key is acceptable but requires expensive on-the-fly aggregation. With the cache table, queries are instant.
- Adding `user_id` to UsageLog (Approach B): Requires schema migration, request pipeline changes, and historical data backfill. Heavier change for marginal benefit.
- Per-day granularity: Not needed — the dashboard only needs current totals and `last_active_at` for time-range filtering.

## Data Model

### New Ent Schema: `UserUsageStats`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| user_id | int | yes | FK to User (0 = unattributed) |
| request_count | int | yes | Total requests |
| success_count | int | yes | Successful requests |
| prompt_tokens | int64 | yes | Prompt token consumption |
| completion_tokens | int64 | yes | Completion token consumption |
| total_tokens | int64 | yes | Total token consumption |
| total_cost | float64 | yes | Total cost |
| last_active_at | time | no | Most recent request timestamp |
| updated_at | time | auto | Record update time |

- One row per user (user_id is unique index).
- `user_id = 0` represents unattributed requests (noauth / service account keys).
- No `date` dimension — all-time aggregation. Time-range filtering uses `last_active_at`.

### Aggregation Logic

1. **Join path**: `UsageLog.api_key_id → APIKey.user_id`
2. **Scheduler** runs every 5 minutes via the existing FX lifecycle hooks.
3. **Full recalculation** on each run (user count is small, full recalc is fast):
   - Aggregate UsageLog by `api_key_id` (request_count, success_count, tokens, cost).
   - Group by `APIKey.user_id` to merge per-user.
   - Upsert into `user_usage_stats`.
   - Set `last_active_at` = max(created_at) from UsageLog for that user's keys.
4. **Cache layer**: `live.IndexedCache` with softTTL = 3 min, hardTTL = 10 min (same pattern as existing dashboard resolvers).

### Active User Definition

A user is "active in time range X" if `last_active_at` falls within that range:
- 7-day active: `last_active_at >= now - 7d`
- 30-day active: `last_active_at >= now - 30d`

## GraphQL API

### Schema Additions

```graphql
type UserUsageStat {
  userID: Int!
  userName: String!
  userEmail: String!
  requestCount: Int!
  successCount: Int!
  successRate: Float!
  promptTokens: Int!
  completionTokens: Int!
  totalTokens: Int!
  totalCost: Float!
  lastActiveAt: Time
}

type UserUsageStatsPayload {
  stats: [UserUsageStat!]!
  totalUsers: Int!
  activeUsers7d: Int!
  activeUsers30d: Int!
}

extend type Query {
  userUsageStats(
    timeRange: TimeRange!
    search: String
    sortBy: UserStatsSortField!
    sortOrder: SortOrder!
    page: Int!
    pageSize: Int!
  ): UserUsageStatsPayload!
}

enum UserStatsSortField {
  REQUEST_COUNT
  TOTAL_COST
  TOTAL_TOKENS
  LAST_ACTIVE_AT
}

enum TimeRange {
  LAST_7D
  LAST_30D
  ALL
}
```

### Resolver Behavior

- `timeRange`: Filters users by `last_active_at`. `LAST_7D` shows only users active in the last 7 days, `ALL` shows everyone.
- `search`: Case-insensitive match on user name or email.
- `sortBy` + `sortOrder`: Orders the result set.
- Pagination: `page` (1-based) and `pageSize` (default 20).
- `totalUsers`, `activeUsers7d`, `activeUsers30d`: Computed from the full dataset (before search/pagination filters) for the summary cards.

## Frontend

### Placement

New collapsible section "按用户统计" within the existing admin dashboard page (`/admin`), alongside existing statistics sections.

### Layout

1. **Summary card row**: Total Users | 7-Day Active | 30-Day Active — each showing a count with a subtle indicator (e.g., colored dot for active).
2. **Filter toolbar**: Time range dropdown (Last 7D / Last 30D / All) + Search input + Sort field dropdown + Sort order toggle.
3. **Data table**: Columns — User Name, Email, Requests, Success Rate, Token Usage, Cost, Last Active. Clickable headers for sorting. Paginated.

Default sort: `REQUEST_COUNT` descending. Default time range: `ALL`.

### Component Structure

- Reuse existing dashboard component patterns (collapsible section, filter toolbar, data table).
- GraphQL hooks follow the same pattern as existing dashboard queries (`useUserUsageStatsQuery`).
- i18n keys added to `en.json` and `zh.json`.

## Files to Create/Modify

### Backend (new)
- `internal/ent/schema/user_usage_stats.go` — Ent schema
- `internal/server/biz/user_usage_stats.go` — Biz service (aggregation logic, cache)
- `internal/server/gql/user_usage_stats.resolvers.go` — GraphQL resolver

### Backend (modify)
- `internal/server/gql/dashboard.graphql` — Add types and query
- `cmd/axonhub/main.go` or FX module — Register scheduler and biz service
- `internal/ent/client.go` (generated) — After `go generate`

### Frontend (new)
- `frontend/src/features/dashboard/data/user-usage-stats.ts` — GraphQL query, hooks
- `frontend/src/features/dashboard/components/UserUsageStatsSection.tsx` — Main section component

### Frontend (modify)
- `frontend/src/features/dashboard/pages/AdminDashboardPage.tsx` — Add the new section
- `frontend/src/locales/en.json`, `zh.json` — Add i18n keys

## Out of Scope

- Per-user detail page with time-series charts (future enhancement)
- Per-day historical data (not needed for current requirements)
- Real-time updates (5-minute refresh is sufficient)
- Adding `user_id` to UsageLog (not needed with aggregation approach)
