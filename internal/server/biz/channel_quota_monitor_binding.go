package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/objects"
)

// ---------------------------------------------------------------------------
// Input types (used by GraphQL resolvers in Task 5)
// ---------------------------------------------------------------------------

// SaveChannelQuotaMonitorBindingInput represents a single binding to save for a channel.
type SaveChannelQuotaMonitorBindingInput struct {
	UsageMonitorChannelID int                                    `json:"usageMonitorChannelId"`
	Enabled               bool                                   `json:"enabled"`
	TriggerStatuses       []string                               `json:"triggerStatuses"`
	Conditions            []objects.QuotaMonitorBindingCondition `json:"conditions"`
}

// SaveChannelQuotaMonitorBindingsInput represents the full set of bindings for a channel.
type SaveChannelQuotaMonitorBindingsInput struct {
	Strategy string                                `json:"strategy"`
	Bindings []SaveChannelQuotaMonitorBindingInput `json:"bindings"`
}

// ---------------------------------------------------------------------------
// View types (used by GraphQL resolvers in Task 5)
// ---------------------------------------------------------------------------

// ChannelQuotaMonitorBindingView is the read-model returned by list queries.
type ChannelQuotaMonitorBindingView struct {
	ID                    int                                    `json:"id"`
	ChannelID             int                                    `json:"channelId"`
	UsageMonitorChannelID int                                    `json:"usageMonitorChannelId"`
	UsageMonitorName      string                                 `json:"usageMonitorName"`
	Enabled               bool                                   `json:"enabled"`
	TriggerStatuses       []string                               `json:"triggerStatuses"`
	Conditions            []objects.QuotaMonitorBindingCondition `json:"conditions"`
	LastTriggeredAt       *time.Time                             `json:"lastTriggeredAt"`
	LastTriggerReason     *string                                `json:"lastTriggerReason"`
}

// UsageMonitorBindingSummary provides a per-binding summary for the monitor
// detail page (Task 8). One entry per channel that is bound to the monitor.
type UsageMonitorBindingSummary struct {
	ChannelID             int                                    `json:"channelId"`
	ChannelName           string                                 `json:"channelName"`
	UsageMonitorChannelID int                                    `json:"usageMonitorChannelId"`
	Strategy              string                                 `json:"strategy"`
	Enabled               bool                                   `json:"enabled"`
	TriggerStatuses       []string                               `json:"triggerStatuses"`
	Conditions            []objects.QuotaMonitorBindingCondition `json:"conditions"`
	Matched               bool                                   `json:"matched"`
	Reason                string                                 `json:"reason"`
}

// ---------------------------------------------------------------------------
// Service methods
// ---------------------------------------------------------------------------

// SaveChannelQuotaMonitorBindings replaces all active bindings for a channel
// and stores the aggregation strategy on the channel row. It does NOT evaluate
// the resulting quota-ready state; use SaveChannelQuotaMonitorBindingsAndEvaluate
// for that.
func (svc *UsageMonitorService) SaveChannelQuotaMonitorBindings(
	ctx context.Context,
	channelID int,
	input SaveChannelQuotaMonitorBindingsInput,
) error {
	// Validate / default strategy
	strategy := strings.TrimSpace(input.Strategy)
	if strategy == "" {
		strategy = string(channel.QuotaMultiMonitorStrategyAny)
	}
	if strategy != string(channel.QuotaMultiMonitorStrategyAny) &&
		strategy != string(channel.QuotaMultiMonitorStrategyAll) {
		return fmt.Errorf("invalid strategy %q: must be %q or %q",
			strategy,
			channel.QuotaMultiMonitorStrategyAny,
			channel.QuotaMultiMonitorStrategyAll,
		)
	}

	return svc.RunInTransaction(ctx, func(ctx context.Context) error {
		client := svc.entFromContext(ctx)

		// Soft-delete existing active bindings for this channel
		_, err := client.ChannelUsageMonitorBinding.Update().
			Where(
				channelusagemonitorbinding.ChannelIDEQ(channelID),
				channelusagemonitorbinding.DeletedAtEQ(0),
			).
			SetDeletedAt(int(time.Now().Unix())).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove old bindings for channel %d: %w", channelID, err)
		}

		// Create new binding rows for nonzero monitor IDs
		for _, b := range input.Bindings {
			if b.UsageMonitorChannelID <= 0 {
				continue
			}

			// Clean trigger statuses: trim whitespace, drop blanks
			triggerStatuses := cleanTriggerStatuses(b.TriggerStatuses)

			// Clean conditions: drop entries with empty field or operator
			conditions := cleanConditions(b.Conditions)

			_, err := client.ChannelUsageMonitorBinding.Create().
				SetChannelID(channelID).
				SetUsageMonitorChannelID(b.UsageMonitorChannelID).
				SetEnabled(b.Enabled).
				SetTriggerStatuses(triggerStatuses).
				SetConditions(conditions).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create binding for monitor %d on channel %d: %w",
					b.UsageMonitorChannelID, channelID, err)
			}
		}

		// Store strategy on the channel row
		_, err = client.Channel.UpdateOneID(channelID).
			SetQuotaMultiMonitorStrategy(channel.QuotaMultiMonitorStrategy(strategy)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to set strategy on channel %d: %w", channelID, err)
		}

		return nil
	})
}

// SoftDeleteBindingsForChannel soft-deletes all active quota monitor bindings
// for the given channel. Called when a channel is hard-deleted so it leaves no
// orphan bindings, which would otherwise surface in binding summaries with an
// empty strategy and break the frontend zod schema.
func (svc *UsageMonitorService) SoftDeleteBindingsForChannel(ctx context.Context, channelID int) error {
	_, err := svc.entFromContext(ctx).ChannelUsageMonitorBinding.Update().
		Where(
			channelusagemonitorbinding.ChannelIDEQ(channelID),
			channelusagemonitorbinding.DeletedAtEQ(0),
		).
		SetDeletedAt(int(time.Now().Unix())).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to soft-delete quota monitor bindings for channel %d: %w", channelID, err)
	}

	return nil
}

// softDeleteBindingsForMonitor soft-deletes all active quota monitor bindings
// that reference the given monitor. Called when a usage monitor channel is
// deleted so its bindings don't linger as orphans.
func (svc *UsageMonitorService) softDeleteBindingsForMonitor(ctx context.Context, monitorID int) error {
	_, err := svc.entFromContext(ctx).ChannelUsageMonitorBinding.Update().
		Where(
			channelusagemonitorbinding.UsageMonitorChannelIDEQ(monitorID),
			channelusagemonitorbinding.DeletedAtEQ(0),
		).
		SetDeletedAt(int(time.Now().Unix())).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove quota monitor bindings for monitor %d: %w", monitorID, err)
	}

	return nil
}

// CleanupOrphanedBindings soft-deletes active quota monitor bindings whose
// channel no longer exists (hard-deleted) or whose monitor has been
// soft-deleted. These orphans accumulate when upgrading from older versions
// that did not clean up bindings on channel/monitor deletion. Returns the
// number of bindings removed.
func (svc *UsageMonitorService) CleanupOrphanedBindings(ctx context.Context) (int, error) {
	client := svc.entFromContext(ctx)
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.DeletedAtEQ(0)).
		WithChannel().
		WithUsageMonitorChannel(func(q *ent.UsageMonitorChannelQuery) {
			q.Where(usagemonitorchannel.DeletedAtEQ(0))
		}).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query bindings for orphan cleanup: %w", err)
	}

	now := int(time.Now().Unix())
	removed := 0
	for _, b := range bindings {
		// Valid binding: both the channel and an active monitor exist.
		if b.Edges.Channel != nil && b.Edges.UsageMonitorChannel != nil {
			continue
		}

		if _, err := client.ChannelUsageMonitorBinding.UpdateOneID(b.ID).
			SetDeletedAt(now).
			Save(ctx); err != nil {
			return removed, fmt.Errorf("failed to soft-delete orphan binding %d: %w", b.ID, err)
		}
		removed++
	}

	return removed, nil
}

// SaveChannelQuotaMonitorBindingsAndEvaluate is like
// SaveChannelQuotaMonitorBindings but also evaluates the channel's
// quota-ready state after the transaction commits.
func (svc *UsageMonitorService) SaveChannelQuotaMonitorBindingsAndEvaluate(
	ctx context.Context,
	channelID int,
	input SaveChannelQuotaMonitorBindingsInput,
) error {
	if err := svc.SaveChannelQuotaMonitorBindings(ctx, channelID, input); err != nil {
		return err
	}

	// Evaluate outside the transaction so the new binding rows are visible.
	if err := svc.evaluateAndUpdateChannelQuotaReady(ctx, channelID); err != nil {
		log.Warn(ctx, "failed to evaluate channel quota-ready after saving bindings",
			log.Int("channel_id", channelID),
			log.Cause(err))
	}
	return nil
}

// ListChannelQuotaMonitorBindings returns all active bindings for a channel,
// preloading the associated monitor for display.
func (svc *UsageMonitorService) ListChannelQuotaMonitorBindings(
	ctx context.Context,
	channelID int,
) ([]ChannelQuotaMonitorBindingView, error) {
	client := svc.entFromContext(ctx)

	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(
			channelusagemonitorbinding.ChannelIDEQ(channelID),
			channelusagemonitorbinding.DeletedAtEQ(0),
		).
		WithUsageMonitorChannel(func(q *ent.UsageMonitorChannelQuery) {
			q.Where(usagemonitorchannel.DeletedAtEQ(0))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query bindings for channel %d: %w", channelID, err)
	}

	views := make([]ChannelQuotaMonitorBindingView, 0, len(bindings))
	for _, b := range bindings {
		view := ChannelQuotaMonitorBindingView{
			ID:                    b.ID,
			ChannelID:             b.ChannelID,
			UsageMonitorChannelID: b.UsageMonitorChannelID,
			Enabled:               b.Enabled,
			TriggerStatuses:       normalizeStringSlice(b.TriggerStatuses),
			Conditions:            normalizeConditions(b.Conditions),
			LastTriggeredAt:       b.LastTriggeredAt,
			LastTriggerReason:     b.LastTriggerReason,
		}
		if b.Edges.UsageMonitorChannel != nil {
			view.UsageMonitorName = b.Edges.UsageMonitorChannel.Name
		}
		views = append(views, view)
	}

	return views, nil
}

// ListUsageMonitorBindingSummaries returns one summary entry per channel that
// has an active binding to the given monitor. Used on the monitor detail page
// to show which channels are affected and their current evaluation result.
func (svc *UsageMonitorService) ListUsageMonitorBindingSummaries(
	ctx context.Context,
) ([]UsageMonitorBindingSummary, error) {
	client := svc.entFromContext(ctx)

	// Query all active bindings, preloading channel and monitor
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.DeletedAtEQ(0)).
		WithChannel().
		WithUsageMonitorChannel(func(q *ent.UsageMonitorChannelQuery) {
			q.Where(usagemonitorchannel.DeletedAtEQ(0))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage monitor binding summaries: %w", err)
	}

	summaries := make([]UsageMonitorBindingSummary, 0, len(bindings))
	for _, b := range bindings {
		// Skip orphan bindings (their channel was hard-deleted). These are also
		// cleaned up on startup by CleanupOrphanedBindings; filtering here is a
		// guard so legacy orphans never surface with a blank channel name.
		if b.Edges.Channel == nil {
			continue
		}
		summary := UsageMonitorBindingSummary{
			ChannelID:             b.ChannelID,
			UsageMonitorChannelID: b.UsageMonitorChannelID,
			Enabled:               b.Enabled,
			TriggerStatuses:       normalizeStringSlice(b.TriggerStatuses),
			Conditions:            normalizeConditions(b.Conditions),
		}

		var chStrategy *channel.QuotaMultiMonitorStrategy
		if b.Edges.Channel != nil {
			summary.ChannelName = b.Edges.Channel.Name
			chStrategy = b.Edges.Channel.QuotaMultiMonitorStrategy
		}
		// Always set Strategy (defaulting to the configured default) so an
		// orphan binding with a missing channel never yields an empty value,
		// which the frontend zod schema (z.enum(['any','all'])) would reject.
		summary.Strategy = resolveStrategy(chStrategy, svc.defaultMultiMonitorStrategy)

		if b.Edges.UsageMonitorChannel != nil {
			// Evaluate this single binding against the monitor's current state
			monitor := b.Edges.UsageMonitorChannel
			parsedFields := extractParsedFieldsFromMonitor(monitor)
			ruleResult := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
				MonitorName:     monitor.Name,
				QuotaStatus:     string(monitor.QuotaStatus),
				TriggerStatuses: b.TriggerStatuses,
				Conditions:      b.Conditions,
				ParsedFields:    parsedFields,
				LastPollData:    monitor.LastPollData,
				QuotaLimits:     monitor.QuotaLimits,
			})
			summary.Matched = ruleResult.Matched
			summary.Reason = ruleResult.Reason
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// hasActiveBindingsForMonitor returns true if at least one active (non-soft-deleted)
// ChannelUsageMonitorBinding row exists for the given monitor ID. Used by pollChannel
// to decide whether the legacy direct-channel evaluation can be skipped in favor of
// the binding-based evaluateChannelsForMonitor path.
func (svc *UsageMonitorService) hasActiveBindingsForMonitor(ctx context.Context, monitorID int) bool {
	client := svc.entFromContext(ctx)
	n, err := client.ChannelUsageMonitorBinding.Query().
		Where(
			channelusagemonitorbinding.UsageMonitorChannelIDEQ(monitorID),
			channelusagemonitorbinding.DeletedAtEQ(0),
		).
		Limit(1).
		Count(ctx)
	if err != nil {
		log.Warn(ctx, "failed to check bindings for monitor, assuming none",
			log.Int("monitor_id", monitorID),
			log.Cause(err))
		return false
	}
	return n > 0
}

// evaluateChannelsForMonitor re-evaluates all channels that have an active
// binding to the given monitor. Called after a monitor is updated, refreshed,
// or deleted.
func (svc *UsageMonitorService) evaluateChannelsForMonitor(ctx context.Context, monitorID int) {
	client := svc.entFromContext(ctx)

	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(
			channelusagemonitorbinding.UsageMonitorChannelIDEQ(monitorID),
			channelusagemonitorbinding.DeletedAtEQ(0),
		).
		All(ctx)
	if err != nil {
		log.Warn(ctx, "failed to query bindings for monitor re-evaluation",
			log.Int("monitor_id", monitorID),
			log.Cause(err))
		return
	}

	// Collect unique channel IDs to avoid evaluating the same channel twice.
	seen := make(map[int]struct{}, len(bindings))
	for _, b := range bindings {
		if _, ok := seen[b.ChannelID]; ok {
			continue
		}
		seen[b.ChannelID] = struct{}{}

		if err := svc.evaluateAndUpdateChannelQuotaReady(ctx, b.ChannelID); err != nil {
			log.Warn(ctx, "failed to re-evaluate channel after monitor change",
				log.Int("channel_id", b.ChannelID),
				log.Int("monitor_id", monitorID),
				log.Cause(err))
		}
	}
}

// cleanTriggerStatuses trims whitespace from each status and drops blanks.
func cleanTriggerStatuses(statuses []string) []string {
	if len(statuses) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(statuses))
	for _, s := range statuses {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// cleanConditions drops conditions with empty field or operator.
func cleanConditions(conditions []objects.QuotaMonitorBindingCondition) []objects.QuotaMonitorBindingCondition {
	if len(conditions) == 0 {
		return []objects.QuotaMonitorBindingCondition{}
	}
	result := make([]objects.QuotaMonitorBindingCondition, 0, len(conditions))
	for _, c := range conditions {
		if strings.TrimSpace(c.Field) == "" || strings.TrimSpace(string(c.Operator)) == "" {
			continue
		}
		result = append(result, c)
	}
	return result
}

// normalizeStringSlice ensures a non-nil slice is returned so that GraphQL
// [String!]! fields never serialize as null. Ent may store nil for empty
// slices, but the GraphQL schema requires a non-null list.
func normalizeStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// normalizeConditions ensures a non-nil slice is returned so that GraphQL
// [QuotaMonitorBindingCondition!]! fields never serialize as null.
func normalizeConditions(c []objects.QuotaMonitorBindingCondition) []objects.QuotaMonitorBindingCondition {
	if c == nil {
		return []objects.QuotaMonitorBindingCondition{}
	}
	return c
}

// resolveStrategy returns the effective strategy for a channel, falling back
// to the global default when the channel-level value is nil/empty.
func resolveStrategy(chStrategy *channel.QuotaMultiMonitorStrategy, defaultStrategy string) string {
	if chStrategy != nil && *chStrategy != "" {
		return string(*chStrategy)
	}
	return defaultStrategy
}

// extractParsedFieldsFromMonitor builds the parsed field map from a monitor's
// last poll data. It reads the structured "fields" array from LastPollData
// (which the pollChannel method writes as []map[string]any with key/value/percent/total keys)
// and produces the authoritative map used by the evaluator.
func extractParsedFieldsFromMonitor(monitor *ent.UsageMonitorChannel) map[string]any {
	if monitor.LastPollData == nil {
		return map[string]any{}
	}

	fieldsRaw, ok := monitor.LastPollData["fields"]
	if !ok {
		// No structured fields; return last poll data as-is for merging
		return map[string]any{}
	}

	fieldList, ok := fieldsRaw.([]any)
	if !ok {
		return map[string]any{}
	}

	parsed := make(map[string]any, len(fieldList))
	for _, item := range fieldList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key, _ := m["key"].(string)
		if key == "" {
			continue
		}

		// Set the primary value
		if val, exists := m["value"]; exists {
			parsed[key] = val
		}

		// Set key.percent if available
		if pct, exists := m["percent"]; exists {
			parsed[key+".percent"] = pct
		}

		// Set key.total if available
		if total, exists := m["total"]; exists {
			parsed[key+".total"] = total
		}
	}

	return parsed
}

// evaluateAndUpdateChannelQuotaReady evaluates all enabled active binding rows
// for the given channel and updates channel.quotaBindingReady accordingly.
//
// The evaluation uses the binding-based rule evaluator from Task 3. If no
// enabled effective bindings exist, the channel is marked ready=true.
//
// This replaces the old auto-disable-only evaluation. The old code path
// (querying UsageMonitorChannel with AutoDisableEnabled=true) is preserved
// for backward compatibility but is now supplemented by the binding path.
func (svc *UsageMonitorService) evaluateAndUpdateChannelQuotaReady(ctx context.Context, channelID int) error {
	client := svc.entFromContext(ctx)

	// Query enabled active ChannelUsageMonitorBinding rows for this channel,
	// preloading the associated monitor.
	bindings, err := client.ChannelUsageMonitorBinding.Query().
		Where(
			channelusagemonitorbinding.ChannelIDEQ(channelID),
			channelusagemonitorbinding.DeletedAtEQ(0),
			channelusagemonitorbinding.EnabledEQ(true),
		).
		WithUsageMonitorChannel(func(q *ent.UsageMonitorChannelQuery) {
			q.Where(usagemonitorchannel.DeletedAtEQ(0))
		}).
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query bindings for channel %d: %w", channelID, err)
	}

	// If no enabled bindings, fall back to legacy auto-disable path.
	if len(bindings) == 0 {
		return svc.evaluateAndUpdateChannelQuotaReadyLegacy(ctx, channelID)
	}

	// Get the channel's strategy
	ch, err := client.Channel.Query().
		Where(channel.IDEQ(channelID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to get channel %d: %w", channelID, err)
	}

	strategy := resolveStrategy(ch.QuotaMultiMonitorStrategy, svc.defaultMultiMonitorStrategy)

	// Evaluate each binding
	var results []quotaMonitorBindingRuleResult
	for _, b := range bindings {
		monitor := b.Edges.UsageMonitorChannel

		// Skip nil or paused monitors. Error-state monitors still keep their
		// last quota fields/status, so they must continue to enforce bindings
		// until a later successful poll recovers them.
		if monitor == nil || monitor.Status == usagemonitorchannel.StatusPaused {
			continue
		}

		parsedFields := extractParsedFieldsFromMonitor(monitor)

		ruleResult := evaluateQuotaMonitorBindingRule(quotaMonitorBindingRuleInput{
			MonitorName:     monitor.Name,
			QuotaStatus:     string(monitor.QuotaStatus),
			TriggerStatuses: b.TriggerStatuses,
			Conditions:      b.Conditions,
			ParsedFields:    parsedFields,
			LastPollData:    monitor.LastPollData,
			QuotaLimits:     monitor.QuotaLimits,
		})

		results = append(results, ruleResult)
	}

	// Aggregate results
	ready, reasons := aggregateQuotaMonitorBindingResults(strategy, results)

	// Build error message if not ready
	var errorMsg string
	if !ready {
		errorMsg = fmt.Sprintf("Channel temporarily disabled due to quota exhaustion (%s)", reasons)
	}

	svc.notifyBindingsActive(channelID, true)
	return svc.updateChannelQuotaBindingReady(ctx, channelID, ready, errorMsg)
}

// notifyBindingsActive reports whether the channel is on the binding path
// (hasBindings=true) or the legacy auto-disable fallback path (hasBindings=false)
// to the registered callback, so ProviderQuotaService can gate the orchestrator's
// independent quotaStatus exhaustion filter. No-op when no callback is registered.
func (svc *UsageMonitorService) notifyBindingsActive(channelID int, hasBindings bool) {
	if svc.bindingsActiveCallback != nil {
		svc.bindingsActiveCallback(channelID, hasBindings)
	}
}

// evaluateAndUpdateChannelQuotaReadyLegacy is the original auto-disable-based
// evaluation path, used when no binding rows exist yet.
func (svc *UsageMonitorService) evaluateAndUpdateChannelQuotaReadyLegacy(ctx context.Context, channelID int) error {
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
		svc.notifyBindingsActive(channelID, false)
		return svc.updateChannelQuotaBindingReady(ctx, channelID, true, "")
	}

	// Get the channel's multi-monitor strategy
	ch, err := client.Channel.Query().
		Where(channel.ID(channelID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("failed to get channel %d: %w", channelID, err)
	}

	strategy := ch.QuotaMultiMonitorStrategy
	if strategy == nil || *strategy == "" {
		defaultStrategy := channel.QuotaMultiMonitorStrategy(svc.defaultMultiMonitorStrategy)
		strategy = &defaultStrategy
	}

	var ready bool
	var errorMsg string

	switch *strategy {
	case channel.QuotaMultiMonitorStrategyAny:
		ready = true
		for _, m := range monitors {
			if m.QuotaReady == nil || !*m.QuotaReady {
				ready = false
				errorMsg = buildErrorMessage(m)
				break
			}
		}

	case channel.QuotaMultiMonitorStrategyAll:
		allNotReady := true
		var firstNotReady *ent.UsageMonitorChannel
		for _, m := range monitors {
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
		log.Warn(ctx, "Unknown multi-monitor strategy, defaulting to ready",
			log.Int("channel_id", channelID),
			log.String("strategy", string(*strategy)),
		)
		ready = true
	}

	svc.notifyBindingsActive(channelID, false)
	return svc.updateChannelQuotaBindingReady(ctx, channelID, ready, errorMsg)
}

// evaluateAndUpdateChannelQuotaReadyForTests is the same as
// evaluateAndUpdateChannelQuotaReady but with system bypass for testing.
func (svc *UsageMonitorService) evaluateAndUpdateChannelQuotaReadyForTests(ctx context.Context, channelID int) error {
	ctx = authz.WithSystemBypass(ctx, "test")
	return svc.evaluateAndUpdateChannelQuotaReady(ctx, channelID)
}
