# Per-User Statistics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-user statistics section to the admin dashboard, backed by a pre-aggregated `user_usage_stats` cache table refreshed every 5 minutes by a scheduled task.

**Architecture:** New Ent schema `UserUsageStats` stores one row per user with all-time aggregated metrics. A biz service `UserUsageStatsService` owns the aggregation logic and a `live.IndexedCache` for reads. A scheduler task runs the aggregation every 5 minutes. A GraphQL query exposes the data. The frontend adds a collapsible section with summary cards, filter toolbar, and paginated table.

**Tech Stack:** Go + Ent ORM + gqlgen + FX (backend), React + TanStack Query + TanStack Table + Tailwind + i18n (frontend)

---

## Task 1: Create the UserUsageStats Ent Schema

**Files:**
- Create: `internal/ent/schema/user_usage_stats.go`

- [ ] **Step 1: Write the Ent schema**

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
)

type UserUsageStats struct {
	ent.Schema
}

func (UserUsageStats) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_usage_stats"},
	}
}

func (UserUsageStats) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.Int("request_count").Default(0),
		field.Int("success_count").Default(0),
		field.Int64("prompt_tokens").Default(0),
		field.Int64("completion_tokens").Default(0),
		field.Int64("total_tokens").Default(0),
		field.Float("total_cost").Default(0),
		field.Time("last_active_at").Optional().Nillable(),
	}
}

func (UserUsageStats) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_usage_stats").
			Field("user_id").
			Unique().
			Annotations(schematype.Directives(forceResolver())),
	}
}

func (UserUsageStats) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").StorageKey("user_usage_stats_user_id_key").Unique(),
	}
}

func (UserUsageStats) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}
```

- [ ] **Step 2: Add the back-reference edge to the User schema**

Add to `internal/ent/schema/user.go` in the `Edges()` method:

```go
edge.To("user_usage_stats", UserUsageStats.Type),
```

Place it alongside the other `edge.To` declarations in the user schema file.

- [ ] **Step 3: Run go generate and verify build**

Run: `cd internal/ent && go generate ./...`
Then: `go build ./...`

Expected: No errors. New `user_usage_stats` package and client methods are generated.

- [ ] **Step 4: Commit**

```bash
git add internal/ent/schema/user_usage_stats.go internal/ent/schema/user.go internal/ent/
git commit -m "feat(ent): add UserUsageStats schema for per-user aggregated statistics"
```

---

## Task 2: Create the UserUsageStats Biz Service

**Files:**
- Create: `internal/server/biz/user_usage_stats.go`
- Modify: `internal/server/biz/fx_module.go`

- [ ] **Step 1: Write the biz service**

```go
package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/apikey"
	"github.com/ldm2060/axonhub/internal/ent/usagelog"
	"github.com/ldm2060/axonhub/internal/ent/userusagestats"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
	"go.uber.org/fx"
)

type UserUsageStatsService struct {
	*AbstractService
	SystemService *SystemService
	cache         *live.IndexedCache[int, *ent.UserUsageStats]
	mu            sync.Mutex
}

type UserUsageStatsServiceParams struct {
	fx.In
	*AbstractService
	*SystemService
}

func NewUserUsageStatsService(p UserUsageStatsServiceParams) *UserUsageStatsService {
	svc := &UserUsageStatsService{
		AbstractService: p.AbstractService,
		SystemService:   p.SystemService,
	}
	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UserUsageStats]{
		Name:            "axonhub:user_usage_stats",
		TTL:             10 * time.Minute,
		RefreshInterval: 3 * time.Minute,
		DebounceDelay:   500 * time.Millisecond,
		KeyFunc:         func(v *ent.UserUsageStats) int { return v.UserID },
		DeletedFunc:     func(v *ent.UserUsageStats) bool { return v.DeletedAt != 0 },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
	})
	return svc
}

func (svc *UserUsageStatsService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "user-usage-stats",
		Description: "Refresh per-user aggregated usage statistics",
		CronExpr:    "*/5 * * * *",
		Timezone:    "UTC",
	}, svc.refreshStats)
}

func (svc *UserUsageStatsService) Start(ctx context.Context) error {
	if err := svc.cache.Load(ctx); err != nil {
		return fmt.Errorf("user usage stats cache initial load failed: %w", err)
	}
	return nil
}

func (svc *UserUsageStatsService) Stop() {
	svc.cache.Stop()
}

func (svc *UserUsageStatsService) QueryUserUsageStats(ctx context.Context, timeRange string, search string, sortBy string, sortOrder string, page int, pageSize int) ([]*ent.UserUsageStats, int, int, int, error) {
	loc := svc.SystemService.TimeLocation(ctx)

	var cutoff7d, cutoff30d time.Time
	now := time.Now()
	cutoff7d = now.AddDate(0, 0, -7)
	cutoff30d = now.AddDate(0, 0, -30)

	var timeCutoff *time.Time
	switch timeRange {
	case "LAST_7D":
		t := cutoff7d
		timeCutoff = &t
	case "LAST_30D":
		t := cutoff30d
		timeCutoff = &t
	default:
	}

	allStats := svc.cache.GetAll()
	totalUsers := len(allStats)
	active7d := 0
	active30d := 0
	for _, s := range allStats {
		if s.LastActiveAt != nil && !s.LastActiveAt.IsZero() {
			if !s.LastActiveAt.Before(cutoff7d) {
				active7d++
			}
			if !s.LastActiveAt.Before(cutoff30d) {
				active30d++
			}
		}
	}

	var filtered []*ent.UserUsageStats
	for _, s := range allStats {
		if timeCutoff != nil && (s.LastActiveAt == nil || s.LastActiveAt.Before(*timeCutoff)) {
			continue
		}
		if search != "" {
			user, err := svc.db.User.Get(ctx, s.UserID)
			if err != nil {
				continue
			}
			nameMatch := user.FirstName != "" && containsCI(user.FirstName+" "+user.LastName, search)
			emailMatch := containsCI(user.Email, search)
			if !nameMatch && !emailMatch {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	sortStats(filtered, sortBy, sortOrder)

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	_ = loc
	return filtered[start:end], totalUsers, active7d, active30d, nil
}

func (svc *UserUsageStatsService) refreshStats(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.recalculateAllStats(ctx)
	svc.cache.ReloadAll(ctx)
}

func (svc *UserUsageStatsService) recalculateAllStats(ctx context.Context) {
	db := svc.entFromContext(ctx)

	type apiKeyAgg struct {
		UserID         int     `json:"user_id"`
		RequestCount   int     `json:"request_count"`
		SuccessCount   int     `json:"success_count"`
		PromptTokens   int64   `json:"prompt_tokens"`
		CompTokens     int64   `json:"completion_tokens"`
		TotalTokens    int64   `json:"total_tokens"`
		TotalCost      float64 `json:"total_cost"`
		LastActiveAt   *time.Time `json:"last_active_at"`
	}

	var results []apiKeyAgg

	usageLogTable := sql.Table(usagelog.Table)
	apiKeyTable := sql.Table(apikey.Table)

	err := db.UsageLog.Query().
		Modify(func(s *sql.Selector) {
			s.Join(apiKeyTable).On(s.C(usagelog.FieldAPIKeyID), apiKeyTable.C(apikey.FieldID))
			s.Where(sql.EQ(apiKeyTable.C(apikey.FieldDeletedAt), 0))
			s.GroupBy(apiKeyTable.C(apikey.FieldUserID))
			s.Select(
				sql.As(apiKeyTable.C(apikey.FieldUserID), "user_id"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
				sql.As(sql.Sum(sql.Expr("(CASE WHEN r.status = 'success' THEN 1 ELSE 0 END)")), "success_count"),
				sql.As(sql.Sum(s.C(usagelog.FieldPromptTokens)), "prompt_tokens"),
				sql.As(sql.Sum(s.C(usagelog.FieldCompletionTokens)), "completion_tokens"),
				sql.As(sql.Sum(s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(sql.Sum(s.C(usagelog.FieldTotalCost)), "total_cost"),
				sql.As(sql.Max(s.C(usagelog.FieldCreatedAt)), "last_active_at"),
			)
		}).
		Scan(ctx, &results)

	if err != nil {
		return
	}

	for _, r := range results {
		if r.UserID == 0 {
			continue
		}
		existing, err := db.UserUsageStats.Query().
			Where(userusagestats.UserID(r.UserID)).
			Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			continue
		}
		if ent.IsNotFound(err) {
			build := db.UserUsageStats.Create().
				SetUserID(r.UserID).
				SetRequestCount(r.RequestCount).
				SetSuccessCount(r.SuccessCount).
				SetPromptTokens(r.PromptTokens).
				SetCompletionTokens(r.CompTokens).
				SetTotalTokens(r.TotalTokens).
				SetTotalCost(r.TotalCost)
			if r.LastActiveAt != nil {
				build = build.SetLastActiveAt(*r.LastActiveAt)
			}
			build.Exec(ctx)
		} else {
			update := db.UserUsageStats.UpdateOne(existing).
				SetRequestCount(r.RequestCount).
				SetSuccessCount(r.SuccessCount).
				SetPromptTokens(r.PromptTokens).
				SetCompletionTokens(r.CompTokens).
				SetTotalTokens(r.TotalTokens).
				SetTotalCost(r.TotalCost)
			if r.LastActiveAt != nil {
				update = update.SetLastActiveAt(*r.LastActiveAt)
			}
			update.Exec(ctx)
		}
	}
}

func (svc *UserUsageStatsService) loadOne(ctx context.Context, userID int) (*ent.UserUsageStats, error) {
	return svc.db.UserUsageStats.Query().
		Where(userusagestats.UserID(userID)).
		Only(ctx)
}

func (svc *UserUsageStatsService) loadSince(ctx context.Context, since time.Time) ([]*ent.UserUsageStats, time.Time, error) {
	q := svc.db.UserUsageStats.Query()
	if !since.IsZero() {
		q = q.Where(userusagestats.UpdatedAtGT(since))
	}
	items, err := q.All(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	var maxTime time.Time
	for _, item := range items {
		if item.UpdatedAt.After(maxTime) {
			maxTime = item.UpdatedAt
		}
	}
	return items, maxTime, nil
}

func containsCI(s, substr string) bool {
	s = lower(s)
	substr = lower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func lower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func sortStats(stats []*ent.UserUsageStats, sortBy string, sortOrder string) {
	less := func(i, j int) bool {
		var less bool
		switch sortBy {
		case "REQUEST_COUNT":
			less = stats[i].RequestCount < stats[j].RequestCount
		case "TOTAL_COST":
			less = stats[i].TotalCost < stats[j].TotalCost
		case "TOTAL_TOKENS":
			less = stats[i].TotalTokens < stats[j].TotalTokens
		case "LAST_ACTIVE_AT":
			if stats[i].LastActiveAt == nil && stats[j].LastActiveAt == nil {
				less = false
			} else if stats[i].LastActiveAt == nil {
				less = true
			} else if stats[j].LastActiveAt == nil {
				less = false
			} else {
				less = stats[i].LastActiveAt.Before(*stats[j].LastActiveAt)
			}
		default:
			less = stats[i].RequestCount < stats[j].RequestCount
		}
		if sortOrder == "DESC" {
			return !less
		}
		return less
	}
	n := len(stats)
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if less(i, j) {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
}
```

- [ ] **Step 2: Register the service in the FX module**

Add to `internal/server/biz/fx_module.go`:

In the `fx.Module("biz", ...)` call, add to the `fx.Provide` list:
```go
fx.Provide(NewUserUsageStatsService),
```

Add a new `fx.Invoke` block after the existing scheduler registrations:
```go
fx.Invoke(func(lc fx.Lifecycle, svc *UserUsageStatsService, s *scheduler.Scheduler) {
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            if err := svc.Start(ctx); err != nil {
                return err
            }
            return svc.RegisterScheduledTasks(ctx, s)
        },
        OnStop: func(ctx context.Context) error {
            svc.Stop()
            return nil
        },
    })
}),
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

Expected: No compilation errors.

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/user_usage_stats.go internal/server/biz/fx_module.go
git commit -m "feat(biz): add UserUsageStatsService with scheduler and cache"
```

---

## Task 3: Add GraphQL Schema and Resolver

**Files:**
- Modify: `internal/server/gql/dashboard.graphql`
- Create: `internal/server/gql/user_usage_stats.resolvers.go`

- [ ] **Step 1: Add types and query to `dashboard.graphql`**

Append to the end of `internal/server/gql/dashboard.graphql`:

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

extend type Query {
  userUsageStats(
    timeRange: TimeRange!
    search: String
    sortBy: UserStatsSortField!
    sortOrder: OrderDirection!
    page: Int!
    pageSize: Int!
  ): UserUsageStatsPayload!
}
```

Note: `OrderDirection` already exists in `ent.graphql` with `ASC` and `DESC` values.

- [ ] **Step 2: Run gqlgen code generation**

Run: `cd internal/server/gql && go run github.com/99designs/gqlgen/graphql-go generate`

Expected: `models_gen.go` is updated with new types. A stub resolver file `user_usage_stats.resolvers.go` is created.

- [ ] **Step 3: Implement the resolver**

Create `internal/server/gql/user_usage_stats.resolvers.go`:

```go
package gql

import (
	"context"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/gql/model"
)

func (r *queryResolver) UserUsageStats(ctx context.Context, timeRange model.TimeRange, search *string, sortBy model.UserStatsSortField, sortOrder model.OrderDirection, page int, pageSize int) (*model.UserUsageStatsPayload, error) {
	ctx = authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)

	timeRangeStr := string(timeRange)
	searchStr := ""
	if search != nil {
		searchStr = *search
	}
	sortByStr := string(sortBy)
	sortOrderStr := string(sortOrder)

	stats, totalUsers, active7d, active30d, err := r.userUsageStatsService.QueryUserUsageStats(ctx, timeRangeStr, searchStr, sortByStr, sortOrderStr, page, pageSize)
	if err != nil {
		return nil, err
	}

	userStats := make([]*model.UserUsageStat, 0, len(stats))
	for _, s := range stats {
		user, err := r.client.User.Get(ctx, s.UserID)
		if err != nil {
			continue
		}
		successRate := 0.0
		if s.RequestCount > 0 {
			successRate = float64(s.SuccessCount) / float64(s.RequestCount)
		}

		userStats = append(userStats, &model.UserUsageStat{
			UserID:           s.UserID,
			UserName:         user.FirstName + " " + user.LastName,
			UserEmail:        user.Email,
			RequestCount:     s.RequestCount,
			SuccessCount:     s.SuccessCount,
			SuccessRate:      successRate,
			PromptTokens:     int(s.PromptTokens),
			CompletionTokens: int(s.CompletionTokens),
			TotalTokens:      int(s.TotalTokens),
			TotalCost:        s.TotalCost,
			LastActiveAt:     s.LastActiveAt,
		})
	}

	return &model.UserUsageStatsPayload{
		Stats:         userStats,
		TotalUsers:    totalUsers,
		ActiveUsers7d: active7d,
		ActiveUsers30d: active30d,
	}, nil
}
```

- [ ] **Step 4: Wire the biz service into the resolver**

In the resolver struct definition (likely in `internal/server/gql/resolver.go`), add:

```go
UserUsageStatsService *biz.UserUsageStatsService
```

And update the FX wiring that constructs the resolver to inject this dependency.

- [ ] **Step 5: Update gqlgen.yml model mappings**

In `internal/server/gql/gqlgen.yml`, add to the `models:` section:

```yaml
UserUsageStat:
  model:
    - github.com/ldm2060/axonhub/internal/server/gql/model.UserUsageStat
UserUsageStatsPayload:
  model:
    - github.com/ldm2060/axonhub/internal/server/gql/model.UserUsageStatsPayload
```

Note: The enums `UserStatsSortField`, `TimeRange` will be auto-generated in `models_gen.go` since they are not explicitly mapped.

- [ ] **Step 6: Verify build**

Run: `go build ./...`

Expected: No compilation errors.

- [ ] **Step 7: Commit**

```bash
git add internal/server/gql/dashboard.graphql internal/server/gql/user_usage_stats.resolvers.go internal/server/gql/resolver.go internal/server/gql/gqlgen.yml internal/server/gql/model/ internal/server/gql/models_gen.go
git commit -m "feat(gql): add userUsageStats query with pagination, search, and sort"
```

---

## Task 4: Fix Aggregation SQL for Request Status

**Files:**
- Modify: `internal/server/biz/user_usage_stats.go`

The `recalculateAllStats` method joins `UsageLog` with `APIKey`, but needs the `Request` table for success status. The `UsageLog` table itself does not have a `status` field. The success count must be computed by joining through `RequestExecution` or `Request`.

- [ ] **Step 1: Update the SQL aggregation to use RequestExecution for success count**

Replace the success count SQL expression in `recalculateAllStats`. Instead of:
```go
sql.As(sql.Sum(sql.Expr("(CASE WHEN r.status = 'success' THEN 1 ELSE 0 END)")), "success_count"),
```

Use a subquery or join with `RequestExecution`:
```go
requestExecTable := sql.Table(requestexecution.Table)
s.Join(requestExecTable).On(s.C(usagelog.FieldRequestID), requestExecTable.C(requestexecution.FieldRequestID))
```

And change the success count to:
```go
sql.As(sql.Sum(sql.Expr("(CASE WHEN request_execution.status = 'success' THEN 1 ELSE 0 END)")), "success_count"),
```

Add the import: `"github.com/ldm2060/axonhub/internal/ent/requestexecution"`

- [ ] **Step 2: Verify build**

Run: `go build ./...`

Expected: No compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/user_usage_stats.go
git commit -m "fix(biz): join RequestExecution for success count in user usage stats aggregation"
```

---

## Task 5: Create Frontend Data Layer

**Files:**
- Create: `frontend/src/features/dashboard/data/user-usage-stats.ts`

- [ ] **Step 1: Write the data layer with Zod schemas, GraphQL query, and React Query hooks**

```typescript
import { z } from 'zod'
import { graphqlRequest } from '@/lib/api-client'
import { useQuery } from '@tanstack/react-query'

// Schemas
export const userUsageStatSchema = z.object({
  userID: z.number(),
  userName: z.string(),
  userEmail: z.string(),
  requestCount: z.number(),
  successCount: z.number(),
  successRate: z.number(),
  promptTokens: z.number(),
  completionTokens: z.number(),
  totalTokens: z.number(),
  totalCost: z.number(),
  lastActiveAt: z.string().nullable(),
})

export const userUsageStatsPayloadSchema = z.object({
  stats: z.array(userUsageStatSchema),
  totalUsers: z.number(),
  activeUsers7d: z.number(),
  activeUsers30d: z.number(),
})

// Types
export type UserUsageStat = z.infer<typeof userUsageStatSchema>
export type UserUsageStatsPayload = z.infer<typeof userUsageStatsPayloadSchema>

export type UserStatsSortField = 'REQUEST_COUNT' | 'TOTAL_COST' | 'TOTAL_TOKENS' | 'LAST_ACTIVE_AT'
export type TimeRange = 'LAST_7D' | 'LAST_30D' | 'ALL'

// GraphQL query
const USER_USAGE_STATS_QUERY = `
  query UserUsageStats(
    $timeRange: TimeRange!
    $search: String
    $sortBy: UserStatsSortField!
    $sortOrder: OrderDirection!
    $page: Int!
    $pageSize: Int!
  ) {
    userUsageStats(
      timeRange: $timeRange
      search: $search
      sortBy: $sortBy
      sortOrder: $sortOrder
      page: $page
      pageSize: $pageSize
    ) {
      stats {
        userID
        userName
        userEmail
        requestCount
        successCount
        successRate
        promptTokens
        completionTokens
        totalTokens
        totalCost
        lastActiveAt
      }
      totalUsers
      activeUsers7d
      activeUsers30d
    }
  }
`

// Hook
export function useUserUsageStats(
  timeRange: TimeRange = 'ALL',
  search: string = '',
  sortBy: UserStatsSortField = 'REQUEST_COUNT',
  sortOrder: 'ASC' | 'DESC' = 'DESC',
  page: number = 1,
  pageSize: number = 20,
) {
  return useQuery({
    queryKey: ['userUsageStats', timeRange, search, sortBy, sortOrder, page, pageSize],
    queryFn: async () => {
      const data = await graphqlRequest<{
        userUsageStats: unknown
      }>(USER_USAGE_STATS_QUERY, {
        timeRange,
        search: search || null,
        sortBy,
        sortOrder,
        page,
        pageSize,
      })
      return userUsageStatsPayloadSchema.parse(data.userUsageStats)
    },
    refetchInterval: 300000,
    placeholderData: (previousData) => previousData,
  })
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`

Expected: No type errors related to the new file.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/dashboard/data/user-usage-stats.ts
git commit -m "feat(frontend): add user usage stats data layer with GraphQL query and hooks"
```

---

## Task 6: Create Frontend UserUsageStatsSection Component

**Files:**
- Create: `frontend/src/features/dashboard/components/user-usage-stats-section.tsx`

- [ ] **Step 1: Write the component with summary cards, filter toolbar, and paginated table**

```tsx
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  ColumnDef,
  flexRender,
} from '@tanstack/react-table'
import { Users, Activity, TrendingUp, ArrowUpDown, Loader2, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription, CardAction } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { ServerSidePagination } from '@/components/server-side-pagination'
import { DataTableColumnHeader } from '@/components/data-table-column-header'
import { CollapsibleSection } from './collapsible-section'
import {
  useUserUsageStats,
  type UserUsageStat,
  type UserStatsSortField,
  type TimeRange,
} from '../data/user-usage-stats'

function formatNumber(n: number): string {
  return n.toLocaleString()
}

function formatCost(cost: number): string {
  return `$${cost.toFixed(4)}`
}

function formatSuccessRate(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toString()
}

function formatDate(dateStr: string | null): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

export function UserUsageStatsSection() {
  const { t } = useTranslation()
  const [timeRange, setTimeRange] = useState<TimeRange>('ALL')
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [sortBy, setSortBy] = useState<UserStatsSortField>('REQUEST_COUNT')
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC')
  const [page, setPage] = useState(1)
  const pageSize = 20

  let debounceTimer: ReturnType<typeof setTimeout>
  const handleSearchChange = (value: string) => {
    setSearch(value)
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => {
      setDebouncedSearch(value)
      setPage(1)
    }, 300)
  }

  const { data, isLoading, isFetching } = useUserUsageStats(
    timeRange,
    debouncedSearch,
    sortBy,
    sortOrder,
    page,
    pageSize,
  )

  const columns = useMemo<ColumnDef<UserUsageStat>[]>(
    () => [
      {
        accessorKey: 'userName',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.userName')} />
        ),
        cell: ({ row }) => <span className="font-medium">{row.original.userName}</span>,
      },
      {
        accessorKey: 'userEmail',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.email')} />
        ),
      },
      {
        accessorKey: 'requestCount',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.requests')} />
        ),
        cell: ({ row }) => formatNumber(row.original.requestCount),
      },
      {
        accessorKey: 'successRate',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.successRate')} />
        ),
        cell: ({ row }) => (
          <span className={row.original.successRate >= 0.99 ? 'text-green-600' : row.original.successRate >= 0.9 ? 'text-yellow-600' : 'text-red-600'}>
            {formatSuccessRate(row.original.successRate)}
          </span>
        ),
      },
      {
        accessorKey: 'totalTokens',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.tokens')} />
        ),
        cell: ({ row }) => formatTokenCount(row.original.totalTokens),
      },
      {
        accessorKey: 'totalCost',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.cost')} />
        ),
        cell: ({ row }) => formatCost(row.original.totalCost),
      },
      {
        accessorKey: 'lastActiveAt',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('dashboard.userStats.columns.lastActive')} />
        ),
        cell: ({ row }) => formatDate(row.original.lastActiveAt),
      },
    ],
    [t],
  )

  const table = useReactTable({
    data: data?.stats ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    pageCount: Math.ceil((data?.stats.length ?? 0) / pageSize),
  })

  return (
    <CollapsibleSection
      title={t('dashboard.userStats.title')}
      icon={<Users className="h-4 w-4" />}
      storageKey="user-usage-stats"
    >
      <div className="space-y-4">
        {/* Summary cards */}
        <div className="grid grid-cols-3 gap-4">
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>{t('dashboard.userStats.cards.totalUsers')}</CardDescription>
              <CardTitle className="text-2xl">{formatNumber(data?.totalUsers ?? 0)}</CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>{t('dashboard.userStats.cards.active7d')}</CardDescription>
              <CardTitle className="text-2xl flex items-center gap-2">
                {formatNumber(data?.activeUsers7d ?? 0)}
                <span className="h-2 w-2 rounded-full bg-green-500" />
              </CardTitle>
            </CardHeader>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>{t('dashboard.userStats.cards.active30d')}</CardDescription>
              <CardTitle className="text-2xl flex items-center gap-2">
                {formatNumber(data?.activeUsers30d ?? 0)}
                <span className="h-2 w-2 rounded-full bg-green-500" />
              </CardTitle>
            </CardHeader>
          </Card>
        </div>

        {/* Filter toolbar */}
        <div className="flex items-center gap-3">
          <Select value={timeRange} onValueChange={(v) => { setTimeRange(v as TimeRange); setPage(1) }}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">{t('dashboard.userStats.timeRange.all')}</SelectItem>
              <SelectItem value="LAST_7D">{t('dashboard.userStats.timeRange.last7d')}</SelectItem>
              <SelectItem value="LAST_30D">{t('dashboard.userStats.timeRange.last30d')}</SelectItem>
            </SelectContent>
          </Select>

          <div className="relative flex-1 max-w-xs">
            <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t('dashboard.userStats.searchPlaceholder')}
              value={search}
              onChange={(e) => handleSearchChange(e.target.value)}
              className="pl-8"
            />
          </div>

          <Select value={sortBy} onValueChange={(v) => { setSortBy(v as UserStatsSortField); setPage(1) }}>
            <SelectTrigger className="w-[160px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="REQUEST_COUNT">{t('dashboard.userStats.sortBy.requestCount')}</SelectItem>
              <SelectItem value="TOTAL_COST">{t('dashboard.userStats.sortBy.totalCost')}</SelectItem>
              <SelectItem value="TOTAL_TOKENS">{t('dashboard.userStats.sortBy.totalTokens')}</SelectItem>
              <SelectItem value="LAST_ACTIVE_AT">{t('dashboard.userStats.sortBy.lastActiveAt')}</SelectItem>
            </SelectContent>
          </Select>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setSortOrder(sortOrder === 'ASC' ? 'DESC' : 'ASC')}
          >
            <ArrowUpDown className="h-4 w-4 mr-1" />
            {sortOrder === 'ASC' ? t('dashboard.userStats.sortOrder.asc') : t('dashboard.userStats.sortOrder.desc')}
          </Button>
        </div>

        {/* Data table */}
        <div className="relative overflow-auto rounded-2xl border shadow-soft">
          {isFetching && !isLoading && (
            <div className="absolute right-2 top-2 z-10">
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            </div>
          )}
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <TableHead key={header.id}>
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="h-24 text-center">
                    <Loader2 className="h-6 w-6 animate-spin mx-auto text-muted-foreground" />
                  </TableCell>
                </TableRow>
              ) : table.getRowModel().rows.length ? (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.original.userID}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell colSpan={columns.length} className="h-24 text-center text-muted-foreground">
                    {t('dashboard.userStats.noData')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>

        {/* Pagination */}
        {data && (
          <ServerSidePagination
            pageInfo={{
              hasPreviousPage: page > 1,
              hasNextPage: data.stats.length === pageSize,
            }}
            pageSize={pageSize}
            dataLength={data.stats.length}
            totalCount={data.totalUsers}
            onPreviousPage={() => setPage(page - 1)}
            onNextPage={() => setPage(page + 1)}
            onFirstPage={() => setPage(1)}
            onPageSizeChange={() => {}}
          />
        )}
      </div>
    </CollapsibleSection>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`

Expected: No type errors in the new component.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/dashboard/components/user-usage-stats-section.tsx
git commit -m "feat(frontend): add UserUsageStatsSection with summary cards, filters, and table"
```

---

## Task 7: Add i18n Keys

**Files:**
- Modify: `frontend/src/locales/en/dashboard.json`
- Modify: `frontend/src/locales/zh-CN/dashboard.json`

- [ ] **Step 1: Add English i18n keys**

Add to `frontend/src/locales/en/dashboard.json`:

```json
{
  "userStats": {
    "title": "User Statistics",
    "searchPlaceholder": "Search users...",
    "noData": "No user data available",
    "cards": {
      "totalUsers": "Total Users",
      "active7d": "7-Day Active",
      "active30d": "30-Day Active"
    },
    "columns": {
      "userName": "User",
      "email": "Email",
      "requests": "Requests",
      "successRate": "Success Rate",
      "tokens": "Tokens",
      "cost": "Cost",
      "lastActive": "Last Active"
    },
    "timeRange": {
      "all": "All Time",
      "last7d": "Last 7 Days",
      "last30d": "Last 30 Days"
    },
    "sortBy": {
      "requestCount": "Requests",
      "totalCost": "Cost",
      "totalTokens": "Tokens",
      "lastActiveAt": "Last Active"
    },
    "sortOrder": {
      "asc": "Asc",
      "desc": "Desc"
    }
  }
}
```

- [ ] **Step 2: Add Chinese i18n keys**

Add to `frontend/src/locales/zh-CN/dashboard.json`:

```json
{
  "userStats": {
    "title": "用户统计",
    "searchPlaceholder": "搜索用户...",
    "noData": "暂无用户数据",
    "cards": {
      "totalUsers": "总用户数",
      "active7d": "7日活跃",
      "active30d": "30日活跃"
    },
    "columns": {
      "userName": "用户",
      "email": "邮箱",
      "requests": "请求数",
      "successRate": "成功率",
      "tokens": "Token",
      "cost": "费用",
      "lastActive": "最后活跃"
    },
    "timeRange": {
      "all": "全部时间",
      "last7d": "最近7天",
      "last30d": "最近30天"
    },
    "sortBy": {
      "requestCount": "请求数",
      "totalCost": "费用",
      "totalTokens": "Token数",
      "lastActiveAt": "最后活跃"
    },
    "sortOrder": {
      "asc": "升序",
      "desc": "降序"
    }
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/en/dashboard.json frontend/src/locales/zh-CN/dashboard.json
git commit -m "feat(i18n): add user statistics translations for en and zh-CN"
```

---

## Task 8: Wire UserUsageStatsSection into Admin Dashboard Page

**Files:**
- Modify: `frontend/src/features/dashboard/index.tsx`

- [ ] **Step 1: Import and render the component**

In `frontend/src/features/dashboard/index.tsx`:

Add the import at the top:
```tsx
import { UserUsageStatsSection } from './components/user-usage-stats-section'
```

Add the component in the `DashboardContent` function, after the API Keys collapsible section and before the Performance section (or at the end before `ReviewQueue`):

```tsx
<UserUsageStatsSection />
```

- [ ] **Step 2: Verify the app builds**

Run: `cd frontend && npx tsc --noEmit`

Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/dashboard/index.tsx
git commit -m "feat(dashboard): add UserUsageStatsSection to admin dashboard"
```

---

## Task 9: Integration Testing and Verification

**Files:**
- No new files

- [ ] **Step 1: Run full backend build**

Run: `go build ./... && cd llm && go build ./...`

Expected: No errors.

- [ ] **Step 2: Run backend lint**

Run: `golangci-lint run --timeout 10m --max-same-issues 50 ./... && cd llm && golangci-lint run --timeout 10m --max-same-issues 50 ./...`

Expected: No new lint errors.

- [ ] **Step 3: Run backend tests**

Run: `go test ./... && cd llm && go test ./...`

Expected: All tests pass.

- [ ] **Step 4: Run frontend type check**

Run: `cd frontend && npx tsc --noEmit`

Expected: No type errors.

- [ ] **Step 5: Manual verification — start the dev server and check the admin dashboard**

Open `/admin` in the browser. Verify:
1. The "User Statistics" collapsible section appears
2. Summary cards show user counts
3. Time range dropdown filters users
4. Search input filters by name/email
5. Sort controls change table order
6. Table columns render correctly
7. Pagination works

- [ ] **Step 6: Commit final state**

```bash
git add -A
git commit -m "chore: verify and finalize per-user statistics feature"
```