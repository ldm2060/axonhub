package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/providerquotastatus"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
)

type QuotaChannelStatus struct {
	ProviderType string
	Status       providerquotastatus.Status
	Ready        bool
	Limits       []provider_quota.QuotaLimitStatus
}

// EffectiveStatus returns the effective quota status for the given limit type.
//
// If the channel-level status is Exhausted, it short-circuits regardless of
// per-limit data — a channel marked exhausted at the top level is treated as
// fully unavailable. This means if a future provider sets channel-level
// "exhausted" for a single limit type (e.g., images), token-limit queries
// would also return "exhausted" even if tokens remain.
func (s *QuotaChannelStatus) EffectiveStatus(limitType provider_quota.QuotaLimitType) (providerquotastatus.Status, bool) {
	if s.Status == providerquotastatus.StatusExhausted {
		return providerquotastatus.StatusExhausted, false
	}

	if len(s.Limits) == 0 {
		return s.Status, s.Ready
	}

	var worstStatus providerquotastatus.Status
	worstReady := true
	found := false

	for _, l := range s.Limits {
		if l.Type != limitType {
			continue
		}

		ls := providerquotastatus.Status(l.Status)
		if !found {
			worstStatus = ls
			worstReady = l.Ready
			found = true
			continue
		}

		if quotaStatusRank(ls) > quotaStatusRank(worstStatus) {
			worstStatus = ls
			worstReady = l.Ready
		} else if quotaStatusRank(ls) == quotaStatusRank(worstStatus) {
			worstReady = worstReady && l.Ready
		}
	}

	if !found {
		// No matching limit type: return Unknown with ready=true so the channel
		// is not filtered out. This differs from a per-limit "unknown" status
		// (where ready=false) because missing data should not block routing.
		return providerquotastatus.StatusUnknown, true
	}

	return worstStatus, worstReady
}

func quotaStatusRank(s providerquotastatus.Status) int {
	switch s {
	case providerquotastatus.StatusAvailable:
		return 0
	case providerquotastatus.StatusWarning:
		return 1
	case providerquotastatus.StatusExhausted:
		return 2
	case providerquotastatus.StatusUnknown:
		return -1
	default:
		return -1
	}
}

type ProviderQuotaServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
	UsageMonitor  *UsageMonitorService
}

type ProviderQuotaService struct {
	*AbstractService

	SystemService *SystemService
	usageMonitor  *UsageMonitorService

	mu         sync.Mutex
	quotaCache sync.Map
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
	svc := &ProviderQuotaService{
		AbstractService: &AbstractService{db: params.Ent},
		SystemService:   params.SystemService,
		usageMonitor:    params.UsageMonitor,
	}

	go svc.loadQuotaCache(context.Background())

	// Wire the quota cache callback so UsageMonitorService updates
	// the routing cache in real-time after each poll
	params.UsageMonitor.SetQuotaCacheCallback(svc.OnUsageMonitorPollComplete)

	return svc
}

// OnUsageMonitorPollComplete is called by UsageMonitorService after each successful poll
// to update the in-memory quota cache for orchestrator routing.
func (svc *ProviderQuotaService) OnUsageMonitorPollComplete(channelID int, providerType string, quotaStatus string, ready bool, limits []map[string]any) {
	var quotaLimits []provider_quota.QuotaLimitStatus
	for _, m := range limits {
		ls := provider_quota.QuotaLimitStatus{}
		if t, ok := m["type"].(string); ok {
			ls.Type = provider_quota.QuotaLimitType(t)
		}
		if s, ok := m["status"].(string); ok {
			ls.Status = s
		}
		if u, ok := m["usageRatio"].(float64); ok {
			ls.UsageRatio = u
		}
		if r, ok := m["ready"].(bool); ok {
			ls.Ready = r
		}
		if ts, ok := m["nextResetAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				ls.NextResetAt = &t
			}
		}
		quotaLimits = append(quotaLimits, ls)
	}

	svc.updateQuotaCache(channelID, providerType, providerquotastatus.Status(quotaStatus), ready, quotaLimits)
}

func (svc *ProviderQuotaService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	// No longer needed — UsageMonitorService handles polling
	return nil
}

func (svc *ProviderQuotaService) loadQuotaCache(ctx context.Context) {
	// Wait for usage monitor cache to be ready
	time.Sleep(2 * time.Second)

	channels := svc.usageMonitor.ListChannelsFromCache()
	for _, ch := range channels {
		if ch.ChannelID == nil || ch.QuotaStatus == "" {
			continue
		}
		limits := extractLimitsFromQuotaLimits(ch.QuotaLimits)
		svc.quotaCache.Store(*ch.ChannelID, &QuotaChannelStatus{
			ProviderType: string(ch.ProviderType),
			Status:       providerquotastatus.Status(ch.QuotaStatus),
			Ready:        ch.QuotaReady != nil && *ch.QuotaReady,
			Limits:       limits,
		})
	}

	log.Debug(ctx, "Loaded quota cache from usage monitor channels", log.Int("channels", len(channels)))
}

func (svc *ProviderQuotaService) GetQuotaStatus(ctx context.Context, channelID int) *QuotaChannelStatus {
	val, ok := svc.quotaCache.Load(channelID)
	if !ok {
		return nil
	}

	status, ok := val.(*QuotaChannelStatus)
	if !ok {
		return nil
	}
	if status.ProviderType != "" {
		settings := svc.SystemService.ProviderQuotaCollectionSettingsOrDefault(ctx)
		if !settings.Enabled || !settings.Providers[status.ProviderType] {
			return nil
		}
	}

	return status
}

func (svc *ProviderQuotaService) updateQuotaCache(channelID int, providerType string, status providerquotastatus.Status, ready bool, limits []provider_quota.QuotaLimitStatus) {
	svc.quotaCache.Store(channelID, &QuotaChannelStatus{
		ProviderType: providerType,
		Status:       status,
		Ready:        ready,
		Limits:       limits,
	})
}

// InvalidateChannelQuota removes a channel's persisted and cached quota state.
// Channel provider identity changes invalidate the previous provider's quota
// result, so serialize this with quota checks before removing the record.
func (svc *ProviderQuotaService) InvalidateChannelQuota(ctx context.Context, channelID int) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	defer svc.quotaCache.Delete(channelID)

	// Drop any persisted legacy provider_quota_status row (pre-usage-monitor
	// architecture) so stale state does not survive a re-read.
	if _, err := svc.db.ProviderQuotaStatus.Delete().
		Where(providerquotastatus.ChannelIDEQ(channelID)).
		Exec(schematype.SkipSoftDelete(ctx)); err != nil {
		return fmt.Errorf("failed to invalidate provider quota status: %w", err)
	}

	// Clear the usage-monitor quota snapshot for the builtin monitor bound to
	// this channel, if present.
	if _, err := svc.db.UsageMonitorChannel.Update().
		Where(usagemonitorchannel.ChannelIDEQ(channelID)).
		ClearQuotaStatus().
		ClearQuotaReady().
		ClearQuotaLimits().
		Save(ctx); err != nil {
		return fmt.Errorf("failed to clear usage monitor quota snapshot: %w", err)
	}

	return nil
}

// ManualCheck forces an immediate quota check for all relevant channels.
func (svc *ProviderQuotaService) ManualCheck(ctx context.Context) {
	svc.usageMonitor.RunPollAll(ctx)
}

// extractLimitsFromQuotaLimits extracts QuotaLimitStatus from the []map[string]any
// stored in the usage_monitor_channel.quota_limits field.
func extractLimitsFromQuotaLimits(data []map[string]any) []provider_quota.QuotaLimitStatus {
	if len(data) == 0 {
		return nil
	}

	var limits []provider_quota.QuotaLimitStatus
	for _, m := range data {
		ls := provider_quota.QuotaLimitStatus{}

		if t, ok := m["type"].(string); ok {
			ls.Type = provider_quota.QuotaLimitType(t)
		}
		if s, ok := m["status"].(string); ok {
			ls.Status = s
		}
		if u, ok := m["usageRatio"].(float64); ok {
			ls.UsageRatio = u
		}
		if r, ok := m["ready"].(bool); ok {
			ls.Ready = r
		}
		if ts, ok := m["nextResetAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				ls.NextResetAt = &t
			}
		}

		limits = append(limits, ls)
	}

	return limits
}
