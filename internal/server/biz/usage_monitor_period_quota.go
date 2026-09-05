package biz

import (
	"context"
	"time"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// fillMonitorPeriodQuotaEstimates prices each derived limit period of a usage
// monitor: it sums what the monitor's provider channels cost according to
// AxonHub usage logs since the period started, and derives the period's total
// money quota from the usage ratio the monitor reported. This mirrors
// ProviderQuotaService.fillPeriodQuotas so monitor-derived quota data carries
// the same periodCost/periodQuota estimates the built-in checkers produce.
//
// Cost attribution works for template monitors, not just channel-bound ones:
// the bound channel is priced directly when present, otherwise every enabled
// channel of the monitor's provider type contributes (a template monitor
// describes that provider's account-level quota, which is the pool those
// channels draw from). Monitors of providers with no AxonHub channels have no
// usage to aggregate and keep their limits without an estimate. Any
// aggregation failure is logged and skipped so the poll still succeeds.
// limitMaps is mutated in place and must be index-aligned with limits (the
// caller builds them from the same slice).
func (svc *UsageMonitorService) fillMonitorPeriodQuotaEstimates(
	ctx context.Context,
	monitor *ent.UsageMonitorChannel,
	limits []provider_quota.QuotaLimitStatus,
	limitMaps []map[string]any,
	now time.Time,
) {
	if len(limits) == 0 {
		return
	}

	channelIDs := svc.monitorCostChannelIDs(ctx, monitor)
	if len(channelIDs) == 0 {
		return
	}

	// Providers repeat the same period start across limits (e.g. Cline reports
	// three windows), so the cost of each distinct period is only queried once.
	costs := make(map[time.Time]float64)

	for i, limit := range limits {
		start := limit.PeriodStart
		if start == nil || !start.Before(now) {
			continue
		}

		cost, ok := costs[*start]
		if !ok {
			for _, id := range channelIDs {
				aggregated, err := aggregateChannelCostSince(ctx, svc.db, id, *start, now)
				if err != nil {
					log.Warn(ctx, "Failed to aggregate channel cost for monitor quota period",
						log.Int("monitor_id", monitor.ID),
						log.Int("channel_id", id),
						log.Time("period_start", *start),
						log.Cause(err))
					continue
				}
				cost += aggregated
			}
			costs[*start] = cost
		}

		if quota, okEstimate := provider_quota.EstimatePeriodQuota(cost, limit.UsageRatio); okEstimate {
			limitMaps[i]["periodCost"] = cost
			limitMaps[i]["periodQuota"] = quota
		}
	}
}

// monitorCostChannelIDs returns the AxonHub channels whose usage logs price
// the monitor's quota windows: the bound channel when one exists, otherwise
// every enabled channel that maps onto the monitor's provider type.
func (svc *UsageMonitorService) monitorCostChannelIDs(ctx context.Context, monitor *ent.UsageMonitorChannel) []int {
	if monitor.ChannelID != nil {
		return []int{*monitor.ChannelID}
	}

	providerType := string(monitor.ProviderType)
	if providerType == "" {
		return nil
	}

	channels, err := svc.db.Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		Select(channel.FieldID, channel.FieldType, channel.FieldBaseURL).
		All(ctx)
	if err != nil {
		log.Warn(ctx, "Failed to query channels for monitor quota cost attribution",
			log.Int("monitor_id", monitor.ID),
			log.String("provider_type", providerType),
			log.Cause(err))
		return nil
	}

	ids := make([]int, 0, len(channels))
	for _, ch := range channels {
		if channelProviderType(ch) == providerType {
			ids = append(ids, ch.ID)
		}
	}
	return ids
}
