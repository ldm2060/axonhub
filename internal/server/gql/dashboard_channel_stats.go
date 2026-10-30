package gql

// Channel and model throughput/success-rate implementations shared by the
// dashboard caching resolver wrappers. Kept out of *.resolvers.go because
// gqlgen regenerates those files and would drop non-resolver functions.

import (
	"context"
	"fmt"
	"sort"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/requestexecution"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/pkg/xtime"
	"github.com/ldm2060/axonhub/internal/server/gql/qb"
)

func normalizeChannelSuccessRatesParams(timeWindow *string, limit *int) (*string, *int) {
	window := "day"
	if timeWindow != nil && *timeWindow != "" {
		switch *timeWindow {
		case "day", "week", "month":
			window = *timeWindow
		default:
			// Unknown windows are all-time in parseTimeWindow; canonicalize
			// them so distinct spellings share one cache key.
			window = "allTime"
		}
	}

	limitCount := 0
	if limit != nil && *limit > 0 {
		limitCount = *limit
	}

	return &window, &limitCount
}

func (r *queryResolver) channelSuccessRates(ctx context.Context, timeWindow *string, limit *int) ([]*ChannelSuccessRate, error) {
	// Parse time window, default to "day"
	if timeWindow == nil || *timeWindow == "" {
		defaultWindow := "day"
		timeWindow = &defaultWindow
	}
	since, applyFilter := r.parseTimeWindow(ctx, timeWindow)

	// Handle limit parameter (0 means no limit)
	limitCount := 0
	if limit != nil && *limit > 0 {
		limitCount = *limit
	}

	type channelExecutionStats struct {
		ChannelID    int `json:"channel_id"`
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}

	var results []channelExecutionStats

	// Step 1: Get success/failure counts from request_execution
	err := r.client.RequestExecution.Query().
		Modify(func(s *sql.Selector) {
			s.Select(
				requestexecution.FieldChannelID,
				sql.As("SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)", "success_count"),
				sql.As("SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END)", "failed_count"),
			).
				Where(sql.NotNull(requestexecution.FieldChannelID))

			// Apply time filter
			if applyFilter {
				s.Where(sql.GTE(s.C(requestexecution.FieldCreatedAt), since))
			}

			s.GroupBy(requestexecution.FieldChannelID)
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel execution stats: %w", err)
	}

	if len(results) == 0 {
		return []*ChannelSuccessRate{}, nil
	}

	// Build response with success rate calculation
	var response []*ChannelSuccessRate

	for _, result := range results {
		totalCount := result.SuccessCount + result.FailedCount

		var successRate float64
		if totalCount > 0 {
			successRate = float64(result.SuccessCount) / float64(totalCount) * 100
		}

		response = append(response, &ChannelSuccessRate{
			ChannelID:       objects.GUID{Type: "Channel", ID: result.ChannelID},
			ChannelName:     "",
			ChannelType:     "",
			ChannelDisabled: false,
			SuccessCount:    result.SuccessCount,
			FailedCount:     result.FailedCount,
			TotalCount:      totalCount,
			SuccessRate:     successRate,
		})
	}

	// Order by total count (descending)
	sort.Slice(response, func(i, j int) bool {
		return response[i].TotalCount > response[j].TotalCount
	})

	// Apply limit
	if limitCount > 0 && len(response) > limitCount {
		response = response[:limitCount]
	}

	// Get channel details for the top channels (including soft-deleted channels)
	channelIDs := lo.Map(response, func(item *ChannelSuccessRate, _ int) int {
		return item.ChannelID.ID
	})

	ctx = schematype.SkipSoftDelete(ctx)

	channels, err := r.client.Channel.Query().
		Where(channel.IDIn(channelIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel details: %w", err)
	}

	channelMap := lo.SliceToMap(channels, func(ch *ent.Channel) (int, *ent.Channel) {
		return ch.ID, ch
	})

	// Fill in channel details
	for _, item := range response {
		if ch, exists := channelMap[item.ChannelID.ID]; exists {
			item.ChannelName = ch.Name
			item.ChannelType = string(ch.Type)
			item.ChannelDisabled = ch.Status != "enabled"
		}
	}

	return response, nil
}

// fastestChannels returns the fastest channels by throughput (tokens per second) based on completed request executions.
// Groups by channel_id and calculates throughput from usage_log.completion_tokens and request_execution.metrics_latency_ms.
func (r *queryResolver) fastestChannels(ctx context.Context, input FastestChannelsInput) ([]*FastestChannel, error) {
	// Validate and set default limit
	if input.Limit == nil || *input.Limit <= 0 {
		input.Limit = new(int)
		*input.Limit = 5
	}
	if *input.Limit > 100 {
		*input.Limit = 100
	}

	// Parse time window using calendar periods (like Token Statistics)
	loc := r.systemService.TimeLocation(ctx)
	period := xtime.GetCalendarPeriods(loc)

	var since time.Time
	switch input.TimeWindow {
	case "day":
		since = period.Today.Start
	case "week":
		since = period.ThisWeek.Start
	case "month":
		since = period.ThisMonth.Start
	default:
		since = period.Today.Start // Default to day
	}

	// Query structure for aggregation

	type channelStats struct {
		ChannelID    int     `json:"channel_id"`
		ChannelName  string  `json:"channel_name"`
		ChannelType  string  `json:"channel_type"`
		TokensCount  int64   `json:"tokens_count"`
		LatencyMs    int64   `json:"latency_ms"`
		RequestCount int64   `json:"request_count"`
		Throughput   float64 `json:"throughput"`
	}
	var results []channelStats
	dbDriver := r.client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	// Detect dialect to use appropriate placeholder syntax
	// PostgreSQL uses $1, $2, etc. while SQLite uses ? placeholders
	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	// PostgreSQL uses DISTINCT ON for this ranking; other dialects keep
	// ROW_NUMBER, which avoids the correlated MAX(id) scan on SQLite.
	queryMode := qb.ThroughputModeRowNumber
	if dialectName == dialect.Postgres {
		queryMode = qb.ThroughputModeDistinctOn
	}

	// Build query using shared helper function
	// Fetch more items than needed to allow confidence-based filtering
	sqlLimit := max(*input.Limit*4, 20)
	query := qb.BuildThroughputQuery(
		useDollarPlaceholders,
		qb.ThroughputQueryByChannel,
		sqlLimit,
		queryMode,
	)

	// Use UTC for the time parameter to match the timezone of the created_at column.
	// This assumes created_at is stored in UTC, which is consistent with the application's timezone handling.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	// MaxID mode requires the timestamp twice (outer WHERE and subquery WHERE)
	// ROW_NUMBER mode only needs it once (in the CTE)
	var queryArgs []any
	if queryMode == qb.ThroughputModeMaxID {
		queryArgs = []any{since.UTC(), since.UTC()}
	} else {
		queryArgs = []any{since.UTC()}
	}

	rows, err := sqlDB.DB().QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fastest channels: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var stat channelStats
		if err := rows.Scan(
			&stat.ChannelID,
			&stat.ChannelName,
			&stat.ChannelType,
			&stat.TokensCount,
			&stat.LatencyMs,
			&stat.RequestCount,
			&stat.Throughput,
		); err != nil {
			return nil, fmt.Errorf("failed to scan channel stats: %w", err)
		}
		results = append(results, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(results) == 0 {
		return []*FastestChannel{}, nil
	}

	resultsToShow := calculateConfidenceAndSort(results,
		func(item channelStats) int64 { return item.RequestCount },
		func(item channelStats) float64 { return item.Throughput },
		*input.Limit,
	)

	// Build response with confidence levels
	return lo.Map(resultsToShow, func(item scoredItem[channelStats], _ int) *FastestChannel {
		return &FastestChannel{
			ChannelID:       objects.GUID{Type: "Channel", ID: item.stats.ChannelID},
			ChannelName:     item.stats.ChannelName,
			ChannelType:     item.stats.ChannelType,
			Throughput:      item.stats.Throughput,
			TokensCount:     safeIntFromInt64(item.stats.TokensCount),
			LatencyMs:       safeIntFromInt64(item.stats.LatencyMs),
			RequestCount:    safeIntFromInt64(item.stats.RequestCount),
			ConfidenceLevel: item.confidence,
		}
	}), nil
}

// fastestModels returns the fastest models by throughput (tokens per second) based on completed request executions.
// Groups by request.model_id (AxonHub model) and calculates throughput from usage_log.completion_tokens and request_execution.metrics_latency_ms.
func (r *queryResolver) fastestModels(ctx context.Context, input FastestChannelsInput) ([]*FastestModel, error) {
	// Validate and set default limit
	if input.Limit == nil || *input.Limit <= 0 {
		input.Limit = new(int)
		*input.Limit = 5
	}
	if *input.Limit > 100 {
		*input.Limit = 100
	}

	// Parse time window using calendar periods (like Token Statistics)
	loc := r.systemService.TimeLocation(ctx)
	period := xtime.GetCalendarPeriods(loc)

	var since time.Time
	switch input.TimeWindow {
	case "day":
		since = period.Today.Start
	case "week":
		since = period.ThisWeek.Start
	case "month":
		since = period.ThisMonth.Start
	default:
		since = period.Today.Start // Default to day
	}

	// Query structure for aggregation

	type modelStats struct {
		ModelID      string  `json:"model_id"`
		ModelName    string  `json:"model_name"`
		TokensCount  int64   `json:"tokens_count"`
		LatencyMs    int64   `json:"latency_ms"`
		RequestCount int64   `json:"request_count"`
		Throughput   float64 `json:"throughput"`
	}
	var results []modelStats
	dbDriver := r.client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	// Detect dialect to use appropriate placeholder syntax
	// PostgreSQL uses $1, $2, etc. while SQLite uses ? placeholders
	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	// PostgreSQL uses DISTINCT ON for this ranking; other dialects keep
	// ROW_NUMBER, which avoids the correlated MAX(id) scan on SQLite.
	queryMode := qb.ThroughputModeRowNumber
	if dialectName == dialect.Postgres {
		queryMode = qb.ThroughputModeDistinctOn
	}

	// Build query with dialect-aware timestamp placeholder
	// Fetch more items than needed to allow confidence-based filtering
	sqlLimit := max(*input.Limit*4, 20)

	query := qb.BuildThroughputQuery(
		useDollarPlaceholders,
		qb.ThroughputQueryByModel,
		sqlLimit,
		queryMode,
	)

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	// MaxID mode requires the timestamp twice (outer WHERE and subquery WHERE)
	// ROW_NUMBER mode only needs it once (in the CTE)
	var queryArgs []any
	if queryMode == qb.ThroughputModeMaxID {
		queryArgs = []any{since.UTC(), since.UTC()}
	} else {
		queryArgs = []any{since.UTC()}
	}

	rows, err := sqlDB.DB().QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fastest models: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var stat modelStats
		if err := rows.Scan(
			&stat.ModelID,
			&stat.ModelName,
			&stat.TokensCount,
			&stat.LatencyMs,
			&stat.RequestCount,
			&stat.Throughput,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model stats: %w", err)
		}
		results = append(results, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(results) == 0 {
		return []*FastestModel{}, nil
	}

	resultsToShow := calculateConfidenceAndSort(results,
		func(item modelStats) int64 { return item.RequestCount },
		func(item modelStats) float64 { return item.Throughput },
		*input.Limit,
	)

	// Build response with confidence levels
	return lo.Map(resultsToShow, func(item scoredItem[modelStats], _ int) *FastestModel {
		return &FastestModel{
			ModelID:         item.stats.ModelID,
			ModelName:       item.stats.ModelName,
			Throughput:      item.stats.Throughput,
			TokensCount:     safeIntFromInt64(item.stats.TokensCount),
			LatencyMs:       safeIntFromInt64(item.stats.LatencyMs),
			RequestCount:    safeIntFromInt64(item.stats.RequestCount),
			ConfidenceLevel: item.confidence,
		}
	}), nil
}
