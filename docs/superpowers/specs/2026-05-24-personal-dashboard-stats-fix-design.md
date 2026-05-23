# Personal Dashboard Statistics Fix Design

## Problem Statement

The personal dashboard (`mode="personal"`) shows inconsistent and incorrect data. Only 1 out of 16+ queries is scoped to the user's projects (`myDashboard`). All breakdown queries (channel, model, API key, cost, performance, daily stats, token stats) return global/project-wide data regardless of dashboard mode. Additional bugs include inconsistent data sources, missing features, navigation errors, and unused code.

## Bug Inventory

| # | Bug | Severity | Impact |
|---|-----|----------|--------|
| 1 | Personal scope mismatch: 15/16 breakdown queries show global data | HIGH | Users see wrong data in personal dashboard |
| 2 | Inconsistent data sources: `requests` vs `usage_logs` tables | MEDIUM | TotalRequests won't match sum of period counts |
| 3 | `averageResponseTime` always null | LOW | Missing feature, defined but unimplemented |
| 4 | Personal "View All" link navigates to admin route | MEDIUM | 404 or permission error |
| 5 | 4+3 sequential DB queries for RequestStats/TokenStats | MEDIUM | Performance concern on large tables |
| 6 | `TopRequestsProjects` limit placement ambiguity | LOW-MEDIUM | May return wrong number of groups |
| 7 | Unused frontend code + reasoningTokens queried but not displayed | LOW | Code bloat, confusing schema |

## Section 1: Personal-Scope Resolvers (Bug 1)

### Approach: Separate `myXxx` Resolvers

Create parallel `myXxx` GraphQL query resolvers that hardcode user/project scoping, keeping admin/personal completely separate.

### New GraphQL Queries

Add to `publish_request.graphql` (extending `Query`):

```
myRequestStats(timeWindow: String): RequestStats!
myRequestStatsByChannel(timeWindow: String): [RequestStatsByChannel!]!
myRequestStatsByModel(timeWindow: String): [RequestStatsByModel!]!
myRequestStatsByAPIKey(timeWindow: String): [RequestStatsByAPIKey!]!
myTokenStatsByAPIKey(timeWindow: String): [TokenStatsByAPIKey!]!
myDailyRequestStats: [DailyRequestStats!]!
myTokenStats: TokenStats!
myChannelSuccessRates(timeWindow: String, limit: Int): [ChannelSuccessRate!]!
myFastestChannels(input: FastestChannelsInput!): [FastestChannel!]!
myFastestModels(input: FastestChannelsInput!): [FastestModel!]!
myModelPerformanceStats: [ModelPerformanceStat!]!
myChannelPerformanceStats: [ChannelPerformanceStat!]!
myTokenStatsByChannel(timeWindow: String): [TokenStatsByChannel!]!
myTokenStatsByModel(timeWindow: String): [TokenStatsByModel!]!
myCostStatsByChannel(timeWindow: String): [CostStatsByChannel!]!
myCostStatsByModel(timeWindow: String): [CostStatsByModel!]!
myCostStatsByAPIKey(timeWindow: String): [CostStatsByAPIKey!]!
myTopRequestsProjects: [TopRequestsProjects!]!
```

### Backend Implementation Pattern

Each `myXxx` resolver follows this pattern:

1. Get authenticated user from context (`contexts.GetUser(ctx)`).
2. Collect user's project IDs via helper function `getUserProjectIDs(ctx, r.client, r.userService)` (extracted from `MyDashboard` lines 86-107).
3. Apply `authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)`.
4. Execute the same aggregation query as the admin version, but inject `projectIDIn(projectIDs...)` (or equivalent raw SQL WHERE clause) on `usage_logs`, `requests`, and `request_executions` tables.
5. Return the same type as the admin version.

**Helper extraction**: Move project ID collection from `MyDashboard` (lines 86-107) into a shared helper:

```go
func getUserProjectIDs(ctx context.Context, client *ent.Client, userService *biz.UserService) ([]int, error) {
    user, ok := contexts.GetUser(ctx)
    if !ok || user == nil {
        return nil, fmt.Errorf("unauthorized")
    }
    projectIDs, err := client.UserProject.Query().
        Where(userproject.UserIDEQ(user.ID)).
        QueryProject().
        IDs(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get user projects: %w", err)
    }
    if len(projectIDs) == 0 {
        privateProjectID, err := userService.EnsurePrivateProject(ctx, user)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve private project: %w", err)
        }
        projectIDs = []int{privateProjectID}
    }
    return projectIDs, nil
}
```

For resolvers that use raw SQL (most breakdown queries), the project filter is injected as an additional WHERE clause: `AND usage_logs.project_id IN (<project_ids>)`. For Ent-based queries, add `.Where(usagelog.ProjectIDIn(projectIDs...))`.

### Resolver File Organization

All `myXxx` resolvers go in `publish_request.resolvers.go` alongside `MyDashboard`, since they share the same user-scoping pattern. After gqlgen code generation, the resolver stubs will appear there.

### Frontend Changes

Each data hook in `dashboard.ts` and `fastest-performers.ts` gains a `mode: DashboardMode` parameter. When `mode === 'personal'`, it calls the `myXxx` GraphQL field; when `mode === 'project'`, it calls the existing admin field.

Pattern (example for `useRequestsByChannel`):

```typescript
export function useRequestsByChannel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['requestStatsByChannel', timeWindow, mode],
    queryFn: async () => {
      const fieldName = isPersonal ? 'myRequestStatsByChannel' : 'requestStatsByChannel';
      const query = isPersonal ? MY_REQUESTS_BY_CHANNEL_QUERY : REQUESTS_BY_CHANNEL_QUERY;
      const data = await graphqlRequest<{ [key: string]: RequestsByChannel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => requestsByChannelSchema.parse(item));
    },
    ...
  });
}
```

The `DashboardPage` component passes `mode` down to all child components, and each component passes `mode` to its data hooks.

**Component prop changes**: All chart/breakdown components receive a `mode` prop and pass it to their hooks:
- `RequestsByChannelChart` → `useRequestsByChannel(timePeriod, mode)`
- `TokenStatsCard` → `useTokenStats(mode)`
- `DailyRequestStats` → `useDailyRequestStats(mode)`
- `ChannelSuccessRate` → `useChannelSuccessRates(mode)`
- etc.

## Section 2: Data Source Consistency (Bug 2)

Keep the existing dual-source approach consistent with the admin panel. `TotalRequests` and `FailedRequests` come from the `requests` table (all statuses), while `RequestStats` period counts come from `usage_logs` (recorded/successful requests). This is intentional: the requests table tracks process lifecycle, usage_logs tracks billing/usage data.

No code changes needed. The inconsistency is a design choice, not a bug. The frontend should display these with appropriate labels (e.g., "Total Requests (all statuses)" vs "Requests (recorded)").

## Section 3: Average Response Time (Bug 3)

Compute `AverageResponseTime` from the `request_executions` table, which stores `latency_ms` and `first_token_latency_ms` fields.

### Backend Implementation

For `DashboardOverview`: add a query after the status counts:

```go
// Average latency across completed request executions
var avgResult []struct {
    AvgLatency float64 `json:"avg_latency"`
}
if err := r.client.RequestExecution.Query().
    Where(requestexecution.StatusEQ(requestexecution.StatusCompleted)).
    Aggregate(ent.As(ent.Mean(requestexecution.FieldLatencyMs), "avg_latency")).
    Scan(ctx, &avgResult); err == nil && len(avgResult) > 0 {
    stats.AverageResponseTime = avgResult[0].AvgLatency
}
```

For `MyDashboard`: same query with `requestexecution.ProjectIDIn(projectIDs...)` filter.

### GraphQL Schema

The `averageResponseTime` field is already defined as `Float` on `DashboardOverview`. No schema change needed.

### Frontend

The frontend already parses `averageResponseTime: z.number().nullable()`. Display it in `TotalRequestsCard` or a new card showing avg response time.

## Section 4: Personal "View All" Navigation Fix (Bug 4)

### Change

In `DashboardContent` (index.tsx:188), make the link conditional on `mode`:

```tsx
<Link
  to={mode === 'personal' ? '/dashboard/channel-success-rates' : '/admin/dashboard/channel-success-rates'}
  className='text-sm text-primary hover:underline'
>
```

Additionally, create a personal route for channel success rates at `frontend/src/routes/_authenticated/dashboard/channel-success-rates.tsx` that uses `mode="personal"`, mirroring the admin route but with personal-scoped data.

## Section 5: Query Optimization (Bug 5)

### RequestStats Consolidation

Replace 4 separate `UsageLog.Query().Count()` calls with a single conditional SUM query:

```go
var periodCounts []struct {
    TodayCount     int `json:"today_count"`
    ThisWeekCount  int `json:"this_week_count"`
    ThisMonthCount int `json:"this_month_count"`
}
if err := r.client.UsageLog.Query().
    Where(usagelog.CreatedAtGTE(period.Today.Start)).
    Modify(func(s *sql.Selector) {
        s.Select(
            sql.As(sql.Count(sql.Column("*")), "today_count"),
            sql.As(sql.Sum(sql.Case(sql.When(usagelog.CreatedAtGTE(period.ThisWeek.Start), 1), sql.Else(0))), "this_week_count"),
            sql.As(sql.Sum(sql.Case(sql.When(usagelog.CreatedAtGTE(period.ThisMonth.Start), 1), sql.Else(0))), "this_month_count"),
        )
    }).
    Scan(ctx, &periodCounts); err == nil && len(periodCounts) > 0 {
    stats.RequestsToday = periodCounts[0].TodayCount
    stats.RequestsThisWeek = periodCounts[0].ThisWeekCount
    stats.RequestsThisMonth = periodCounts[0].ThisMonthCount
}
// LastWeek still needs a separate query since it has both start and end bounds
```

Same pattern for `TokenStats` — consolidate 3 `getTokenSums` calls into a single conditional SUM.

### Implementation Notes

The CASE/WHEN syntax varies by dialect. Use Ent's `sql` package dialect-aware helpers, or build the conditional SUM in raw SQL within `Modify()`. LastWeek needs a separate query since it has a bounded range (start AND end).

## Section 6: TopRequestsProjects Limit Fix (Bug 6)

Move the `Limit(10)` into the `Modify()` function so it applies after GROUP BY, not before. Replace:

```go
r.client.UsageLog.Query().Limit(10).Modify(func(s *sql.Selector) { ... })
```

With:

```go
r.client.UsageLog.Query().Modify(func(s *sql.Selector) {
    // ... GROUP BY and ORDER BY ...
    s.Limit(10)
})
```

This ensures the limit constrains the grouped output, not the input rows.

## Section 7: Unused Code Cleanup (Bug 7)

### Remove unused frontend code:
- Delete `frontend/src/features/dashboard/components/top-projects.tsx` and its hook from `dashboard.ts` (`useTopProjects`, `TOP_PROJECTS_QUERY`)
- Delete `frontend/src/features/dashboard/components/requests-by-time-card.tsx`
- Delete `useHourlyRequestStats` and `HOURLY_REQUEST_STATS_QUERY` from `dashboard.ts`
- Remove the `HourlyRequestStats` type from `dashboard.graphql` if no backend resolver uses it

### ReasoningTokens display:
- Option A (recommended): Add `reasoningTokens` to the stacked bar charts alongside `inputTokens`, `outputTokens`, `cachedTokens`. This makes the token breakdown complete.
- Option B: Remove `reasoningTokens` from the GraphQL queries and Zod schemas if it's not needed for display.

## Implementation Sequence

1. Extract `getUserProjectIDs` helper from `MyDashboard`
2. Add `myXxx` GraphQL queries to `publish_request.graphql`
3. Run gqlgen code generation
4. Implement `myXxx` resolvers in `publish_request.resolvers.go`
5. Update frontend hooks to accept `mode` parameter
6. Update frontend components to pass `mode` to hooks
7. Create personal channel-success-rates route
8. Fix "View All" navigation link
9. Implement `AverageResponseTime` computation
10. Consolidate RequestStats/TokenStats queries
11. Fix `TopRequestsProjects` limit placement
12. Clean up unused frontend code
13. Add `reasoningTokens` to chart display or remove from queries

## Scope Boundaries

- All `myXxx` resolvers reuse existing GraphQL types — no new types needed
- Admin dashboard behavior is unchanged — all `myXxx` resolvers are additive
- Frontend changes are incremental — each hook gains a `mode` parameter without breaking existing calls (default `'project'`)