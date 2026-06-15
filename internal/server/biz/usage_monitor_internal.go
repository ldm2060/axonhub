package biz

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
)

func (svc *UsageMonitorService) runPollAllScheduled(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	ctx = authz.WithSystemBypass(ctx, "usage_monitor")
	svc.runPollAll(ctx)
}

// evaluateAndUpdateChannelQuotaReady evaluates all monitors bound to a channel
// and updates the channel's quota_binding_ready status based on the aggregation strategy.
func (svc *UsageMonitorService) evaluateAndUpdateChannelQuotaReady(ctx context.Context, channelID int) error {
	client := svc.entFromContext(ctx)

	// Query all active monitors with auto_disable_enabled=true for this channel
	monitors, err := client.UsageMonitorChannel.Query().
		Where(
			usagemonitorchannel.ChannelID(channelID),
			usagemonitorchannel.StatusEQ(usagemonitorchannel.StatusActive),
			usagemonitorchannel.AutoDisableEnabled(true),
			usagemonitorchannel.DeletedAtEQ(0),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query monitors for channel %d: %w", channelID, err)
	}

	// If no monitors with auto-disable, set ready=true
	if len(monitors) == 0 {
		return svc.updateChannelQuotaBindingReady(ctx, channelID, true, "")
	}

	// Get the channel's multi-monitor strategy
	ch, err := client.Channel.Query().
		Where(channel.ID(channelID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to get channel %d: %w", channelID, err)
	}

	// Determine strategy: use channel-level if set, otherwise fall back to global default
	strategy := ch.QuotaMultiMonitorStrategy
	if strategy == nil || *strategy == "" {
		defaultStrategy := channel.QuotaMultiMonitorStrategy(svc.defaultMultiMonitorStrategy)
		strategy = &defaultStrategy
	}

	var ready bool
	var errorMsg string

	switch *strategy {
	case channel.QuotaMultiMonitorStrategyAny:
		// Disable if ANY monitor is not ready
		ready = true
		for _, m := range monitors {
			// Handle nullable QuotaReady field safely
			if m.QuotaReady == nil || !*m.QuotaReady {
				ready = false
				errorMsg = buildErrorMessage(m)
				break
			}
		}

	case channel.QuotaMultiMonitorStrategyAll:
		// Disable if ALL monitors are not ready
		allNotReady := true
		var firstNotReady *ent.UsageMonitorChannel
		for _, m := range monitors {
			// Handle nullable QuotaReady field safely
			if m.QuotaReady != nil && *m.QuotaReady {
				allNotReady = false
				break
			} else if firstNotReady == nil {
				firstNotReady = m
			}
		}
		if allNotReady {
			ready = false
			if firstNotReady != nil {
				errorMsg = buildErrorMessage(firstNotReady)
			} else if len(monitors) > 0 {
				errorMsg = buildErrorMessage(monitors[0])
			}
		} else {
			ready = true
		}

	default:
		// Unknown strategy - default to ready
		log.Warn(ctx, "Unknown multi-monitor strategy, defaulting to ready",
			log.Int("channel_id", channelID),
			log.String("strategy", string(*strategy)),
		)
		ready = true
	}

	return svc.updateChannelQuotaBindingReady(ctx, channelID, ready, errorMsg)
}

// updateChannelQuotaBindingReady updates a channel's quota_binding_ready field and error_message.
func (svc *UsageMonitorService) updateChannelQuotaBindingReady(ctx context.Context, channelID int, ready bool, errorMsg string) error {
	// Use system bypass to update channel quota binding status
	ctx = authz.WithSystemBypass(ctx, "usage_monitor_quota_binding")
	client := svc.entFromContext(ctx)

	update := client.Channel.UpdateOneID(channelID).
		SetQuotaBindingReady(ready)

	// error_message is shared with other subsystems: auto-disable
	// (channel_auto_disable.go) and all-API-keys-disabled set it together with
	// flipping the channel to StatusDisabled. Only touch error_message for
	// channels that are still enabled — those are excluded from routing purely
	// by quota status, so the quota-exhaustion reason is ours to own. A disabled
	// channel keeps the message from whichever subsystem disabled it.
	ch, statusErr := client.Channel.Query().
		Where(channel.IDEQ(channelID)).
		Select(channel.FieldStatus).
		Only(ctx)
	if statusErr == nil && ch.Status == channel.StatusEnabled {
		if errorMsg != "" {
			update.SetErrorMessage(errorMsg)
		} else {
			update.ClearErrorMessage()
		}
	} else if statusErr != nil && !ent.IsNotFound(statusErr) {
		log.Warn(ctx, "failed to read channel status before updating quota binding error_message",
			log.Int("channel_id", channelID),
			log.Cause(statusErr))
	}

	_, err := update.Save(ctx)

	if err == nil {
		log.Info(
			ctx, "Channel quota binding status updated",
			log.Int("channel_id", channelID),
			log.Bool("ready", ready),
			log.String("error_msg", errorMsg),
		)

		// Refresh the enabled-channels cache so ProviderQuotaSelector stops/starts
		// using this channel immediately, instead of waiting up to 1 minute for
		// the cache's refresh tick.
		if svc.channelsReloadCallback != nil {
			svc.channelsReloadCallback(ctx, channelID)
		}
	}

	return err
}

// buildErrorMessage constructs an error message from a monitor's state.
func buildErrorMessage(monitor *ent.UsageMonitorChannel) string {
	// Extract usage ratio from last_poll_data if available
	var usageStr string
	if monitor.LastPollData != nil {
		// Try to find the highest usage ratio from quota_limits
		maxRatio := 0.0
		for _, limit := range monitor.QuotaLimits {
			if ratio, ok := limit["usageRatio"].(float64); ok && ratio > maxRatio {
				maxRatio = ratio
			}
		}
		if maxRatio > 0 {
			usageStr = fmt.Sprintf("%.0f%%", maxRatio*100)
		} else {
			usageStr = "N/A"
		}
	} else {
		usageStr = "N/A"
	}

	return fmt.Sprintf(
		"Channel temporarily disabled due to quota exhaustion (monitor: %s, usage: %s)",
		monitor.Name,
		usageStr,
	)
}
