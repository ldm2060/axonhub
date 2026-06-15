package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xtime"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// startPerformanceProcess starts the background goroutine to flush metrics to database.
func (svc *ChannelService) startPerformanceProcess() {
	ctx := authz.WithSystemBypass(context.Background(), "channel-record-performance")
	for perf := range svc.perfCh {
		svc.RecordPerformance(ctx, perf)
	}
}

func (svc *ChannelService) runSyncChannelModelsPeriodically(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "channel-run-model-sync")
	setting := svc.SystemService.ChannelSettingOrDefault(ctx)
	if !svc.shouldRunModelSync(xtime.UTCNow(), setting.AutoSync.Frequency) {
		return
	}

	svc.syncChannelModels(ctx)
}

func (svc *ChannelService) shouldRunModelSync(now time.Time, frequency AutoSyncFrequency) bool {
	intervalMinutes := getIntervalMinutesFromAutoSyncFrequency(frequency)
	alignedTime := now.Truncate(time.Duration(intervalMinutes) * time.Minute)

	svc.modelSyncMu.Lock()
	defer svc.modelSyncMu.Unlock()

	if !svc.lastModelSyncExecutionTime.IsZero() && svc.lastModelSyncExecutionTime.Equal(alignedTime) {
		return false
	}

	svc.lastModelSyncExecutionTime = alignedTime
	return true
}

func getIntervalMinutesFromAutoSyncFrequency(frequency AutoSyncFrequency) int {
	switch frequency {
	case AutoSyncFrequencyOneHour:
		return 60
	case AutoSyncFrequencySixHours:
		return 360
	case AutoSyncFrequencyOneDay:
		return 1440
	default:
		return 60
	}
}

func (svc *ChannelService) onCacheRefreshed(ctx context.Context, current []*Channel, lastUpdate time.Time) ([]*Channel, time.Time, bool, error) {
	ctx = authz.WithSystemBypass(ctx, "channel-refresh-cache")
	return svc.reloadEnabledChannels(ctx, current, lastUpdate)
}

func (svc *ChannelService) onTokenRefreshed(ch *ent.Channel) func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
	return func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
		ctx = authz.WithSystemBypass(ctx, "channel-refresh-cache")
		return svc.refreshOAuthToken(ctx, ch, refreshed)
	}
}

func (svc *ChannelService) initChannelPerformances(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "int-channel-load-performances")
	if err := svc.loadChannelPerformances(ctx); err != nil {
		log.Warn(ctx, "failed to load channel performances", log.Cause(err))
	}
}

func (svc *ChannelService) ReloadEnabledChannelsCache(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, "channel-reload-enabled-channels-cache")
	if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
		return fmt.Errorf("failed to reload enabled channels cache: %w", err)
	}

	return nil
}

// onQuotaBindingChannelsReload is registered with UsageMonitorService and
// invoked after a channel's quota_binding_ready state changes. It refreshes
// the local enabled-channels cache immediately and notifies other instances,
// so the orchestrator stops/starts routing to the channel without waiting for
// the cache's 1-minute refresh tick (mirrors markChannelUnavailable).
func (svc *ChannelService) onQuotaBindingChannelsReload(ctx context.Context, channelID int) {
	if err := svc.ReloadEnabledChannelsCache(ctx); err != nil {
		log.Warn(ctx, "failed to reload enabled channels cache after quota binding change",
			log.Int("channel_id", channelID),
			log.Cause(err))
		return
	}
	svc.asyncReloadChannels()
}
