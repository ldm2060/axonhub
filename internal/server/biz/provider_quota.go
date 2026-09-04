package biz

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/providerquotastatus"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
	"github.com/ldm2060/axonhub/internal/server/scheduler"
	"github.com/ldm2060/axonhub/llm/httpclient"
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
	HttpClient    *httpclient.HttpClient
}

type ProviderQuotaService struct {
	*AbstractService

	SystemService *SystemService
	usageMonitor  *UsageMonitorService
	httpClient    *httpclient.HttpClient

	mu                  sync.Mutex
	quotaCache          sync.Map
	bindingsActiveCache sync.Map
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
	svc := &ProviderQuotaService{
		AbstractService: &AbstractService{db: params.Ent},
		SystemService:   params.SystemService,
		usageMonitor:    params.UsageMonitor,
		httpClient:      params.HttpClient,
	}

	go svc.loadQuotaCache(context.Background())

	// Wire the quota cache callback so UsageMonitorService updates
	// the routing cache in real-time after each poll
	params.UsageMonitor.SetQuotaCacheCallback(svc.OnUsageMonitorPollComplete)
	// Wire the binding-presence callback so UsageMonitorService reports whether
	// each channel is on the binding path (has effective quota_monitor_bindings)
	// or the legacy fallback path. The orchestrator uses this to decide whether
	// the independent quotaStatus exhaustion filter applies.
	params.UsageMonitor.SetBindingsActiveCallback(svc.OnChannelBindingsActive)

	return svc
}

// OnChannelBindingsActive is called by UsageMonitorService after
// evaluateAndUpdateChannelQuotaReady runs, reporting whether the channel has
// effective quota_monitor_bindings rows (binding path) or relies on the legacy
// auto-disable fallback (no bindings).
func (svc *ProviderQuotaService) OnChannelBindingsActive(channelID int, hasBindings bool) {
	if hasBindings {
		svc.bindingsActiveCache.Store(channelID, struct{}{})
	} else {
		svc.bindingsActiveCache.Delete(channelID)
	}
}

// HasActiveBindings reports whether the channel currently has effective quota
// monitor bindings (the binding path). Binding-first channels are enforced
// solely via QuotaBindingReady; the independent quotaStatus exhaustion filter
// is the fallback for channels without bindings.
func (svc *ProviderQuotaService) HasActiveBindings(_ context.Context, channelID int) bool {
	_, ok := svc.bindingsActiveCache.Load(channelID)
	return ok
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

// ListResets returns the reset capability and available resets for a channel.
// Providers that do not implement Resetter report Supported=false without an
// error so callers can treat resetting as an optional capability.
func (svc *ProviderQuotaService) ListResets(ctx context.Context, channelID int) (provider_quota.ResetList, error) {
	ch, err := svc.db.Channel.Query().Where(channel.IDEQ(channelID)).Only(ctx)
	if err != nil {
		return provider_quota.ResetList{}, fmt.Errorf("failed to load channel: %w", err)
	}

	providerType := getProviderType(ch)
	resetter, supported := svc.resetCheckerForProvider(providerType)
	if !supported || resetter == nil {
		return provider_quota.ResetList{Supported: false, Resets: nil, Error: ""}, nil
	}

	if enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType); err != nil {
		return provider_quota.ResetList{}, fmt.Errorf("failed to read provider quota collection settings: %w", err)
	} else if !enabled {
		return provider_quota.ResetList{}, fmt.Errorf("provider quota collection is disabled for %s", providerType)
	}

	if !hasCredentialsForProvider(ch) {
		return provider_quota.ResetList{}, fmt.Errorf("channel has no credentials")
	}

	resets, err := resetter.ListResets(ctx, ch)
	resets.Supported = true
	if err != nil {
		return resets, fmt.Errorf("failed to list %s quota resets: %w", providerType, err)
	}

	return resets, nil
}

// ResetChannelQuotaNow attempts to redeem a provider-managed reset for a channel.
func (svc *ProviderQuotaService) ResetChannelQuotaNow(ctx context.Context, channelID int) error {
	ch, err := svc.db.Channel.Query().Where(channel.IDEQ(channelID)).Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to load channel: %w", err)
	}

	providerType := getProviderType(ch)
	resetter, supported := svc.resetCheckerForProvider(providerType)
	if !supported || resetter == nil {
		return fmt.Errorf("%w for provider %q", provider_quota.ErrResetUnsupported, providerType)
	}

	if enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType); err != nil {
		return fmt.Errorf("failed to read provider quota collection settings: %w", err)
	} else if !enabled {
		return fmt.Errorf("provider quota collection is disabled for %s", providerType)
	}

	if !hasCredentialsForProvider(ch) {
		return fmt.Errorf("channel has no credentials")
	}

	if err := resetter.Reset(ctx, ch); err != nil {
		return fmt.Errorf("failed to reset %s quota: %w", providerType, err)
	}

	// Refresh the quota status immediately so the UI reflects the reset. Our
	// architecture delegates polling to UsageMonitorService, so trigger a full
	// poll rather than a single-channel check.
	svc.mu.Lock()
	svc.usageMonitor.RunPollAll(ctx)
	svc.mu.Unlock()

	return nil
}

// resetCheckerForProvider returns a Resetter for the given provider type by
// instantiating the provider's quota checker directly. Unlike upstream's
// svc.checkers map, this does not retain a long-lived checker instance — the
// checker is cheap to construct and only the Reset/ListResets capability is
// needed here; regular polling is handled by UsageMonitorService.
func (svc *ProviderQuotaService) resetCheckerForProvider(providerType string) (provider_quota.Resetter, bool) {
	switch providerType {
	case "codex":
		return provider_quota.NewCodexQuotaChecker(svc.httpClient), true
	default:
		return nil, false
	}
}

func getProviderType(ch *ent.Channel) string {
	switch ch.Type { //nolint:exhaustive
	case channel.TypeClaudecode:
		return "claudecode"
	case channel.TypeCodex:
		return "codex"
	case channel.TypeXaiSubscription:
		return "xai_subscription"
	case channel.TypeGithubCopilot:
		return "github_copilot"
	case channel.TypeNanogpt, channel.TypeNanogptResponses:
		return "nanogpt"
	case channel.TypeCline:
		return "cline"
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		return provider_quota.DetectProviderFromURL(ch.BaseURL)
	case channel.TypeOpencodeGo, channel.TypeOpencodeGoAnthropic:
		return "opencode_go"
	case channel.TypeMoonshotCoding:
		return "kimi_code"
	case channel.TypeMinimax, channel.TypeMinimaxAnthropic:
		return "minimax"
	case channel.TypeZhipu, channel.TypeZhipuAnthropic:
		return "zhipu"
	default:
		return ""
	}
}

func hasCredentialsForProvider(ch *ent.Channel) bool {
	if ch.Type == channel.TypeOpenai || ch.Type == channel.TypeOpenaiResponses {
		providerType := provider_quota.DetectProviderFromURL(ch.BaseURL)
		if _, ok := provider_quota.URLDetectedProviders()[providerType]; ok {
			return strings.TrimSpace(ch.Credentials.APIKey) != "" || len(ch.Credentials.APIKeys) > 0
		}
	}

	if ch.Type == channel.TypeCodex || ch.Type == channel.TypeClaudecode || ch.Type == channel.TypeXaiSubscription {
		return ch.Credentials.OAuth != nil || isOAuthJSON(ch.Credentials.APIKey)
	}

	if ch.Type == channel.TypeCline {
		if strings.TrimSpace(ch.Credentials.APIKey) != "" {
			return true
		}
		for _, apiKey := range ch.Credentials.APIKeys {
			if strings.TrimSpace(apiKey) != "" {
				return true
			}
		}
		return false
	}

	return ch.Credentials.OAuth != nil || isOAuthJSON(ch.Credentials.APIKey) ||
		strings.TrimSpace(ch.Credentials.APIKey) != "" || len(ch.Credentials.APIKeys) > 0
}

// extractLimitsFromQuotaLimits extracts QuotaLimitStatus from the []map[string]any
// stored in the usage_monitor_channel.quota_limits field.
func extractLimitsFromQuotaLimits(data []map[string]any) []provider_quota.QuotaLimitStatus {
	if len(data) == 0 {
		return nil
	}

	limits := make([]provider_quota.QuotaLimitStatus, 0, len(data))
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
