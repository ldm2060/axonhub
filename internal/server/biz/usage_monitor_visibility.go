package biz

// 职责: 按渠道可用性过滤使用监视器列表，决定一个用户能在右上角电池面板看到哪些配额监控。
// 关联文件:
//   internal/server/biz/usage_monitor.go
//   internal/server/biz/usage_monitor_period_quota.go
//   internal/scopes/rule_channel_usage.go
//   internal/objects/channel.go
// 遵循的 Rules:
//   .agents/rules/channel-access.md
//   .agents/rules/quota-header-monitors.md
// 注意事项:
//   "监控对应的渠道"沿用周期额度估算的归属规则：绑定渠道优先，否则同 providerType 的启用渠道，再加上把该监控当作拼车总额的渠道。
//   监控的创建者永远能看到自己的监控，否则别人的私有渠道会让自己的模板监控从面板里消失。
//   渠道查询走系统 bypass：请求主体按可见性查渠道，共享渠道不在结果里。

import (
	"context"
	"slices"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/scopes"
)

// filterMonitorsByChannelAccess keeps the monitors the viewer may see: a monitor
// is listed when at least one channel its quota belongs to is routable by that
// user — their own, published, shared with them, or pooled with them — or when
// the viewer is the one who added it. A monitor someone else added that belongs
// to no channel at all (a custom monitor, or a template of a provider with no
// AxonHub channels) has no channel to gate on and stays hidden, so one user's
// private account never shows up in another's panel.
//
// The system owner and admins keep the unfiltered list: they may route through
// every channel, and a monitor they did not add must not disappear from their
// panel either. A nil viewer is a system principal and is unrestricted.
//
// Fork-specific visibility rule (see .agents/rules/channel-access.md): routing
// availability decides, never the read_channels scope, which self-registered
// users hold by default.
func (svc *UsageMonitorService) filterMonitorsByChannelAccess(
	ctx context.Context,
	viewer *ent.User,
	monitors []*ent.UsageMonitorChannel,
) []*ent.UsageMonitorChannel {
	if viewer == nil || viewer.IsOwner || viewer.IsAdmin {
		return monitors
	}

	channels := svc.monitorVisibilityChannels(ctx, monitors)
	owned := svc.monitorIDsOwnedBy(ctx, viewer.ID)

	byID := make(map[int]*ent.Channel, len(channels))
	byProviderType := make(map[string][]*ent.Channel, len(channels))
	byPooledMonitor := make(map[int][]*ent.Channel, len(channels))
	for _, ch := range channels {
		byID[ch.ID] = ch
		if providerType := channelProviderType(ch); providerType != "" {
			byProviderType[providerType] = append(byProviderType[providerType], ch)
		}
		// A channel pooling off this monitor is a channel the monitor describes,
		// whichever channel the monitor itself is bound to.
		if cfg := poolingConfigOf(ch); cfg != nil && cfg.MonitorID > 0 {
			byPooledMonitor[cfg.MonitorID] = append(byPooledMonitor[cfg.MonitorID], ch)
		}
	}

	visible := make([]*ent.UsageMonitorChannel, 0, len(monitors))
	for _, monitor := range monitors {
		// The author always keeps their own monitor: a template of a provider
		// whose only channels are somebody else's private ones describes the
		// author's own account, not a channel they were granted.
		if owned[monitor.ID] {
			visible = append(visible, monitor)
			continue
		}
		candidates := monitorAccessChannels(monitor, byID, byProviderType, byPooledMonitor)
		if slices.ContainsFunc(candidates, func(ch *ent.Channel) bool {
			return scopes.CanRouteThroughChannel(viewer, ch)
		}) {
			visible = append(visible, monitor)
		}
	}
	return visible
}

// monitorAccessChannels returns the channels whose availability decides whether
// the monitor is listed, in the same order the period-quota estimator prices
// them (see monitorCostChannelIDs). A bound channel is the whole answer — another
// channel of the same provider type never widens a monitor that already
// describes one account — except for the channel pooling off the monitor, whose
// members see it through the share they hold.
func monitorAccessChannels(
	monitor *ent.UsageMonitorChannel,
	byID map[int]*ent.Channel,
	byProviderType map[string][]*ent.Channel,
	byPooledMonitor map[int][]*ent.Channel,
) []*ent.Channel {
	pooled := byPooledMonitor[monitor.ID]

	if monitor.ChannelID != nil {
		bound, ok := byID[*monitor.ChannelID]
		if !ok {
			// The bound channel is gone: nothing left to gate on.
			return nil
		}
		return append([]*ent.Channel{bound}, pooled...)
	}

	providerType := string(monitor.ProviderType)
	if providerType == "" {
		return nil
	}

	matched := byProviderType[providerType]
	candidates := make([]*ent.Channel, 0, len(matched)+len(pooled))
	candidates = append(candidates, matched...)
	candidates = append(candidates, pooled...)
	return candidates
}

// monitorVisibilityChannels loads the channels a monitor's quota can belong to.
// It runs under a system bypass because the requesting principal's own channel
// query is filtered down to what it may see, which would hide the shared
// channels the availability check has to evaluate.
func (svc *UsageMonitorService) monitorVisibilityChannels(ctx context.Context, monitors []*ent.UsageMonitorChannel) []*ent.Channel {
	boundIDs := make([]int, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor.ChannelID != nil {
			boundIDs = append(boundIDs, *monitor.ChannelID)
		}
	}

	query := svc.db.Channel.Query()
	if len(boundIDs) > 0 {
		// A bound channel counts even when it is disabled: its quota is what the
		// monitor reports, and a channel taken out of rotation still has an owner
		// and shares.
		query = query.Where(channel.Or(channel.StatusEQ(channel.StatusEnabled), channel.IDIn(boundIDs...)))
	} else {
		query = query.Where(channel.StatusEQ(channel.StatusEnabled))
	}

	var channels []*ent.Channel
	if err := authz.RunWithSystemBypassVoid(ctx, "usage-monitor-visibility", func(bypassCtx context.Context) error {
		rows, err := query.Select(
			channel.FieldID,
			channel.FieldType,
			channel.FieldBaseURL,
			channel.FieldOwnerID,
			channel.FieldVisibility,
			channel.FieldSharedWith,
			channel.FieldSettings,
		).All(bypassCtx)
		if err != nil {
			return err
		}
		channels = rows
		return nil
	}); err != nil {
		log.Warn(ctx, "failed to load channels for usage monitor visibility", log.Cause(err))
		return nil
	}
	return channels
}

// monitorIDsOwnedBy returns the IDs of the monitors the user added. A monitor
// whose owner record is gone simply drops out and then behaves like any other
// user's monitor.
func (svc *UsageMonitorService) monitorIDsOwnedBy(ctx context.Context, userID int) map[int]bool {
	ids, err := svc.db.UsageMonitorChannel.Query().
		Where(
			usagemonitorchannel.DeletedAtEQ(0),
			usagemonitorchannel.HasOwnerWith(user.IDEQ(userID)),
		).
		IDs(ctx)
	if err != nil {
		log.Warn(ctx, "failed to query owned usage monitor channels", log.Int("user_id", userID), log.Cause(err))
		return nil
	}

	owned := make(map[int]bool, len(ids))
	for _, id := range ids {
		owned[id] = true
	}
	return owned
}
