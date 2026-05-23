package gql

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/requestexecution"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/ent/usagelog"
	"github.com/ldm2060/axonhub/internal/ent/userproject"
	"github.com/ldm2060/axonhub/internal/pkg/xtime"
	"github.com/samber/lo"
)

func (r *queryResolver) getUserProjectIDs(ctx context.Context) ([]int, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	projectIDs, err := r.client.UserProject.Query().
		Where(userproject.UserIDEQ(user.ID)).
		QueryProject().
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user projects: %w", err)
	}
	if len(projectIDs) == 0 {
		privateProjectID, err := r.userService.EnsurePrivateProject(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve private project: %w", err)
		}
		projectIDs = []int{privateProjectID}
	}
	return projectIDs, nil
}

func (r *queryResolver) buildMyChannelPerformanceStatsFromExecutions(ctx context.Context, projectIDs []int, startDateLocal time.Time, offsetSeconds int, daysCount int) ([]*ChannelPerformanceStat, error) {
	startDateUTC := startDateLocal.UTC()
	loc := r.systemService.TimeLocation(ctx)

	type dailyChannelStat struct {
		Date         string   `json:"date"`
		ChannelID    int      `json:"channel_id"`
		TokensCount  int64    `json:"tokens_count"`
		LatencyMs    int64    `json:"latency_ms"`
		FirstTokenMs *float64 `json:"first_token_ms"`
		RequestCount int64    `json:"request_count"`
		Throughput   *float64 `json:"throughput"`
	}

	var results []dailyChannelStat

	err := r.client.UsageLog.Query().
		Where(usagelog.ProjectIDIn(projectIDs...), usagelog.CreatedAtGTE(startDateUTC)).
		Modify(func(s *sql.Selector) {
			reTable := sql.Table(requestexecution.Table)
			s.Join(reTable).On(s.C(usagelog.FieldRequestID), reTable.C(requestexecution.FieldRequestID))
			s.Where(sql.EQ(reTable.C(requestexecution.FieldStatus), "completed"))
			s.Where(sql.NotNull(reTable.C(requestexecution.FieldChannelID)))

			var dateExpr string
			createdAtCol := reTable.C(requestexecution.FieldCreatedAt)
			switch s.Dialect() {
			case dialect.SQLite:
				dateExpr = fmt.Sprintf("strftime('%%Y-%%m-%%d', datetime(substr(%s, 1, 19), '%+d seconds'))", createdAtCol, offsetSeconds)
			case dialect.MySQL:
				offsetStr := xtime.FormatUTCOffset(offsetSeconds)
				dateExpr = fmt.Sprintf("DATE_FORMAT(CONVERT_TZ(%s, '+00:00', '%s'), '%%Y-%%m-%%d')", createdAtCol, offsetStr)
			case dialect.Postgres:
				dateExpr = fmt.Sprintf("to_char(%s AT TIME ZONE '%s', 'YYYY-MM-DD')", createdAtCol, loc.String())
			default:
				dateExpr = fmt.Sprintf("DATE(%s)", createdAtCol)
			}

			s.Select(
				sql.As(dateExpr, "date"),
				sql.As(reTable.C(requestexecution.FieldChannelID), "channel_id"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "tokens_count"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", reTable.C(requestexecution.FieldMetricsLatencyMs)), "latency_ms"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", reTable.C(requestexecution.FieldMetricsFirstTokenLatencyMs)), "first_token_ms"),
				sql.As(sql.Count("*"), "request_count"),
				sql.As(fmt.Sprintf("CASE WHEN COALESCE(SUM(%s), 0) > 0 AND COALESCE(SUM(%s), 0) > 0 THEN CAST(COALESCE(SUM(%s), 0) AS FLOAT) / (CAST(COALESCE(SUM(%s), 0) AS FLOAT) / 1000.0) ELSE 0 END",
					s.C(usagelog.FieldCompletionTokens),
					reTable.C(requestexecution.FieldMetricsLatencyMs),
					s.C(usagelog.FieldCompletionTokens),
					reTable.C(requestexecution.FieldMetricsLatencyMs)), "throughput"),
			)
			s.GroupBy(dateExpr, reTable.C(requestexecution.FieldChannelID))
			s.OrderBy("date")
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get my channel performance stats: %w", err)
	}

	type channelStatsBucket struct {
		totalRequests int64
		results       []*ChannelPerformanceStat
	}
	channelStats := make(map[int]*channelStatsBucket)
	for _, raw := range results {
		var ttftMs *float64
		if raw.FirstTokenMs != nil && *raw.FirstTokenMs > 0 {
			ttftMs = raw.FirstTokenMs
		}
		var throughput *float64
		if raw.Throughput != nil && *raw.Throughput > 0 {
			throughput = raw.Throughput
		}
		stat := &ChannelPerformanceStat{
			Date:         raw.Date,
			ChannelID:    fmt.Sprintf("%d", raw.ChannelID),
			ChannelName:  "",
			Throughput:   throughput,
			TtftMs:       ttftMs,
			RequestCount: safeIntFromInt64(raw.RequestCount),
		}
		if channelStats[raw.ChannelID] == nil {
			channelStats[raw.ChannelID] = &channelStatsBucket{}
		}
		channelStats[raw.ChannelID].totalRequests += raw.RequestCount
		channelStats[raw.ChannelID].results = append(channelStats[raw.ChannelID].results, stat)
	}

	type channelInfo struct {
		channelID    int
		requestCount int64
	}
	channelInfos := lo.MapToSlice(channelStats, func(channelID int, stats *channelStatsBucket) channelInfo {
		return channelInfo{channelID: channelID, requestCount: stats.totalRequests}
	})
	topChannels := calculateConfidenceAndSort(channelInfos, func(c channelInfo) int64 { return c.requestCount }, func(c channelInfo) float64 { return float64(c.requestCount) }, topPerformersLimit)

	channelIDs := lo.Map(topChannels, func(item scoredItem[channelInfo], _ int) int { return item.stats.channelID })
	ctx = schematype.SkipSoftDelete(ctx)
	channels, err := r.client.Channel.Query().Where(channel.IDIn(channelIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel names: %w", err)
	}
	channelNameMap := lo.SliceToMap(channels, func(ch *ent.Channel) (int, string) { return ch.ID, ch.Name })

	var allStats []*ChannelPerformanceStat
	for _, item := range topChannels {
		name := channelNameMap[item.stats.channelID]
		for _, stat := range channelStats[item.stats.channelID].results {
			stat.ChannelName = name
			allStats = append(allStats, stat)
		}
	}

	return allStats, nil
}
