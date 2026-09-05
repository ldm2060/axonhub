package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/apikey"
	"github.com/ldm2060/axonhub/internal/ent/requestexecution"
	"github.com/ldm2060/axonhub/internal/ent/usagelog"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/ent/userusagestats"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
	"github.com/ldm2060/axonhub/internal/pkg/xtext"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
)

// TimeRange defines the time range for filtering usage stats.
type TimeRange string

const (
	TimeRangeLast7D  TimeRange = "LAST_7D"
	TimeRangeLast30D TimeRange = "LAST_30D"
	TimeRangeAll     TimeRange = "ALL"
)

// SortField defines the field to sort by.
type SortField string

const (
	SortFieldRequestCount SortField = "request_count"
	SortFieldTotalTokens  SortField = "total_tokens"
	SortFieldTotalCost    SortField = "total_cost"
	SortFieldLastActiveAt SortField = "last_active_at"
)

// UserUsageStatsServiceParams contains dependencies for UserUsageStatsService.
type UserUsageStatsServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
}

// UserUsageStatsService handles user usage statistics operations.
type UserUsageStatsService struct {
	*AbstractService

	SystemService *SystemService
	cache         *live.IndexedCache[int, *ent.UserUsageStats]
	mu            sync.Mutex
}

// NewUserUsageStatsService creates a new UserUsageStatsService.
func NewUserUsageStatsService(params UserUsageStatsServiceParams) *UserUsageStatsService {
	svc := &UserUsageStatsService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		SystemService: params.SystemService,
	}

	svc.cache = live.NewIndexedCache(live.IndexedOptions[int, *ent.UserUsageStats]{
		Name:            "user_usage_stats",
		TTL:             5 * time.Minute,
		RefreshInterval: 30 * time.Second,
		KeyFunc:         func(v *ent.UserUsageStats) int { return v.UserID },
		LoadOneFunc:     svc.loadOne,
		LoadSinceFunc:   svc.loadSince,
	})

	return svc
}

// Start initializes the cache by loading all records.
func (svc *UserUsageStatsService) Start(ctx context.Context) error {
	return svc.cache.Load(ctx)
}

// Stop gracefully stops the cache background workers.
func (svc *UserUsageStatsService) Stop() {
	svc.cache.Stop()
}

// RegisterScheduledTasks registers the periodic refresh task.
func (svc *UserUsageStatsService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "user-usage-stats-refresh",
		Description: "Recalculate user usage stats every 5 minutes",
		CronExpr:    "*/5 * * * *",
		Timezone:    "UTC",
	}, svc.refreshStats)
}

// refreshStats recalculates all stats from usage logs and reloads the cache.
// It is protected by a mutex to prevent concurrent refreshes.
func (svc *UserUsageStatsService) refreshStats(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if err := svc.recalculateAllStats(ctx); err != nil {
		log.Error(ctx, "failed to recalculate user usage stats", log.Cause(err))
		return
	}

	if err := svc.cache.Load(ctx); err != nil {
		log.Error(ctx, "failed to reload user usage stats cache", log.Cause(err))
	}
}

// usageAggregation holds the aggregated result from the usage log query.
type usageAggregation struct {
	UserID           int        `json:"user_id"`
	RequestCount     int        `json:"request_count"`
	SuccessCount     int        `json:"success_count"`
	PromptTokens     int64      `json:"prompt_tokens"`
	CompletionTokens int64      `json:"completion_tokens"`
	TotalTokens      int64      `json:"total_tokens"`
	TotalCost        float64    `json:"total_cost"`
	LastActiveAt     *time.Time `json:"last_active_at"`
}

// scanLastActiveAt scans the result of MAX(created_at) into *time.Time.
//
// Raw SQL aggregation bypasses ent's (de)serialization, so the driver returns
// the timestamp in whatever form it stores: modernc/sqlite returns a Go
// time.String() like "2006-01-02 15:04:05.999999999 +0000 UTC" (and may append a
// monotonic " m=-..." suffix); Postgres/MySQL typically return time.Time or
// RFC3339 text. time.Time's standard layouts do not cover the sqlite form, so
// scan into any and coerce.
func scanLastActiveAt(src any, dst **time.Time) error {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		t := v
		*dst = &t
		return nil
	case *time.Time:
		if v != nil {
			*dst = v
		}
		return nil
	case string:
		t, err := parseAggregatedTime(v)
		if err != nil {
			return err
		}
		*dst = &t
		return nil
	case []byte:
		t, err := parseAggregatedTime(string(v))
		if err != nil {
			return err
		}
		*dst = &t
		return nil
	}
	return fmt.Errorf("unsupported last_active_at scan source type %T", src)
}

// parseAggregatedTime parses a timestamp produced by MAX(created_at).
//
// Layouts cover RFC3339 (Postgres/MySQL text) and Go time.String() (modernc
// sqlite), including the optional monotonic " m=-..." suffix which is stripped
// first since time.Parse has no layout token for it.
func parseAggregatedTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if i := strings.Index(s, " m="); i >= 0 {
		s = s[:i]
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format %q: %w", s, lastErr)
}

// recalculateAllStats aggregates UsageLog by api_key_id joined with APIKey to group by user_id,
// then upserts the results into UserUsageStats.
func (svc *UserUsageStatsService) recalculateAllStats(ctx context.Context) error {
	client := svc.entFromContext(ctx)

	dbDriver := client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return fmt.Errorf("failed to get underlying SQL driver")
	}

	query := fmt.Sprintf(`
			SELECT
				ak.%s as user_id,
				COUNT(*) as request_count,
				SUM(CASE WHEN re.%s = 'success' THEN 1 ELSE 0 END) as success_count,
				COALESCE(SUM(ul.%s), 0) as prompt_tokens,
				COALESCE(SUM(ul.%s), 0) as completion_tokens,
				COALESCE(SUM(ul.%s), 0) as total_tokens,
				COALESCE(SUM(ul.%s), 0) as total_cost,
				MAX(ul.%s) as last_active_at
			FROM %s ul
			JOIN %s ak ON ul.%s = ak.%s
			JOIN %s re ON ul.%s = re.%s
			WHERE ak.%s = 0
			GROUP BY ak.%s
			HAVING ak.%s > 0
		`,
		apikey.FieldUserID,
		requestexecution.FieldStatus,
		usagelog.FieldPromptTokens,
		usagelog.FieldCompletionTokens,
		usagelog.FieldTotalTokens,
		usagelog.FieldTotalCost,
		usagelog.FieldCreatedAt,
		usagelog.Table,
		apikey.Table,
		usagelog.FieldAPIKeyID,
		apikey.FieldID,
		requestexecution.Table,
		usagelog.FieldRequestID,
		requestexecution.FieldRequestID,
		apikey.FieldDeletedAt,
		apikey.FieldUserID,
		apikey.FieldUserID,
	)

	rows, err := sqlDB.DB().QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to aggregate user usage stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var aggregations []usageAggregation
	for rows.Next() {
		var (
			agg          usageAggregation
			lastActiveAt any
		)
		if err := rows.Scan(
			&agg.UserID,
			&agg.RequestCount,
			&agg.SuccessCount,
			&agg.PromptTokens,
			&agg.CompletionTokens,
			&agg.TotalTokens,
			&agg.TotalCost,
			&lastActiveAt,
		); err != nil {
			return fmt.Errorf("failed to scan usage aggregation: %w", err)
		}
		if err := scanLastActiveAt(lastActiveAt, &agg.LastActiveAt); err != nil {
			return fmt.Errorf("failed to scan usage aggregation last_active_at: %w", err)
		}
		aggregations = append(aggregations, agg)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating usage aggregations: %w", err)
	}

	// Upsert into UserUsageStats
	for _, agg := range aggregations {
		existing, err := client.UserUsageStats.Query().
			Where(userusagestats.UserIDEQ(agg.UserID)).
			First(ctx)

		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("failed to query existing user usage stats for user %d: %w", agg.UserID, err)
		}

		if existing != nil {
			// Update existing record
			update := client.UserUsageStats.UpdateOneID(existing.ID).
				SetRequestCount(agg.RequestCount).
				SetSuccessCount(agg.SuccessCount).
				SetPromptTokens(agg.PromptTokens).
				SetCompletionTokens(agg.CompletionTokens).
				SetTotalTokens(agg.TotalTokens).
				SetTotalCost(agg.TotalCost)

			if agg.LastActiveAt != nil {
				update.SetLastActiveAt(*agg.LastActiveAt)
			}

			if err := update.Exec(ctx); err != nil {
				return fmt.Errorf("failed to update user usage stats for user %d: %w", agg.UserID, err)
			}
		} else {
			// Create new record
			create := client.UserUsageStats.Create().
				SetUserID(agg.UserID).
				SetRequestCount(agg.RequestCount).
				SetSuccessCount(agg.SuccessCount).
				SetPromptTokens(agg.PromptTokens).
				SetCompletionTokens(agg.CompletionTokens).
				SetTotalTokens(agg.TotalTokens).
				SetTotalCost(agg.TotalCost)

			if agg.LastActiveAt != nil {
				create.SetLastActiveAt(*agg.LastActiveAt)
			}

			if _, err := create.Save(ctx); err != nil {
				return fmt.Errorf("failed to create user usage stats for user %d: %w", agg.UserID, err)
			}
		}
	}

	log.Info(ctx, "recalculated user usage stats", log.Int("user_count", len(aggregations)))

	return nil
}

// loadOne loads a single UserUsageStats record by user ID for the IndexedCache.
func (svc *UserUsageStatsService) loadOne(ctx context.Context, userID int) (*ent.UserUsageStats, error) {
	client := svc.entFromContext(ctx)

	item, err := client.UserUsageStats.Query().
		Where(userusagestats.UserIDEQ(userID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, live.ErrKeyNotFound
		}
		return nil, err
	}

	return item, nil
}

// loadSince loads UserUsageStats records updated since the given time for the IndexedCache.
func (svc *UserUsageStatsService) loadSince(ctx context.Context, since time.Time) ([]*ent.UserUsageStats, time.Time, error) {
	client := svc.entFromContext(ctx)

	q := client.UserUsageStats.Query()
	if !since.IsZero() {
		q = q.Where(userusagestats.UpdatedAtGT(since))
	}

	items, err := q.All(ctx)
	if err != nil {
		return nil, since, err
	}

	maxUpdated := since
	for _, item := range items {
		if item.UpdatedAt.After(maxUpdated) {
			maxUpdated = item.UpdatedAt
		}
	}

	return items, maxUpdated, nil
}

// QueryUserUsageStatsInput defines the input parameters for QueryUserUsageStats.
type QueryUserUsageStatsInput struct {
	TimeRange TimeRange
	Search    string
	SortField SortField
	SortOrder string // "asc" or "desc"
	Page      int    // 1-based
	PageSize  int
}

// QueryUserUsageStatsResult holds the result of QueryUserUsageStats.
type QueryUserUsageStatsResult struct {
	Stats      []*ent.UserUsageStats
	TotalUsers int
	Active7D   int
	Active30D  int
}

// QueryUserUsageStats queries user usage statistics from the cache with filtering, sorting, and pagination.
func (svc *UserUsageStatsService) QueryUserUsageStats(ctx context.Context, input QueryUserUsageStatsInput) (*QueryUserUsageStatsResult, error) {
	allStats := svc.cache.GetAll()

	now := time.Now()
	cutoff7D := now.Add(-7 * 24 * time.Hour)
	cutoff30D := now.Add(-30 * 24 * time.Hour)

	var active7D, active30D int

	// Build slice from cache, counting active users
	statsSlice := make([]*ent.UserUsageStats, 0, len(allStats))
	for _, stat := range allStats {
		if stat.LastActiveAt != nil {
			if stat.LastActiveAt.After(cutoff7D) {
				active7D++
			}
			if stat.LastActiveAt.After(cutoff30D) {
				active30D++
			}

			// Apply time range filter
			switch input.TimeRange {
			case TimeRangeLast7D:
				if !stat.LastActiveAt.After(cutoff7D) {
					continue
				}
			case TimeRangeLast30D:
				if !stat.LastActiveAt.After(cutoff30D) {
					continue
				}
			}
		}

		statsSlice = append(statsSlice, stat)
	}

	// Apply search filter by looking up users by name/email
	if input.Search != "" {
		if len(statsSlice) > 0 {
			searchLower := strings.ToLower(input.Search)
			userIDs := make([]int, 0, len(statsSlice))
			for _, stat := range statsSlice {
				userIDs = append(userIDs, stat.UserID)
			}

			users, err := svc.db.User.Query().
				Where(user.IDIn(userIDs...)).
				All(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to query users: %w", err)
			}

			userMap := make(map[int]*ent.User, len(users))
			for _, u := range users {
				userMap[u.ID] = u
			}

			filtered := statsSlice[:0]
			for _, stat := range statsSlice {
				u, ok := userMap[stat.UserID]
				if !ok {
					continue
				}
				fullName := xtext.FormatUserName(u.FirstName, u.LastName)
				if strings.Contains(strings.ToLower(fullName), searchLower) ||
					strings.Contains(strings.ToLower(u.Email), searchLower) {
					filtered = append(filtered, stat)
				}
			}
			statsSlice = filtered
		}
	}

	totalUsers := len(statsSlice)

	// Sort
	sortOrder := input.SortOrder
	if sortOrder != "asc" {
		sortOrder = "desc"
	}

	sort.Slice(statsSlice, func(i, j int) bool {
		si, sj := statsSlice[i], statsSlice[j]
		var less bool
		switch input.SortField {
		case SortFieldRequestCount:
			less = si.RequestCount < sj.RequestCount
		case SortFieldTotalTokens:
			less = si.TotalTokens < sj.TotalTokens
		case SortFieldTotalCost:
			less = si.TotalCost < sj.TotalCost
		case SortFieldLastActiveAt:
			switch {
			case si.LastActiveAt == nil && sj.LastActiveAt == nil:
				less = false
			case si.LastActiveAt == nil:
				less = true
			case sj.LastActiveAt == nil:
				less = false
			default:
				less = si.LastActiveAt.Before(*sj.LastActiveAt)
			}
		default:
			less = si.TotalTokens < sj.TotalTokens
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})

	// Paginate (1-based page)
	page := max(input.Page, 1)
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	if start >= len(statsSlice) {
		return &QueryUserUsageStatsResult{
			Stats:      []*ent.UserUsageStats{},
			TotalUsers: totalUsers,
			Active7D:   active7D,
			Active30D:  active30D,
		}, nil
	}

	end := min(start+pageSize, len(statsSlice))

	return &QueryUserUsageStatsResult{
		Stats:      statsSlice[start:end],
		TotalUsers: totalUsers,
		Active7D:   active7D,
		Active30D:  active30D,
	}, nil
}
