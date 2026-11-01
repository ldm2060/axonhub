package biz

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/request"
	"github.com/ldm2060/axonhub/internal/ent/usagelog"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/scopes"
)

// Member-facing pool statuses surfaced by the GraphQL queries and the battery
// popover. "unavailable" means the bound monitor could not produce a period
// estimate for the configured window, in which case the pool fails open.
const (
	PoolStatusAvailable   = "available"
	PoolStatusWarning     = "warning"
	PoolStatusPaused      = "paused"
	PoolStatusUnavailable = "unavailable"
)

// poolUsageCacheTTL bounds how stale a member's in-window usage may be on the
// routing path. Within the TTL a member who just exhausted their share can
// still push a few requests through; the next lookup stops them, the same
// trade-off every quota cache in this repo makes.
const poolUsageCacheTTL = time.Minute

// poolUsageCacheMax bounds the usage cache; pruning drops expired entries once
// the map grows past it, so window rollovers and churned members cannot leak.
const poolUsageCacheMax = 4096

// poolWindowAliases maps a pooling window onto the window labels the quota
// derivers stamp onto limits: Claude's 7d pool is the weekly pool and 30d is
// the monthly one, while the estimate config only offers weekly/monthly.
var poolWindowAliases = map[string][]string{
	objects.PoolingWindow5h:      {"5h"},
	objects.PoolingWindowDaily:   {"daily"},
	objects.PoolingWindowWeekly:  {"weekly", "7d"},
	objects.PoolingWindowMonthly: {"monthly", "30d"},
}

// poolMemberState is one member's slice of the pool: the fixed share and the
// sub-quota derived from it (total × share), both recomputed whenever the
// monitor repolls or the channel settings change.
type poolMemberState struct {
	userID   int
	sharePct float64
	quota    float64
}

// channelPoolState is the derived pool state of one channel. It is recomputed
// whenever the channel's UpdatedAt or the monitor's LastPollAt moves, so no
// explicit invalidation callbacks are needed anywhere.
type channelPoolState struct {
	channelUpdatedAt  time.Time
	monitorLastPollAt time.Time
	window            string
	totalQuota        float64
	periodStart       *time.Time
	nextResetAt       *time.Time
	estimateOK        bool
	ownerReservedPct  float64
	members           []poolMemberState
}

func (s *channelPoolState) matches(channelUpdatedAt, monitorLastPollAt time.Time) bool {
	return s.channelUpdatedAt.Equal(channelUpdatedAt) && s.monitorLastPollAt.Equal(monitorLastPollAt)
}

func (s *channelPoolState) memberState(userID int) *poolMemberState {
	for i := range s.members {
		if s.members[i].userID == userID {
			return &s.members[i]
		}
	}
	return nil
}

type poolUsageKey struct {
	channelID       int
	userID          int
	periodStartUnix int64
}

type poolUsageEntry struct {
	cost      float64
	fetchedAt time.Time
}

// ChannelPoolingService splits a channel's estimated quota (from one bound
// usage monitor window) between pooling members. It owns no persistence: the
// config lives in channel settings, the total in the monitor's persisted
// period-quota estimate, and member usage is aggregated from usage logs on
// demand behind a short-TTL cache.
type ChannelPoolingService struct {
	*AbstractService

	channels *ChannelService
	monitors *UsageMonitorService

	mu    sync.Mutex
	state map[int]*channelPoolState

	usageMu    sync.Mutex
	usageCache map[poolUsageKey]poolUsageEntry
}

type ChannelPoolingServiceParams struct {
	fx.In

	Ent      *ent.Client
	Channels *ChannelService
	Monitors *UsageMonitorService
}

func NewChannelPoolingService(params ChannelPoolingServiceParams) *ChannelPoolingService {
	return &ChannelPoolingService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		channels:   params.Channels,
		monitors:   params.Monitors,
		state:      make(map[int]*channelPoolState),
		usageCache: make(map[poolUsageKey]poolUsageEntry),
	}
}

// poolingConfigOf returns the channel's pooling config, or nil when absent.
func poolingConfigOf(ch *ent.Channel) *objects.ChannelPoolingConfig {
	if ch == nil || ch.Settings == nil {
		return nil
	}
	return ch.Settings.Pooling
}

// AllowRoutingThroughChannel reports whether user may route a request through
// the channel under pooling rules. It is the pooling counterpart of
// scopes.CanRouteThroughChannel and must only be called after that check:
// pooling narrows access, it never widens it.
//
// Fail-open semantics: any doubt (no config, non-member, missing estimate,
// usage lookup error) allows the request, so a broken monitor can never lock
// members out of their channels. Only an enabled pool with a live estimate and
// a member at or past their sub-quota blocks.
func (svc *ChannelPoolingService) AllowRoutingThroughChannel(ctx context.Context, user *ent.User, ch *ent.Channel) bool {
	if svc == nil || user == nil || ch == nil {
		return true
	}
	// The owner and system-level operators are exempt from pool enforcement:
	// shares only bound what the owner handed out, never their own channel.
	if user.IsOwner || scopes.HasSystemScope(user, scopes.ScopeWriteChannels) {
		return true
	}
	cfg := poolingConfigOf(ch)
	if cfg == nil || !cfg.Enabled {
		return true
	}
	if cfg.MemberByUserID(user.ID) == nil {
		return true
	}

	state := svc.stateFor(ctx, ch, cfg)
	if !state.estimateOK || state.periodStart == nil {
		return true
	}
	member := state.memberState(user.ID)
	if member == nil || member.quota <= 0 {
		return true
	}

	used, err := svc.memberUsage(ctx, ch.ID, user.ID, *state.periodStart)
	if err != nil {
		log.Warn(ctx, "pool usage lookup failed; failing open",
			log.Cause(err),
			log.Int("channel_id", ch.ID),
			log.Int("user_id", user.ID))
		return true
	}
	return used < member.quota
}

// MyPoolQuota is one pool the viewer is a member of, as shown in the battery
// popover: their share, the sub-quota derived from it, and their in-window
// usage on the pooled channel.
type MyPoolQuota struct {
	ChannelID   int        `json:"channelId"`
	ChannelName string     `json:"channelName"`
	Window      string     `json:"window"`
	SharePct    float64    `json:"sharePct"`
	MemberQuota float64    `json:"memberQuota"`
	Used        float64    `json:"used"`
	TotalQuota  *float64   `json:"totalQuota"`
	PeriodStart *time.Time `json:"periodStart"`
	NextResetAt *time.Time `json:"nextResetAt"`
	Status      string     `json:"status"`
}

// MyPoolQuotas returns every enabled pool the viewer is a member of, across
// all enabled channels.
func (svc *ChannelPoolingService) MyPoolQuotas(ctx context.Context, viewerID int) ([]*MyPoolQuota, error) {
	result := []*MyPoolQuota{}
	if svc == nil || viewerID <= 0 {
		return result, nil
	}

	for _, ch := range svc.channels.GetEnabledChannels() {
		cfg := poolingConfigOf(ch.Channel)
		if cfg == nil || !cfg.Enabled {
			continue
		}
		member := cfg.MemberByUserID(viewerID)
		if member == nil {
			continue
		}

		state := svc.stateFor(ctx, ch.Channel, cfg)
		poolMember := state.memberState(viewerID)
		if poolMember == nil {
			continue
		}

		entry := MyPoolQuota{
			ChannelID:   ch.ID,
			ChannelName: ch.Name,
			Window:      cfg.Window,
			SharePct:    member.SharePct,
			MemberQuota: poolMember.quota,
			TotalQuota:  state.totalQuotaPointer(),
			PeriodStart: state.periodStart,
			NextResetAt: state.nextResetAt,
			Status:      PoolStatusAvailable,
		}
		if !state.estimateOK {
			entry.Status = PoolStatusUnavailable
		} else if used, err := svc.memberUsage(ctx, ch.ID, viewerID, *state.periodStart); err == nil {
			entry.Used = used
			entry.Status = poolMemberStatus(used, poolMember.quota)
		}
		result = append(result, &entry)
	}
	return result, nil
}

// PoolMemberStatus is one member's standing in a channel's pool, for the owner
// panel. Names resolve on the GraphQL side via the SharedUser field so this
// stays free of user lookups.
type PoolMemberStatus struct {
	UserID   int      `json:"userId"`
	SharePct float64  `json:"sharePct"`
	Quota    *float64 `json:"quota"`
	Used     float64  `json:"used"`
	Status   string   `json:"status"`
}

// ChannelPoolStatus is the owner-facing pool state of one channel.
type ChannelPoolStatus struct {
	ChannelID         int                `json:"channelId"`
	Enabled           bool               `json:"enabled"`
	Window            string             `json:"window"`
	TotalQuota        *float64           `json:"totalQuota"`
	PeriodStart       *time.Time         `json:"periodStart"`
	NextResetAt       *time.Time         `json:"nextResetAt"`
	EstimateAvailable bool               `json:"estimateAvailable"`
	OwnerReservedPct  float64            `json:"ownerReservedPct"`
	Members           []PoolMemberStatus `json:"members"`
}

// PoolStatusForChannel returns the pool state of one channel for its owner.
// A channel without pooling returns a disabled, empty status instead of an
// error so the panel can render the "off" state.
func (svc *ChannelPoolingService) PoolStatusForChannel(ctx context.Context, channelID int) (*ChannelPoolStatus, error) {
	if svc == nil {
		return nil, nil
	}
	ch, err := svc.channels.GetChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}

	cfg := poolingConfigOf(ch.Channel)
	if cfg == nil {
		return &ChannelPoolStatus{ChannelID: channelID, Members: []PoolMemberStatus{}}, nil
	}

	state := svc.stateFor(ctx, ch.Channel, cfg)
	status := &ChannelPoolStatus{
		ChannelID:         channelID,
		Enabled:           cfg.Enabled,
		Window:            cfg.Window,
		TotalQuota:        state.totalQuotaPointer(),
		PeriodStart:       state.periodStart,
		NextResetAt:       state.nextResetAt,
		EstimateAvailable: state.estimateOK,
		OwnerReservedPct:  state.ownerReservedPct,
		Members:           []PoolMemberStatus{},
	}
	for _, member := range state.members {
		entry := PoolMemberStatus{
			UserID:   member.userID,
			SharePct: member.sharePct,
			Quota:    &member.quota,
			Status:   PoolStatusUnavailable,
		}
		if state.estimateOK && state.periodStart != nil {
			if used, err := svc.memberUsage(ctx, channelID, member.userID, *state.periodStart); err == nil {
				entry.Used = used
				entry.Status = poolMemberStatus(used, member.quota)
			}
		}
		status.Members = append(status.Members, entry)
	}
	return status, nil
}

// stateFor returns the pool state of the channel, recomputing it when the
// channel settings or the monitor's last poll have moved on since the cached
// state was derived. The equality check doubles as the invalidation mechanism:
// settings writes bump UpdatedAt and polls bump LastPollAt, so no callback
// wiring is needed anywhere.
func (svc *ChannelPoolingService) stateFor(ctx context.Context, ch *ent.Channel, cfg *objects.ChannelPoolingConfig) *channelPoolState {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if state := svc.state[ch.ID]; state != nil {
		if state.matches(ch.UpdatedAt, svc.monitorStamp(ctx, cfg.MonitorID)) {
			return state
		}
	}

	state := svc.computeState(ctx, ch, cfg)
	svc.state[ch.ID] = state
	return state
}

// monitorStamp reads the monitor's LastPollAt from the in-memory cache; a
// monitor the cache has not loaded yet stamps as the zero time, which only
// costs one recompute once it loads.
func (svc *ChannelPoolingService) monitorStamp(ctx context.Context, monitorID int) time.Time {
	if monitor, ok := svc.monitors.GetChannelCached(monitorID); ok && monitor != nil && monitor.LastPollAt != nil {
		return *monitor.LastPollAt
	}
	return time.Time{}
}

// computeState derives the pool state: member quotas always (so the owner
// panel shows configured shares even while the estimate is down), the estimate
// when the monitor can price the configured window.
func (svc *ChannelPoolingService) computeState(ctx context.Context, ch *ent.Channel, cfg *objects.ChannelPoolingConfig) *channelPoolState {
	state := &channelPoolState{
		channelUpdatedAt: ch.UpdatedAt,
		window:           cfg.Window,
		ownerReservedPct: cfg.OwnerReservedPct(),
	}
	for _, m := range cfg.Members {
		state.members = append(state.members, poolMemberState{userID: m.UserID, sharePct: m.SharePct})
	}

	monitor, ok := svc.monitors.GetChannelCached(cfg.MonitorID)
	if !ok {
		// Cache miss (cold start): load through the service so the monitor
		// enters the cache for subsequent hot-path reads.
		loaded, err := svc.monitors.GetChannel(ctx, cfg.MonitorID)
		if err == nil {
			monitor = loaded
		}
	}
	if monitor != nil {
		if monitor.LastPollAt != nil {
			state.monitorLastPollAt = *monitor.LastPollAt
		}
		if total, start, reset, okEst := poolEstimate(monitor, cfg.Window); okEst {
			state.totalQuota = total
			state.periodStart = start
			state.nextResetAt = reset
			state.estimateOK = true
			for i := range state.members {
				state.members[i].quota = total * state.members[i].sharePct / 100
			}
		}
	}
	return state
}

func (s *channelPoolState) totalQuotaPointer() *float64 {
	if !s.estimateOK {
		return nil
	}
	total := s.totalQuota
	return &total
}

// poolEstimate extracts the pool total from a monitor's persisted quota
// limits: the first limit whose window matches (by alias) and that carries a
// positive period-quota estimate. Without a period start the usage window is
// unknowable, so the estimate is rejected.
func poolEstimate(monitor *ent.UsageMonitorChannel, window string) (total float64, start, reset *time.Time, ok bool) {
	labels, known := poolWindowAliases[window]
	if !known {
		return 0, nil, nil, false
	}
	for _, item := range monitor.QuotaLimits {
		w, _ := item["window"].(string)
		if !slices.Contains(labels, w) {
			continue
		}
		quota, quotaOK := poolFloat(item["periodQuota"])
		if !quotaOK || quota <= 0 {
			continue
		}
		periodStart := poolTime(item["periodStart"])
		if periodStart == nil {
			continue
		}
		return quota, periodStart, poolTime(item["nextResetAt"]), true
	}
	return 0, nil, nil, false
}

// poolFloat accepts the number shapes a JSON round-trip can leave behind.
func poolFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// poolTime parses the RFC3339 stamps the persist path writes; a live time.Time
// is accepted defensively for entities that never round-tripped through JSON.
func poolTime(v any) *time.Time {
	switch t := v.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return nil
		}
		return &parsed
	case time.Time:
		parsed := t
		return &parsed
	}
	return nil
}

// poolMemberStatus classifies a member's usage against their sub-quota.
func poolMemberStatus(used, quota float64) string {
	if quota > 0 && used >= quota {
		return PoolStatusPaused
	}
	if quota > 0 && used >= quota*0.8 {
		return PoolStatusWarning
	}
	return PoolStatusAvailable
}

// memberUsage sums the member's cost on the channel since the window started.
// The query joins usage logs to their request for the user dimension, the same
// shape the trace aggregates use, under a system bypass because a request-time
// principal cannot read usage logs across projects (see aggregateChannelCostSince).
func (svc *ChannelPoolingService) memberUsage(ctx context.Context, channelID, userID int, periodStart time.Time) (float64, error) {
	key := poolUsageKey{channelID: channelID, userID: userID, periodStartUnix: periodStart.Unix()}
	now := time.Now()

	svc.usageMu.Lock()
	if entry, ok := svc.usageCache[key]; ok && now.Sub(entry.fetchedAt) < poolUsageCacheTTL {
		svc.usageMu.Unlock()
		return entry.cost, nil
	}
	svc.usageMu.Unlock()

	var cost float64
	if _, err := authz.RunWithSystemBypass(ctx, "channel-pool-usage", func(bypassCtx context.Context) (struct{}, error) {
		q := svc.entFromContext(bypassCtx).UsageLog.Query().Where(
			usagelog.ChannelIDEQ(channelID),
			usagelog.CreatedAtGTE(periodStart),
			usagelog.HasRequestWith(request.UserIDEQ(userID)),
		)
		agg, err := aggregateUsageMetadata(bypassCtx, q)
		if err != nil {
			return struct{}{}, err
		}
		cost = agg.TotalCost.InexactFloat64()
		return struct{}{}, nil
	}); err != nil {
		return 0, err
	}

	svc.usageMu.Lock()
	if len(svc.usageCache) >= poolUsageCacheMax {
		for k, entry := range svc.usageCache {
			if now.Sub(entry.fetchedAt) >= poolUsageCacheTTL {
				delete(svc.usageCache, k)
			}
		}
	}
	svc.usageCache[key] = poolUsageEntry{cost: cost, fetchedAt: now}
	svc.usageMu.Unlock()

	return cost, nil
}
