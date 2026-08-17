package usage_monitor

import (
	"fmt"
	"time"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

const warningThreshold = 0.8

type QuotaDerivedStatus struct {
	Status      string
	Ready       bool
	Limits      []provider_quota.QuotaLimitStatus
	NextResetAt *time.Time
}

// DeriveQuotaStatus derives routing-compatible quota status from parsed field data.
// The providerType determines which fields to inspect and what thresholds to use.
// For unknown/empty provider types, a generic derivation is applied based on
// percentage and fraction display fields.
func DeriveQuotaStatus(providerType string, fields []ParsedField) QuotaDerivedStatus {
	switch providerType {
	case "claudecode":
		return deriveClaudeCode(fields)
	case "codex":
		return deriveCodex(fields)
	case "github_copilot":
		return deriveGitHubCopilot(fields)
	case "nanogpt":
		return deriveNanoGPT(fields)
	case "wafer":
		return deriveWafer(fields)
	case "synthetic":
		return deriveSynthetic(fields)
	case "neuralwatt":
		return deriveNeuralWatt(fields)
	case "apertis":
		return deriveApertis(fields)
	case "zhipu":
		return deriveZhipu(fields)
	case "antigravity":
		return deriveAntigravity(fields)
	case "opencode_go":
		return deriveOpenCodeGo(fields)
	case "cline":
		return deriveCline(fields)
	case "minimax":
		return deriveMinimax(fields)
	case "xai_subscription":
		return deriveXaiSubscription(fields)
	case "charm_hyper":
		return deriveCharmHyper(fields)
	default:
		return deriveGeneric(fields)
	}
}

func deriveClaudeCode(fields []ParsedField) QuotaDerivedStatus {
	var maxUtil float64
	for _, f := range fields {
		if f.Format == "fraction" {
			if v, err := toFloat(f.Value); err == nil && v > maxUtil {
				maxUtil = v
			}
		}
		if f.Key == "unified_status" {
			if val, ok := f.Value.(string); ok && val == "throttled" {
				return QuotaDerivedStatus{
					Status: "exhausted",
					Ready:  false,
					Limits: []provider_quota.QuotaLimitStatus{
						provider_quota.NewTokenLimitStatus("exhausted", maxUtil, nil),
					},
				}
			}
		}
	}

	status := "available"
	if maxUtil >= warningThreshold {
		status = "warning"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status: status,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, maxUtil, nextReset),
		},
		NextResetAt: nextReset,
	}
}

func deriveCodex(fields []ParsedField) QuotaDerivedStatus {
	// If limit_reached is "true", the overall status is exhausted.
	for _, f := range fields {
		if f.Key == "limit_reached" {
			if val, ok := f.Value.(string); ok && val == "true" {
				pct := findFieldPercent(fields, "primary_used_pct") / 100.0
				return QuotaDerivedStatus{
					Status: "exhausted",
					Ready:  false,
					Limits: []provider_quota.QuotaLimitStatus{
						provider_quota.NewTokenLimitStatus("exhausted", pct, nil),
					},
				}
			}
		}
	}

	// Collect all percentage windows: primary, secondary, and additional N.
	type windowInfo struct {
		label     string
		pct       float64
		nextReset *time.Time
	}
	windows := make([]windowInfo, 0, 6)

	// Primary window
	primaryPct := findFieldPercent(fields, "primary_used_pct") / 100.0
	primaryReset := findFieldTime(fields, "primary_reset")
	windows = append(windows, windowInfo{label: "primary", pct: primaryPct, nextReset: primaryReset})

	// Secondary window
	secondaryPct := findFieldPercent(fields, "secondary_used_pct") / 100.0
	secondaryReset := findFieldTime(fields, "secondary_reset")
	windows = append(windows, windowInfo{label: "secondary", pct: secondaryPct, nextReset: secondaryReset})

	// Additional rate limit windows (0 and 1)
	for i := range 2 {
		featureName := findFieldString(fields, fmt.Sprintf("additional_%d_name", i))
		if featureName == "" {
			continue
		}

		for _, suffix := range []string{"primary", "secondary"} {
			pctKey := fmt.Sprintf("additional_%d_%s_pct", i, suffix)
			resetKey := fmt.Sprintf("additional_%d_%s_reset", i, suffix)
			pct := findFieldPercent(fields, pctKey) / 100.0
			if pct == 0 {
				continue
			}
			reset := findFieldTime(fields, resetKey)
			windows = append(windows, windowInfo{
				label:     fmt.Sprintf("%s_%s", featureName, suffix),
				pct:       pct,
				nextReset: reset,
			})
		}
	}

	// Build per-window QuotaLimitStatus entries and find worst status.
	var limits []provider_quota.QuotaLimitStatus
	var worstStatus string

	for _, w := range windows {
		limitStatus := "available"
		if w.pct >= 1.0 {
			limitStatus = "exhausted"
		} else if w.pct >= warningThreshold {
			limitStatus = "warning"
		}

		limits = append(limits, provider_quota.QuotaLimitStatus{
			Type:        provider_quota.QuotaLimitTypeToken,
			Status:      limitStatus,
			UsageRatio:  w.pct,
			Ready:       limitStatus != "exhausted",
			NextResetAt: w.nextReset,
		})

		if worstStatus == "" || quotaStatusRank(limitStatus) > quotaStatusRank(worstStatus) {
			worstStatus = limitStatus
		}
	}

	if worstStatus == "" {
		worstStatus = "available"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status:      worstStatus,
		Ready:       worstStatus != "exhausted",
		Limits:      limits,
		NextResetAt: nextReset,
	}
}

func deriveGitHubCopilot(fields []ParsedField) QuotaDerivedStatus {
	var maxUsage float64
	var hasQuotaData bool

	// Build a map of quota type -> unlimited status
	unlimitedMap := make(map[string]bool)
	for _, f := range fields {
		switch f.Key {
		case "chat_unlimited":
			if v, ok := f.Value.(bool); ok {
				unlimitedMap["chat"] = v
			}
		case "completions_unlimited":
			if v, ok := f.Value.(bool); ok {
				unlimitedMap["completions"] = v
			}
		case "premium_unlimited":
			if v, ok := f.Value.(bool); ok {
				unlimitedMap["premium_interactions"] = v
			}
		}
	}

	for _, f := range fields {
		switch f.Key {
		case "limited_quotas":
			// Free accounts: limited_user_quotas is a map of feature -> remaining count.
			// Compare with monthly_quotas to derive the used ratio.
			limitedMap, ok := f.Value.(map[string]any)
			if !ok || len(limitedMap) == 0 {
				continue
			}
			// Find the monthly_quotas field for totals
			var monthlyMap map[string]any
			for _, mf := range fields {
				if mf.Key == "monthly_quotas" {
					monthlyMap, _ = mf.Value.(map[string]any)
					break
				}
			}
			for key, remainingVal := range limitedMap {
				remaining, err := toFloat(remainingVal)
				if err != nil {
					continue
				}
				total := remaining
				if monthlyMap != nil {
					if t, err2 := toFloat(monthlyMap[key]); err2 == nil && t > 0 {
						total = t
					}
				}
				if total > 0 {
					usedRatio := (total - remaining) / total
					if usedRatio < 0 {
						usedRatio = 0
					}
					if usedRatio > maxUsage {
						maxUsage = usedRatio
					}
					hasQuotaData = true
				}
			}

		case "chat_pct", "completions_pct", "premium_pct":
			// Template rendering converts GitHub's percent_remaining to used percentage.
			var quotaType string
			switch f.Key {
			case "chat_pct":
				quotaType = "chat"
			case "completions_pct":
				quotaType = "completions"
			case "premium_pct":
				quotaType = "premium_interactions"
			}

			// Skip unlimited quotas
			if unlimited, ok := unlimitedMap[quotaType]; ok && unlimited {
				continue
			}

			if pct, err := toFloat(f.Value); err == nil {
				usedRatio := pct / 100.0
				if usedRatio > maxUsage {
					maxUsage = usedRatio
				}
				hasQuotaData = true
			}
		}
	}

	// No quota data available (e.g. only plan/access_type fields)
	if !hasQuotaData {
		nextReset := earliestDatetime(fields)
		return QuotaDerivedStatus{Status: "available", Ready: true, NextResetAt: nextReset}
	}

	status := "available"
	if maxUsage >= 1.0 {
		status = "exhausted"
	} else if maxUsage >= warningThreshold {
		status = "warning"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status:      status,
		Ready:       status != "exhausted",
		Limits:      []provider_quota.QuotaLimitStatus{provider_quota.NewTokenLimitStatus(status, maxUsage, nextReset)},
		NextResetAt: nextReset,
	}
}

func deriveNanoGPT(fields []ParsedField) QuotaDerivedStatus {
	var maxPct float64
	for _, f := range fields {
		if f.Format == "percentage" && f.Percent > maxPct {
			maxPct = f.Percent / 100.0
		}
	}

	// Check state field
	for _, f := range fields {
		if f.Key == "state" {
			if val, ok := f.Value.(string); ok {
				if val == "inactive" {
					return QuotaDerivedStatus{
						Status: "exhausted",
						Ready:  false,
						Limits: []provider_quota.QuotaLimitStatus{
							provider_quota.NewTokenLimitStatus("exhausted", maxPct, nil),
						},
					}
				}
				if val == "grace" {
					return QuotaDerivedStatus{
						Status: "warning",
						Ready:  true,
						Limits: []provider_quota.QuotaLimitStatus{
							provider_quota.NewTokenLimitStatus("warning", maxPct, nil),
						},
					}
				}
			}
		}
	}

	status := "available"
	if maxPct >= warningThreshold {
		status = "warning"
	}

	return QuotaDerivedStatus{
		Status: status,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, maxPct, nil),
		},
	}
}

func deriveWafer(fields []ParsedField) QuotaDerivedStatus {
	pct := findFieldPercent(fields, "used_pct") / 100.0
	remaining := findFieldNumber(fields, "remaining")

	status := "available"
	if remaining == 0 && pct > 0 {
		status = "exhausted"
	} else if pct >= warningThreshold {
		status = "warning"
	}

	return QuotaDerivedStatus{
		Status: status,
		Ready:  status != "exhausted",
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, pct, nil),
		},
	}
}

func deriveSynthetic(fields []ParsedField) QuotaDerivedStatus {
	pct := findFieldPercent(fields, "weekly_remaining_pct")
	usedPct := 1.0 - pct/100.0

	status := "available"
	if usedPct >= 1.0 {
		status = "exhausted"
	} else if usedPct >= warningThreshold {
		status = "warning"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status: status,
		Ready:  status != "exhausted",
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, usedPct, nextReset),
		},
		NextResetAt: nextReset,
	}
}

func deriveNeuralWatt(fields []ParsedField) QuotaDerivedStatus {
	var kwhUsed, kwhIncluded float64
	for _, f := range fields {
		switch f.Key {
		case "kwh_used":
			kwhUsed, _ = toFloat(f.Value)
		case "kwh_included":
			kwhIncluded, _ = toFloat(f.Value)
		}
	}

	var ratio float64
	if kwhIncluded > 0 {
		ratio = kwhUsed / kwhIncluded
	}

	status := "available"
	if ratio >= 1.0 {
		status = "exhausted"
	} else if ratio >= warningThreshold {
		status = "warning"
	}

	return QuotaDerivedStatus{
		Status: status,
		Ready:  status != "exhausted",
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, ratio, nil),
		},
	}
}

func deriveApertis(fields []ParsedField) QuotaDerivedStatus {
	cycleLimit := findFieldNumber(fields, "cycle_limit")
	if cycleLimit == 0 {
		cycleLimit = findFieldTotal(fields, "cycle_used")
	}
	isSubscriber := findFieldBool(fields, "is_subscriber") || findFieldString(fields, "subscription_status") != "" || cycleLimit > 0
	var limits []provider_quota.QuotaLimitStatus
	var bestStatus string

	if isSubscriber {
		subStatus := findFieldString(fields, "subscription_status")
		cycleUsed := findFieldNumber(fields, "cycle_used")
		cycleRemaining := findFieldNumber(fields, "cycle_remaining")
		usageRatio := 0.0
		if cycleLimit > 0 {
			usageRatio = cycleUsed / cycleLimit
		}

		status := "available"
		if subStatus == "suspended" || subStatus == "cancelled" || (cycleLimit > 0 && cycleRemaining <= 0) {
			status = "exhausted"
		} else if usageRatio >= warningThreshold {
			status = "warning"
		} else if cycleLimit <= 0 {
			status = "unknown"
		}

		limits = append(limits, provider_quota.QuotaLimitStatus{
			Type:       provider_quota.QuotaLimitTypeSubscriptionCycle,
			Status:     status,
			UsageRatio: usageRatio,
			Ready:      provider_quota.IsReadyStatus(status),
		})
		bestStatus = status
	}

	accountCredits := findFieldNumber(fields, "account_credits")
	tokenTotal := findFieldNumber(fields, "token_total")
	if tokenTotal == 0 {
		tokenTotal = findFieldTotal(fields, "token_used")
	}
	tokenUsed := findFieldNumber(fields, "token_used")
	tokenUnlimited := findFieldBool(fields, "token_is_unlimited")
	hasPayg := tokenUnlimited || accountCredits > 0 || tokenTotal > 0 || tokenUsed > 0
	if hasPayg && (!isSubscriber || findFieldBool(fields, "payg_fallback_enabled") || bestStatus == "exhausted" || bestStatus == "") {
		status := "available"
		usageRatio := 0.0
		if tokenUnlimited {
			status = "available"
		} else if accountCredits <= 0 {
			status = "exhausted"
		} else if tokenTotal > 0 {
			usageRatio = tokenUsed / tokenTotal
			if usageRatio >= 1.0 {
				status = "exhausted"
			} else if usageRatio >= warningThreshold {
				status = "warning"
			}
		}

		limits = append(limits, provider_quota.QuotaLimitStatus{
			Type:       provider_quota.QuotaLimitTypeToken,
			Status:     status,
			UsageRatio: usageRatio,
			Ready:      provider_quota.IsReadyStatus(status),
		})
		bestStatus = betterQuotaStatus(bestStatus, status)
	}

	if bestStatus == "" {
		bestStatus = "unknown"
	}

	return QuotaDerivedStatus{
		Status:      bestStatus,
		Ready:       provider_quota.IsReadyStatus(bestStatus),
		Limits:      limits,
		NextResetAt: earliestDatetime(fields),
	}
}

func deriveZhipu(fields []ParsedField) QuotaDerivedStatus {
	var limits []provider_quota.QuotaLimitStatus
	var worstStatus string

	for _, f := range fields {
		if f.Format != "percentage" {
			continue
		}

		pct := f.Percent / 100.0
		limitStatus := "available"
		if pct >= 1.0 {
			limitStatus = "exhausted"
		} else if pct >= warningThreshold {
			limitStatus = "warning"
		}

		var limitType provider_quota.QuotaLimitType
		switch f.Key {
		case "token_pct":
			limitType = provider_quota.QuotaLimitTypeToken
		case "time_pct":
			limitType = provider_quota.QuotaLimitTypeTime
		default:
			continue
		}

		var nextReset *time.Time
		switch f.Key {
		case "token_pct":
			nextReset = findFieldTime(fields, "token_reset")
		case "time_pct":
			nextReset = findFieldTime(fields, "time_reset")
		}

		limits = append(limits, provider_quota.QuotaLimitStatus{
			Type:        limitType,
			Status:      limitStatus,
			UsageRatio:  pct,
			Ready:       limitStatus != "exhausted",
			NextResetAt: nextReset,
		})

		if worstStatus == "" || quotaStatusRank(limitStatus) > quotaStatusRank(worstStatus) {
			worstStatus = limitStatus
		}
	}

	if worstStatus == "" {
		worstStatus = "unknown"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status:      worstStatus,
		Ready:       worstStatus != "exhausted",
		Limits:      limits,
		NextResetAt: nextReset,
	}
}

func deriveAntigravity(fields []ParsedField) QuotaDerivedStatus {
	// remaining_fraction is 0-1 where 1=full, 0=exhausted. Invert for usage ratio.
	remaining := 1.0
	for _, f := range fields {
		if f.Key == "bucket_0_remaining_fraction" {
			if v, err := toFloat(f.Value); err == nil {
				remaining = v
			}
		}
	}
	used := 1.0 - remaining

	status := "available"
	if used >= 1.0 {
		status = "exhausted"
	} else if used >= warningThreshold {
		status = "warning"
	}

	nextReset := findFieldTime(fields, "bucket_0_reset_time")
	return QuotaDerivedStatus{
		Status: status,
		Ready:  status != "exhausted",
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, used, nextReset),
		},
		NextResetAt: nextReset,
	}
}

func deriveMinimax(fields []ParsedField) QuotaDerivedStatus {
	var limits []provider_quota.QuotaLimitStatus
	worstStatus := "available"

	for _, key := range []string{"interval", "weekly"} {
		if !hasField(fields, key+"_used_pct") {
			continue
		}

		ratio := findFieldPercent(fields, key+"_used_pct") / 100.0
		status := "available"
		if ratio >= 1.0 {
			status = "exhausted"
		} else if ratio >= warningThreshold {
			status = "warning"
		}
		reset := findFieldTime(fields, key+"_reset")
		limits = append(limits, provider_quota.NewTokenLimitStatus(status, ratio, reset))
		if quotaStatusRank(status) > quotaStatusRank(worstStatus) {
			worstStatus = status
		}
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status:      worstStatus,
		Ready:       worstStatus != "exhausted",
		Limits:      limits,
		NextResetAt: nextReset,
	}
}

// deriveOpenCodeGo derives quota status from OpenCode Go dashboard usage windows
// (rolling/weekly/monthly). Each window contributes a percentage field
// (<window>_used_pct, 0-100) and a datetime reset field (<window>_reset).
func deriveOpenCodeGo(fields []ParsedField) QuotaDerivedStatus {
	var limits []provider_quota.QuotaLimitStatus
	var worstStatus string

	for _, key := range []string{"rolling", "weekly", "monthly"} {
		if !hasField(fields, key+"_used_pct") {
			// The converter only emits fields for windows present in the
			// dashboard response, so absence means "no data for this window".
			continue
		}
		ratio := findFieldPercent(fields, key+"_used_pct") / 100.0

		status := "available"
		if ratio >= 1.0 {
			status = "exhausted"
		} else if ratio >= warningThreshold {
			status = "warning"
		}

		nextReset := findFieldTime(fields, key+"_reset")
		limits = append(limits, provider_quota.QuotaLimitStatus{
			Type:        provider_quota.QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  ratio,
			Ready:       status != "exhausted",
			NextResetAt: nextReset,
		})

		if worstStatus == "" || quotaStatusRank(status) > quotaStatusRank(worstStatus) {
			worstStatus = status
		}
	}

	if worstStatus == "" {
		worstStatus = "unknown"
	}

	return QuotaDerivedStatus{
		Status:      worstStatus,
		Ready:       worstStatus != "exhausted",
		Limits:      limits,
		NextResetAt: earliestDatetime(fields),
	}
}

func deriveCline(fields []ParsedField) QuotaDerivedStatus {
	scope := findFieldString(fields, "model_scope")
	var limits []provider_quota.QuotaLimitStatus
	worstStatus := "unknown"

	for _, key := range []string{"last5h", "last7d", "last30d"} {
		if !hasField(fields, key+"_used_pct") {
			continue
		}
		ratio := findFieldPercent(fields, key+"_used_pct") / 100.0
		status := "available"
		if ratio >= 1.0 {
			status = "exhausted"
		} else if ratio >= warningThreshold {
			status = "warning"
		}

		// Pass exhaustion must not disable channels which can still route direct
		// credit models. Unknown scope is deliberately treated the same way.
		if scope != "cline_pass_only" && status == "exhausted" {
			status = "warning"
		}

		nextReset := findFieldTime(fields, key+"_reset")
		limits = append(limits, provider_quota.QuotaLimitStatus{
			Type:        provider_quota.QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  ratio,
			Ready:       status != "exhausted",
			NextResetAt: nextReset,
		})
		if quotaStatusRank(status) > quotaStatusRank(worstStatus) {
			worstStatus = status
		}
	}

	if scope == "direct_only" || len(limits) == 0 {
		return QuotaDerivedStatus{Status: "available", Ready: true, NextResetAt: earliestDatetime(fields)}
	}

	return QuotaDerivedStatus{
		Status:      worstStatus,
		Ready:       worstStatus != "exhausted",
		Limits:      limits,
		NextResetAt: earliestDatetime(fields),
	}
}

func quotaStatusRank(s string) int {
	switch s {
	case "available":
		return 0
	case "warning":
		return 1
	case "exhausted":
		return 2
	default:
		return -1
	}
}

func findFieldPercent(fields []ParsedField, key string) float64 {
	for _, f := range fields {
		if f.Key == key {
			return f.Percent
		}
	}
	return 0
}

func findFieldNumber(fields []ParsedField, key string) float64 {
	for _, f := range fields {
		if f.Key == key {
			v, _ := toFloat(f.Value)
			return v
		}
	}
	return 0
}

func findFieldTotal(fields []ParsedField, key string) float64 {
	for _, f := range fields {
		if f.Key == key {
			v, _ := toFloat(f.Total)
			return v
		}
	}
	return 0
}

func findFieldTime(fields []ParsedField, key string) *time.Time {
	for _, f := range fields {
		if f.Key == key && f.Format == "datetime" {
			if s, ok := f.Value.(string); ok {
				if t, err := time.Parse(time.RFC3339, s); err == nil {
					return &t
				}
			}
		}
	}
	return nil
}

func findFieldBool(fields []ParsedField, key string) bool {
	for _, f := range fields {
		if f.Key != key {
			continue
		}
		if b, ok := f.Value.(bool); ok {
			return b
		}
		if s, ok := f.Value.(string); ok {
			return s == "true"
		}
	}
	return false
}

func betterQuotaStatus(a, b string) string {
	if a == "" {
		return b
	}
	if quotaStatusRank(b) < quotaStatusRank(a) {
		return b
	}
	return a
}

func findFieldString(fields []ParsedField, key string) string {
	for _, f := range fields {
		if f.Key == key {
			if s, ok := f.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

func hasField(fields []ParsedField, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

func earliestDatetime(fields []ParsedField) *time.Time {
	var earliest *time.Time
	for _, f := range fields {
		if f.Format != "datetime" || f.Value == nil {
			continue
		}
		s, ok := f.Value.(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			continue
		}
		if t.After(time.Now()) && (earliest == nil || t.Before(*earliest)) {
			earliest = &t
		}
	}
	return earliest
}

// deriveGeneric derives quota status from any percentage or fraction display fields.
// It collects all numeric utilization ratios and applies the standard 0.8/1.0 thresholds.
// For fraction fields with a computed percent (value/total), it uses percent/100;
// for fraction fields without percent (pure ratio 0-1), it uses value directly.
// This enables quota indicators for custom/generic templates without a hardcoded
// provider-specific derivation function.
func deriveGeneric(fields []ParsedField) QuotaDerivedStatus {
	var maxRatio float64
	var hasNumericField bool

	for _, f := range fields {
		switch f.Format {
		case "percentage":
			if f.Percent > 0 {
				maxRatio = max(maxRatio, f.Percent/100.0)
				hasNumericField = true
			}
		case "fraction":
			// Prefer computed percent (value/total) over raw value.
			// Fraction fields with a total produce percent via computePercent;
			// those without represent a pure 0-1 ratio.
			if f.Percent > 0 {
				maxRatio = max(maxRatio, f.Percent/100.0)
				hasNumericField = true
			} else if v, err := toFloat(f.Value); err == nil && v > 0 {
				maxRatio = max(maxRatio, v)
				hasNumericField = true
			}
		}
	}

	if !hasNumericField {
		return QuotaDerivedStatus{Status: "unknown", Ready: true}
	}

	status := "available"
	if maxRatio >= 1.0 {
		status = "exhausted"
	} else if maxRatio >= warningThreshold {
		status = "warning"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status: status,
		Ready:  status != "exhausted",
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, maxRatio, nextReset),
		},
		NextResetAt: nextReset,
	}
}

// deriveXaiSubscription derives routing status from the xAI subscription
// billing windows emitted by XaiSubscriptionQuotaToParsedFields. The worst
// window status wins, matching the checker's own aggregation.
func deriveXaiSubscription(fields []ParsedField) QuotaDerivedStatus {
	var limits []provider_quota.QuotaLimitStatus
	worstStatus := ""
	var nextReset *time.Time

	for _, key := range []string{"weekly", "monthly"} {
		if !hasField(fields, key+"_used_pct") {
			continue
		}
		ratio := findFieldPercent(fields, key+"_used_pct") / 100.0

		status := "available"
		if ratio >= 1.0 {
			status = "exhausted"
		} else if ratio >= warningThreshold {
			status = "warning"
		}
		reset := findFieldTime(fields, key+"_reset")
		limits = append(limits, provider_quota.NewTokenLimitStatus(status, ratio, reset))
		if nextReset == nil || (reset != nil && reset.Before(*nextReset)) {
			nextReset = reset
		}
		if worstStatus == "" || quotaStatusRank(status) > quotaStatusRank(worstStatus) {
			worstStatus = status
		}
	}

	if worstStatus == "" {
		return QuotaDerivedStatus{Status: "unknown", Ready: false}
	}

	return QuotaDerivedStatus{
		Status:      worstStatus,
		Ready:       worstStatus != "exhausted",
		Limits:      limits,
		NextResetAt: nextReset,
	}
}

// deriveCharmHyper derives routing status from the credit balance emitted by
// CharmHyperQuotaToParsedFields. Balance == 0 is exhausted, balance <= 20 is
// warning, otherwise available; matches the checker's computeStatus thresholds.
func deriveCharmHyper(fields []ParsedField) QuotaDerivedStatus {
	const (
		charmHyperBaseline         = 100.0
		charmHyperWarningThreshold = 20.0
	)

	if !hasField(fields, "balance") {
		return QuotaDerivedStatus{Status: "unknown", Ready: false}
	}

	balance := findFieldNumber(fields, "balance")

	usageRatio := 1.0 - balance/charmHyperBaseline
	if usageRatio < 0 {
		usageRatio = 0
	}

	status := "available"
	ready := true
	if balance == 0 {
		status = "exhausted"
		ready = false
		usageRatio = 1.0
	} else if balance <= charmHyperWarningThreshold {
		status = "warning"
	}

	return QuotaDerivedStatus{
		Status: status,
		Ready:  ready,
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, usageRatio, nil),
		},
	}
}
