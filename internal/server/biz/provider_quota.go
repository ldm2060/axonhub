package biz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"

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

const maxConcurrentQuotaChecks = 8

// Quota check failures back off exponentially so a persistently failing channel
// (expired credentials, or a scraped provider dashboard whose markup changed) is
// retried at a slow cadence instead of on every check interval. This mirrors the
// model circuit breaker's probe backoff (see model_circuit_breaker.go).
const (
	quotaErrorCodeCheckFailed        = "check_failed"
	quotaErrorCodeMissingCredentials = "missing_credentials"

	// maxQuotaErrorBackoffMultiplier caps the backoff growth at 8x the base
	// interval, matching the circuit breaker's probe backoff cap.
	maxQuotaErrorBackoffMultiplier = 8

	// maxQuotaErrorBackoffSteps is the consecutive-failure count at which the
	// multiplier saturates (1, 2, 4, 8). The persisted error_count is clamped
	// here so it stays bounded for a channel that never recovers.
	maxQuotaErrorBackoffSteps = 4
)

func quotaErrorCode(err error) string {
	if err != nil && err.Error() == "channel has no credentials" {
		return quotaErrorCodeMissingCredentials
	}

	return quotaErrorCodeCheckFailed
}

var providerQuotaChannelTypes = []channel.Type{
	channel.TypeClaudecode,
	channel.TypeCodex,
	channel.TypeAntigravity,
	channel.TypeXaiSubscription,
	channel.TypeGithubCopilot,
	channel.TypeNanogpt,
	channel.TypeNanogptResponses,
	channel.TypeZenmux,
	channel.TypeZenmuxResponses,
	channel.TypeZenmuxAnthropic,
	channel.TypeZenmuxGemini,
	channel.TypeCline,
	channel.TypeOpenai,
	channel.TypeOpenaiResponses,
	channel.TypeOpencodeGo,
	channel.TypeOpencodeGoAnthropic,
	channel.TypeMoonshotCoding,
	channel.TypeMinimax,
	channel.TypeMinimaxAnthropic,
	channel.TypeZhipu,
	channel.TypeZhipuAnthropic,
	channel.TypeCommandcode,
	channel.TypeCommandcodeAnthropic,
}

// quotaErrorBackoff returns the next-check delay after `failures` consecutive
// quota check failures: base, 2x, 4x, ... capped at maxQuotaErrorBackoffMultiplier.
// A successful check clears the counter (saveQuotaStatus overwrites quota_data),
// so the cadence returns to base as soon as the provider recovers.
func quotaErrorBackoff(base time.Duration, failures int) time.Duration {
	multiplier := 1
	for i := 1; i < failures && multiplier < maxQuotaErrorBackoffMultiplier; i++ {
		multiplier *= 2
	}

	if multiplier > maxQuotaErrorBackoffMultiplier {
		multiplier = maxQuotaErrorBackoffMultiplier
	}

	return base * time.Duration(multiplier)
}

// quotaErrorCount reads the persisted consecutive-failure counter from quota_data.
// Values stored in-process are int; values reloaded from the DB are float64.
func quotaErrorCount(data map[string]any) int {
	switch v := data["error_count"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// nextQuotaErrorCount increments the consecutive-failure counter, clamped at
// maxQuotaErrorBackoffSteps so the value persisted in quota_data stays bounded
// once the backoff multiplier has saturated.
func nextQuotaErrorCount(prev int) int {
	if prev+1 > maxQuotaErrorBackoffSteps {
		return maxQuotaErrorBackoffSteps
	}

	return prev + 1
}

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

	Ent                       *ent.Client
	SystemService             *SystemService
	UsageMonitor              *UsageMonitorService
	HttpClient                *httpclient.HttpClient
	CheckInterval             time.Duration `name:"provider_quota_check_interval" optional:"true"`
	WarningCheckIntervalRatio int           `name:"provider_quota_warning_check_interval_ratio" optional:"true"`
}

type ProviderQuotaService struct {
	*AbstractService

	SystemService             *SystemService
	checkInterval             time.Duration
	warningCheckIntervalRatio int
	httpClient                *httpclient.HttpClient

	// Registry of provider-specific quota checkers used by the polling engine.
	checkers map[string]provider_quota.QuotaChecker

	mu                  sync.Mutex
	quotaCache          sync.Map
	bindingsActiveCache sync.Map
}

func NewProviderQuotaService(params ProviderQuotaServiceParams) *ProviderQuotaService {
	svc := &ProviderQuotaService{
		AbstractService:           &AbstractService{db: params.Ent},
		SystemService:             params.SystemService,
		httpClient:                params.HttpClient,
		checkers:                  make(map[string]provider_quota.QuotaChecker),
		checkInterval:             params.CheckInterval,
		warningCheckIntervalRatio: params.WarningCheckIntervalRatio,
	}

	svc.registerProviderQuotaSupport()

	svc.loadQuotaCache(context.Background())

	if params.UsageMonitor != nil {
		// Wire the quota cache callback so UsageMonitorService updates
		// the routing cache in real-time after each poll
		params.UsageMonitor.SetQuotaCacheCallback(svc.OnUsageMonitorPollComplete)
		// Wire the binding-presence callback so UsageMonitorService reports whether
		// each channel is on the binding path (has effective quota_monitor_bindings)
		// or the legacy fallback path. The orchestrator uses this to decide whether
		// the independent quotaStatus exhaustion filter applies.
		params.UsageMonitor.SetBindingsActiveCallback(svc.OnChannelBindingsActive)
	}

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

func (svc *ProviderQuotaService) registerProviderQuotaSupport() {
	svc.registerClaudeCodeSupport()
	svc.registerCodexSupport()
	svc.registerAntigravitySupport()
	svc.registerXAISubscriptionSupport()
	svc.registerGithubCopilotSupport()
	svc.registerNanoGPTSupport()
	svc.registerZenmuxSupport()
	svc.registerClineSupport()
	svc.registerWaferSupport()
	svc.registerSyntheticSupport()
	svc.registerNeuralWattSupport()
	svc.registerApertisSupport()
	svc.registerOpenCodeGoSupport()
	svc.registerKimiCodeSupport()
	svc.registerMinimaxSupport()
	svc.registerZhipuSupport()
	svc.registerCharmHyperSupport()
	svc.registerCommandCodeSupport()
}

func (svc *ProviderQuotaService) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	cronExpr := svc.intervalToCronExpr(svc.getCheckInterval())
	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "provider-quota-check",
		Description: "Check provider quota usage periodically",
		CronExpr:    cronExpr,
		FixRate:     0,
		Timezone:    "UTC",
	}, svc.runQuotaCheckScheduled)
}

func (svc *ProviderQuotaService) registerClaudeCodeSupport() {
	svc.checkers["claudecode"] = provider_quota.NewClaudeCodeQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerCommandCodeSupport() {
	svc.checkers["commandcode"] = provider_quota.NewCommandCodeQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerCodexSupport() {
	svc.checkers["codex"] = provider_quota.NewCodexQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerAntigravitySupport() {
	svc.checkers["antigravity"] = provider_quota.NewAntigravityQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerXAISubscriptionSupport() {
	svc.checkers["xai_subscription"] = provider_quota.NewXAISubscriptionQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerGithubCopilotSupport() {
	svc.checkers["github_copilot"] = provider_quota.NewGithubCopilotQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerNanoGPTSupport() {
	svc.checkers["nanogpt"] = provider_quota.NewNanoGPTQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerZenmuxSupport() {
	svc.checkers["zenmux"] = provider_quota.NewZenmuxQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerClineSupport() {
	svc.checkers["cline"] = provider_quota.NewClineQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerWaferSupport() {
	svc.checkers["wafer"] = provider_quota.NewWaferQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerSyntheticSupport() {
	svc.checkers["synthetic"] = provider_quota.NewSyntheticQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerNeuralWattSupport() {
	svc.checkers["neuralwatt"] = provider_quota.NewNeuralWattQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerApertisSupport() {
	svc.checkers["apertis"] = provider_quota.NewApertisQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerOpenCodeGoSupport() {
	svc.checkers["opencode_go"] = provider_quota.NewOpenCodeGoQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerKimiCodeSupport() {
	svc.checkers["kimi_code"] = provider_quota.NewKimiCodeQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerMinimaxSupport() {
	svc.checkers["minimax"] = provider_quota.NewMinimaxQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerZhipuSupport() {
	svc.checkers["zhipu"] = provider_quota.NewZhipuQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) registerCharmHyperSupport() {
	svc.checkers["charm_hyper"] = provider_quota.NewCharmHyperQuotaChecker(svc.httpClient)
}

func (svc *ProviderQuotaService) intervalToCronExpr(interval time.Duration) string {
	minutes := int(interval.Minutes())
	hours := int(interval.Hours())

	// Hourly or longer intervals
	if hours >= 1 && minutes%60 == 0 {
		if hours == 1 {
			return "0 * * * *" // Every hour
		}

		return fmt.Sprintf("0 */%d * * *", hours) // Every N hours
	}

	// Minute intervals that divide evenly into 60
	if minutes > 0 && 60%minutes == 0 {
		return fmt.Sprintf("*/%d * * * *", minutes)
	}

	// Round down to nearest supported interval (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60)
	supportedIntervals := []int{1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60}
	filtered := lo.Filter(supportedIntervals, func(si int, _ int) bool {
		return si <= minutes
	})

	rounded := 60
	if len(filtered) > 0 {
		rounded = lo.Max(filtered)
	}

	log.Warn(context.Background(), "Quota check interval does not divide evenly into 60 minutes, rounding to nearest supported interval",
		log.Int("requested_minutes", minutes),
		log.Int("rounded_minutes", rounded))

	return fmt.Sprintf("*/%d * * * *", rounded)
}

func (svc *ProviderQuotaService) getWarningCheckInterval() time.Duration {
	ratio := svc.warningCheckIntervalRatio
	if ratio <= 0 {
		ratio = 4
	}

	return svc.getCheckInterval() * time.Duration(ratio)
}

func (svc *ProviderQuotaService) nextCheckIntervalForStatus(status providerquotastatus.Status) time.Duration {
	if status == providerquotastatus.StatusWarning {
		return svc.getWarningCheckInterval()
	}
	return svc.getCheckInterval()
}

func (svc *ProviderQuotaService) getCheckInterval() time.Duration {
	if svc.checkInterval > 0 {
		return svc.checkInterval
	}

	return 5 * time.Minute
}

// loadQuotaCache warms the in-memory routing cache from persisted
// provider_quota_status rows so a restart does not start cold. Fresh data
// arrives afterwards from the polling engine (saveQuotaStatus) and from the
// usage monitor callbacks wired in the constructor.
func (svc *ProviderQuotaService) loadQuotaCache(ctx context.Context) {
	records, err := svc.db.ProviderQuotaStatus.Query().All(ctx)
	if err != nil {
		log.Error(ctx, "Failed to load quota cache from DB", log.Cause(err))
		return
	}

	for _, r := range records {
		svc.quotaCache.Store(r.ChannelID, &QuotaChannelStatus{
			ProviderType: r.ProviderType.String(),
			Status:       r.Status,
			Ready:        r.Ready,
			Limits:       extractLimitsFromQuotaData(r.QuotaData),
		})
	}

	log.Debug(ctx, "Loaded quota cache from DB", log.Int("records", len(records)))
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

	return svc.invalidateChannelQuotaLocked(ctx, channelID)
}

// invalidateChannelQuotaLocked removes persisted and cached quota state while
// svc.mu is already held by the quota collection loop.
func (svc *ProviderQuotaService) invalidateChannelQuotaLocked(ctx context.Context, channelID int) error {
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
	svc.runQuotaCheckForce(ctx)
}

// ListResets returns the reset capability and available resets for a channel.
// Providers that do not implement Resetter report Supported=false without an
// error so callers can treat resetting as an optional capability.
func (svc *ProviderQuotaService) ListResets(ctx context.Context, channelID int) (provider_quota.ResetList, error) {
	ch, err := svc.db.Channel.Query().Where(channel.IDEQ(channelID)).Only(ctx)
	if err != nil {
		return provider_quota.ResetList{}, fmt.Errorf("failed to load channel: %w", err)
	}

	providerType := svc.getProviderType(ch)
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

	providerType := svc.getProviderType(ch)
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
	now := time.Now()
	svc.checkChannelQuota(ctx, quotaCheckGroup{
		channels:   []*ent.Channel{ch},
		accountKey: quotaAccountKey(providerType, ch),
	}, now)
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

func (svc *ProviderQuotaService) runQuotaCheckForce(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.runQuotaCheck(ctx, true)
}

func (svc *ProviderQuotaService) runQuotaCheck(ctx context.Context, force bool) {
	ctx = ent.NewContext(ctx, svc.db)
	settings := svc.SystemService.ProviderQuotaCollectionSettingsOrDefault(ctx)
	if !settings.Enabled {
		return
	}

	now := time.Now()
	log.Debug(ctx, "Checking for channels to poll",
		log.Time("now", now),
		log.String("now_formatted", now.Format(time.RFC3339)),
		log.Bool("force", force),
	)

	q := svc.db.Channel.Query().
		Where(
			channel.StatusEQ(channel.StatusEnabled),
			channel.TypeIn(providerQuotaChannelTypes...),
		)

	channelsToCheck, err := q.
		WithProviderQuotaStatus().
		All(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query channels for quota check", log.Cause(err))
		return
	}
	channelsToCheck = lo.Filter(channelsToCheck, func(ch *ent.Channel, _ int) bool {
		providerType := svc.getProviderType(ch)
		return providerType != "" && settings.Providers[providerType]
	})

	if len(channelsToCheck) == 0 {
		log.Debug(ctx, "No channels need quota check at this time")
		return
	}

	log.Info(ctx, "Running quota check",
		log.Int("channels", len(channelsToCheck)),
		log.Bool("force", force),
	)

	channelGroups := svc.groupChannelsByQuotaAccount(channelsToCheck)
	if !force {
		channelGroups = lo.Filter(channelGroups, func(group quotaCheckGroup, _ int) bool {
			return quotaCheckGroupIsDue(group, now)
		})
	}
	if len(channelGroups) == 0 {
		log.Debug(ctx, "No channels need quota check at this time")
		return
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(min(maxConcurrentQuotaChecks, len(channelGroups)))
	for _, group := range channelGroups {
		eg.Go(func() error {
			svc.checkChannelQuota(egCtx, group, now)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		log.Info(ctx, "quota check group interrupted", log.Cause(err))
	}
}

// checkChannelQuota runs under svc.mu, held by both scheduled and manual checks.
// That lets credential failures remove persisted and cached status atomically
// with the rest of the quota collection state.
func (svc *ProviderQuotaService) checkChannelQuota(ctx context.Context, group quotaCheckGroup, now time.Time) {
	ch := group.channels[0]
	providerType := svc.getProviderType(ch)
	if providerType == "" {
		return
	}
	if enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType); err != nil {
		log.Warn(ctx, "failed to read provider quota collection settings",
			log.String("provider", providerType),
			log.Cause(err))
	} else if !enabled {
		return
	}

	if !hasCredentialsForProvider(ch) {
		if err := svc.invalidateChannelQuotaLocked(ctx, ch.ID); err != nil {
			log.Error(ctx, "Failed to invalidate provider quota after credentials disappeared",
				log.Int("channel_id", ch.ID),
				log.String("provider", providerType),
				log.Cause(err))
		}
		log.Debug(ctx, "channel does not support check quota", log.Int("channel_id", ch.ID), log.String("channel_name", ch.Name))
		return
	}

	checker, ok := svc.checkers[providerType]
	if !ok {
		log.Error(ctx, "No checker for provider",
			log.String("provider", providerType),
			log.Int("channel_id", ch.ID))

		return
	}

	// Make quota check request
	quotaData, err := checker.CheckQuota(ctx, ch)
	if err != nil {
		log.Error(ctx, "Quota check failed",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
			log.String("provider", providerType),
			log.Cause(err))

		failures := nextQuotaGroupErrorCount(group.channels, providerType)
		for _, member := range group.channels {
			svc.saveQuotaError(ctx, member, providerType, group.accountKey, err, failures, now)
		}
		return
	}

	resetList := provider_quota.ResetList{Supported: false, Resets: nil, Error: ""}
	if resetter, ok := checker.(provider_quota.Resetter); ok {
		resetList.Supported = true
		resetList, err = resetter.ListResets(ctx, ch)
		resetList.Supported = true
		if err != nil {
			resetList.Error = err.Error()
			log.Warn(ctx, "Failed to list provider quota resets",
				log.Int("channel_id", ch.ID),
				log.String("provider", providerType),
				log.Cause(err))
		}
	}
	quotaData.Resets = &resetList

	for _, member := range group.channels {
		memberQuotaData := quotaData
		memberQuotaData.Limits = slices.Clone(quotaData.Limits)
		svc.fillPeriodQuotas(ctx, member.ID, &memberQuotaData, now)
		svc.saveQuotaStatus(ctx, member.ID, providerType, group.accountKey, memberQuotaData, now)

		log.Debug(ctx, "Updated quota status",
			log.Int("channel_id", member.ID),
			log.String("provider", providerType),
			log.String("status", memberQuotaData.Status),
			log.Bool("ready", memberQuotaData.Ready))
	}
}

func (svc *ProviderQuotaService) saveQuotaStatus(
	ctx context.Context,
	channelID int,
	providerType string,
	accountKey string,
	quotaData provider_quota.QuotaData,
	now time.Time,
) {
	nextCheck := now.Add(svc.nextCheckIntervalForStatus(providerquotastatus.Status(quotaData.Status)))
	pt := providerquotastatus.ProviderType(providerType)

	create := svc.db.ProviderQuotaStatus.Create().
		SetChannelID(channelID).
		SetProviderType(pt).
		SetAccountKey(accountKey).
		SetStatus(providerquotastatus.Status(quotaData.Status)).
		SetQuotaData(svc.mergeLimitsIntoQuotaData(quotaData)).
		SetNextCheckAt(nextCheck)

	// Only set next_reset_at if it exists (it's optional in schema)
	if quotaData.NextResetAt != nil {
		create.SetNextResetAt(*quotaData.NextResetAt)
	}

	// Set ready based on status
	create.SetReady(quotaData.Ready)

	upsert := create.
		OnConflict(
			sql.ConflictColumns("channel_id"),
		).
		UpdateNewValues()
	if quotaData.NextResetAt == nil {
		upsert.ClearNextResetAt()
	}

	err := upsert.Exec(ctx)
	if err != nil {
		log.Error(ctx, "Failed to save quota status",
			log.Int("channel_id", channelID),
			log.Cause(err))
		return
	}

	svc.updateQuotaCache(channelID, providerType, providerquotastatus.Status(quotaData.Status), quotaData.Ready, quotaData.Limits)
}

func (svc *ProviderQuotaService) saveQuotaError(
	ctx context.Context,
	ch *ent.Channel,
	providerType string,
	accountKey string,
	quotaErr error,
	failures int,
	now time.Time,
) {
	pt := providerquotastatus.ProviderType(providerType)
	nextCheck := now.Add(quotaErrorBackoff(svc.getCheckInterval(), failures))
	errorCode := quotaErrorCode(quotaErr)

	if ch.Edges.ProviderQuotaStatus != nil {
		existing := ch.Edges.ProviderQuotaStatus
		providerChanged := existing.ProviderType != pt
		invalidCredentials := errors.Is(quotaErr, provider_quota.ErrInvalidCredentials)
		if providerChanged || invalidCredentials {
			nextCheck := now.Add(quotaErrorBackoff(svc.getCheckInterval(), 1))
			quotaData := map[string]any{
				"error_code":  errorCode,
				"error_count": 1,
			}

			// A provider change or invalid credentials makes the previous
			// status and limits untrustworthy, so persist only the error.
			err := svc.db.ProviderQuotaStatus.UpdateOne(existing).
				SetProviderType(pt).
				SetAccountKey(accountKey).
				SetStatus(providerquotastatus.StatusUnknown).
				SetReady(false).
				SetQuotaData(quotaData).
				ClearNextResetAt().
				SetNextCheckAt(nextCheck).
				Exec(ctx)
			if err != nil {
				log.Error(ctx, "Failed to reset quota status after quota check error",
					log.Int("channel_id", ch.ID),
					log.String("previous_provider", existing.ProviderType.String()),
					log.String("provider", providerType),
					log.Cause(err))
				return
			}

			svc.updateQuotaCache(ch.ID, providerType, providerquotastatus.StatusUnknown, false, nil)
			return
		}

		existingData := existing.QuotaData
		if existingData == nil {
			existingData = map[string]any{}
		}

		merged := lo.Assign(existingData, map[string]any{
			"error_code":  errorCode,
			"error_count": failures,
		})
		delete(merged, "error")

		err := svc.db.ProviderQuotaStatus.UpdateOne(existing).
			SetAccountKey(accountKey).
			SetQuotaData(merged).
			SetNextCheckAt(nextCheck).
			Exec(ctx)
		if err != nil {
			log.Error(ctx, "Failed to save quota error",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			return
		}

		existingLimits := extractLimitsFromQuotaData(existing.QuotaData)
		svc.updateQuotaCache(ch.ID, providerType, existing.Status, existing.Ready, existingLimits)

		return
	}

	err := svc.db.ProviderQuotaStatus.Create().
		SetChannelID(ch.ID).
		SetProviderType(pt).
		SetAccountKey(accountKey).
		SetStatus(providerquotastatus.StatusUnknown).
		SetReady(false).
		SetQuotaData(map[string]any{
			"error_code":  errorCode,
			"error_count": failures,
		}).
		SetNextCheckAt(nextCheck).
		Exec(ctx)
	if err != nil {
		log.Error(ctx, "Failed to save quota error",
			log.Int("channel_id", ch.ID),
			log.Cause(err))
		return
	}

	svc.updateQuotaCache(ch.ID, providerType, providerquotastatus.StatusUnknown, false, nil)
}

func (svc *ProviderQuotaService) getProviderType(ch *ent.Channel) string {
	switch ch.Type { //nolint:exhaustive
	case channel.TypeClaudecode:
		return "claudecode"
	case channel.TypeCodex:
		return "codex"
	case channel.TypeAntigravity:
		return "antigravity"
	case channel.TypeXaiSubscription:
		return "xai_subscription"
	case channel.TypeGithubCopilot:
		return "github_copilot"
	case channel.TypeNanogpt, channel.TypeNanogptResponses:
		return "nanogpt"
	case channel.TypeZenmux, channel.TypeZenmuxResponses, channel.TypeZenmuxAnthropic, channel.TypeZenmuxGemini:
		return "zenmux"
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
	case channel.TypeCommandcode, channel.TypeCommandcodeAnthropic:
		return "commandcode"
	default:
		return ""
	}
}

func hasCredentialsForProvider(ch *ent.Channel) bool {
	switch ch.Type { //nolint:exhaustive // Only ZenMux uses the separate management credential.
	case channel.TypeZenmux, channel.TypeZenmuxResponses, channel.TypeZenmuxAnthropic, channel.TypeZenmuxGemini:
		return strings.TrimSpace(ch.Credentials.ManagementAPIKey) != ""
	default:
	}

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

	if isCommandCodeChannelType(ch.Type) {
		// Command Code quota collection is authenticated with the account
		// session cookie, never the inference API key.
		if ch.Settings == nil || ch.Settings.ProviderQuota == nil || ch.Settings.ProviderQuota.CommandCode == nil {
			return false
		}

		_, err := provider_quota.NormalizeCommandCodeCookie(ch.Settings.ProviderQuota.CommandCode.AuthCookie)
		return err == nil
	}

	return ch.Credentials.OAuth != nil || isOAuthJSON(ch.Credentials.APIKey) ||
		strings.TrimSpace(ch.Credentials.APIKey) != "" || len(ch.Credentials.APIKeys) > 0
}

// mergeLimitsIntoQuotaData merges limit statuses into the raw quota data map
// persisted in provider_quota_status.quota_data, keyed under "_limits" so it
// round-trips alongside the provider's own raw fields.
func (svc *ProviderQuotaService) mergeLimitsIntoQuotaData(quotaData provider_quota.QuotaData) map[string]any {
	data := lo.Assign(map[string]any{}, quotaData.RawData)
	if quotaData.Resets != nil {
		data["_resets"] = quotaData.Resets
	}

	if len(quotaData.Limits) > 0 {
		limitMaps := make([]map[string]any, 0, len(quotaData.Limits))
		for _, l := range quotaData.Limits {
			m := map[string]any{
				"type":       string(l.Type),
				"status":     l.Status,
				"usageRatio": l.UsageRatio,
				"ready":      l.Ready,
			}
			if l.NextResetAt != nil {
				m["nextResetAt"] = l.NextResetAt.Format(time.RFC3339)
			}
			if l.Window != "" {
				m["window"] = l.Window
			}
			if l.PeriodStart != nil {
				m["periodStart"] = l.PeriodStart.Format(time.RFC3339)
			}
			if l.PeriodCost != nil {
				m["periodCost"] = *l.PeriodCost
			}
			if l.PeriodQuota != nil {
				m["periodQuota"] = *l.PeriodQuota
			}
			limitMaps = append(limitMaps, m)
		}
		data["_limits"] = limitMaps
	}

	return data
}

// extractLimitsFromQuotaData reads the "_limits" list that
// mergeLimitsIntoQuotaData persisted, tolerating both the in-process
// []map[string]any form and the []any form produced by JSON round-trips.
func extractLimitsFromQuotaData(data map[string]any) []provider_quota.QuotaLimitStatus {
	rawLimits, ok := data["_limits"]
	if !ok {
		return nil
	}

	// Handle both []map[string]any (from mergeLimitsIntoQuotaData) and []any (from JSON unmarshaling)
	var limitMaps []map[string]any
	if directMaps, ok := rawLimits.([]map[string]any); ok {
		limitMaps = directMaps
	} else if anySlice, ok := rawLimits.([]any); ok {
		limitMaps = make([]map[string]any, 0, len(anySlice))
		for _, raw := range anySlice {
			if m, ok := raw.(map[string]any); ok {
				limitMaps = append(limitMaps, m)
			}
		}
	} else {
		return nil
	}

	var limits []provider_quota.QuotaLimitStatus

	for _, m := range limitMaps {
		ls := provider_quota.QuotaLimitStatus{
			Type:        "",
			Status:      "",
			UsageRatio:  0,
			Ready:       false,
			NextResetAt: nil,
			Window:      "",
			PeriodStart: nil,
			PeriodCost:  nil,
			PeriodQuota: nil,
		}

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

		if w, ok := m["window"].(string); ok {
			ls.Window = w
		}

		if ts, ok := m["periodStart"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				ls.PeriodStart = &t
			}
		}

		if c, ok := m["periodCost"].(float64); ok {
			ls.PeriodCost = &c
		}

		if q, ok := m["periodQuota"].(float64); ok {
			ls.PeriodQuota = &q
		}

		limits = append(limits, ls)
	}

	return limits
}
