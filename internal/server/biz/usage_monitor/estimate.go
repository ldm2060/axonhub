package usage_monitor

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// Estimate window values accepted for a monitor's estimate config. Each maps
// onto a window label the frontend renders via quota.window.*.
const (
	EstimateWindow5h      = "5h"
	EstimateWindowDaily   = "daily"
	EstimateWindowWeekly  = "weekly"
	EstimateWindowMonthly = "monthly"
)

// EstimateConfig drives one entry of a monitor's period quota estimate.
// FieldKey selects which parsed field reports the usage ratio; when set
// (custom monitors, and builtin monitors that pick a field) the entry
// contributes one windowed limit priced from that field. When FieldKey is
// empty and only Window is set (template monitors), the window is stamped
// onto the template's windowless limits without changing their ratios.
// Remaining marks the field as expressing remaining quota (inverted to used);
// ResetFieldKey picks the datetime field marking the reset (empty = earliest
// future datetime). Providers commonly expose several windows at once (a 5h
// rolling one and a weekly pool), so a monitor carries a list of entries.
type EstimateConfig struct {
	FieldKey      string `json:"fieldKey"`
	Remaining     bool   `json:"remaining,omitempty"`
	ResetFieldKey string `json:"resetFieldKey,omitempty"`
	Window        string `json:"window"`
}

// EstimateConfigsFromMap decodes the estimate_config JSON column. The column
// stores {"items": [...]} going forward; a legacy flat single-config object
// (before multi-entry support) still decodes to a one-element list. A nil map
// or one without any window/field decodes to nil — no estimate configured.
func EstimateConfigsFromMap(m map[string]any) ([]EstimateConfig, error) {
	if len(m) == 0 {
		return nil, nil
	}
	decode := func(raw any) ([]EstimateConfig, error) {
		bytes, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to encode estimate config: %w", err)
		}
		var cfgs []EstimateConfig
		if err := json.Unmarshal(bytes, &cfgs); err != nil {
			return nil, fmt.Errorf("failed to decode estimate config: %w", err)
		}
		return cfgs, nil
	}

	if items, ok := m["items"]; ok {
		cfgs, err := decode(items)
		if err != nil {
			return nil, err
		}
		return nonEmptyConfigs(cfgs), nil
	}
	// Legacy single-config object.
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to encode estimate config: %w", err)
	}
	var cfg EstimateConfig
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode estimate config: %w", err)
	}
	return nonEmptyConfigs([]EstimateConfig{cfg}), nil
}

// nonEmptyConfigs drops entries that carry neither a field nor a window.
func nonEmptyConfigs(cfgs []EstimateConfig) []EstimateConfig {
	out := make([]EstimateConfig, 0, len(cfgs))
	for _, cfg := range cfgs {
		if cfg.FieldKey == "" && cfg.Window == "" {
			continue
		}
		out = append(out, cfg)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EstimateConfigsToMap encodes the config list for the estimate_config JSON
// column in the {"items": [...]} storage shape.
func EstimateConfigsToMap(cfgs []EstimateConfig) map[string]any {
	items := make([]map[string]any, 0, len(cfgs))
	for _, cfg := range cfgs {
		entry := map[string]any{
			"fieldKey": cfg.FieldKey,
			"window":   cfg.Window,
		}
		if cfg.Remaining {
			entry["remaining"] = true
		}
		if cfg.ResetFieldKey != "" {
			entry["resetFieldKey"] = cfg.ResetFieldKey
		}
		items = append(items, entry)
	}
	return map[string]any{"items": items}
}

// ValidateEstimateConfigs checks every submitted entry: the window is always
// mandatory, the usage field is optional (a window-only entry is the template
// stamp mode), and the referenced fields must exist among the monitor's
// display fields when those are known.
func ValidateEstimateConfigs(cfgs []EstimateConfig, displayFieldKeys []string) error {
	for _, cfg := range cfgs {
		switch cfg.Window {
		case EstimateWindow5h, EstimateWindowDaily, EstimateWindowWeekly, EstimateWindowMonthly:
		default:
			return fmt.Errorf("estimateConfig.window must be one of 5h, daily, weekly, monthly")
		}
		if len(displayFieldKeys) > 0 {
			if cfg.FieldKey != "" && !slices.Contains(displayFieldKeys, cfg.FieldKey) {
				return fmt.Errorf("estimateConfig.fieldKey %q is not a display field of this monitor", cfg.FieldKey)
			}
			if cfg.ResetFieldKey != "" && !slices.Contains(displayFieldKeys, cfg.ResetFieldKey) {
				return fmt.Errorf("estimateConfig.resetFieldKey %q is not a display field of this monitor", cfg.ResetFieldKey)
			}
		}
	}
	return nil
}

// windowedLimit builds the limit for one estimate window: the fixed-length
// windows derive their period start from the reset time, monthly windows from
// the reset date itself.
func windowedLimit(ratio float64, reset *time.Time, window string) (provider_quota.QuotaLimitStatus, bool) {
	switch window {
	case EstimateWindowMonthly:
		// Monthly windows reset on a calendar day, not a fixed length ago, so
		// the period start derives from the reset date itself.
		limit := provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindowMonthly, 0)
		limit.PeriodStart = provider_quota.PeriodStartFromMonthlyReset(reset)
		return limit, true
	case EstimateWindowWeekly:
		return provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindowWeekly, 7*24*time.Hour), true
	case EstimateWindowDaily:
		return provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindowDaily, 24*time.Hour), true
	case EstimateWindow5h:
		return provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindow5h, 5*time.Hour), true
	default:
		return provider_quota.QuotaLimitStatus{}, false
	}
}

// ApplyEstimateConfigs drives the period estimate for field-selection sources
// (custom/builtin): every entry contributes one windowed limit priced from its
// field's ratio, so a monitor can carry both a 5h and a weekly window at once.
// Entries whose field is missing or yields no usable ratio are skipped; when
// no entry succeeds the derived limits are left untouched. The routing status
// follows the worst contributed limit.
func ApplyEstimateConfigs(fields []ParsedField, cfgs []EstimateConfig, derived *QuotaDerivedStatus) {
	var out []provider_quota.QuotaLimitStatus
	worstStatus := ""
	var earliestReset *time.Time
	for _, cfg := range cfgs {
		if cfg.FieldKey == "" {
			continue
		}
		ratio, ok := estimateUsageRatio(fields, cfg)
		if !ok {
			continue
		}
		limit, ok := windowedLimit(ratio, estimateResetTime(fields, cfg.ResetFieldKey), cfg.Window)
		if !ok {
			continue
		}
		status := "available"
		if ratio >= 1.0 {
			status = "exhausted"
		} else if ratio >= warningThreshold {
			status = "warning"
		}
		limit.Status = status
		limit.Ready = provider_quota.IsReadyStatus(status)
		limit.FieldKey = cfg.FieldKey
		out = append(out, limit)
		if quotaStatusRank(status) > quotaStatusRank(worstStatus) {
			worstStatus = status
		}
		if limit.NextResetAt != nil && (earliestReset == nil || limit.NextResetAt.Before(*earliestReset)) {
			earliestReset = limit.NextResetAt
		}
	}
	if len(out) == 0 {
		return
	}
	derived.Limits = out
	derived.Status = worstStatus
	derived.Ready = provider_quota.IsReadyStatus(worstStatus)
	derived.NextResetAt = earliestReset
}

// StampEstimateWindows is the template-monitor path: every configured window
// is stamped onto the limits that have none, so a template reporting one ratio
// can be priced against both a 5h and a weekly window when both are
// configured. The template keeps its own ratios, statuses, and field keys;
// limits that already carry a window are left untouched.
func StampEstimateWindows(fields []ParsedField, cfgs []EstimateConfig, derived *QuotaDerivedStatus) {
	var windowless []provider_quota.QuotaLimitStatus
	var windowed []provider_quota.QuotaLimitStatus
	for _, l := range derived.Limits {
		if l.Window != "" {
			windowed = append(windowed, l)
		} else {
			windowless = append(windowless, l)
		}
	}
	if len(windowless) == 0 {
		return
	}

	var stamped []provider_quota.QuotaLimitStatus
	var earliestReset *time.Time
	for _, cfg := range cfgs {
		if cfg.Window == "" {
			continue
		}
		reset := estimateResetTime(fields, cfg.ResetFieldKey)
		for _, orig := range windowless {
			limit, ok := windowedLimit(orig.UsageRatio, reset, cfg.Window)
			if !ok {
				continue
			}
			limit.Status = orig.Status
			limit.Ready = orig.Ready
			limit.FieldKey = orig.FieldKey
			stamped = append(stamped, limit)
			if limit.NextResetAt != nil && (earliestReset == nil || limit.NextResetAt.Before(*earliestReset)) {
				earliestReset = limit.NextResetAt
			}
		}
	}
	if len(stamped) == 0 {
		return
	}
	derived.Limits = append(windowed, stamped...)
	if derived.NextResetAt == nil {
		derived.NextResetAt = earliestReset
	}
}

// estimateUsageRatio extracts the usage ratio the configured field reports,
// mirroring deriveGeneric's value handling: the computed percent for
// percentage fields and fractions with a total, the raw value otherwise.
// remaining=true means the field expresses unused quota and the ratio is
// inverted to used.
func estimateUsageRatio(fields []ParsedField, cfg EstimateConfig) (float64, bool) {
	for _, f := range fields {
		if f.Key != cfg.FieldKey {
			continue
		}

		ratio := 0.0
		switch f.Format {
		case string(FieldFormatPercentage):
			ratio = f.Percent / 100.0
		case string(FieldFormatFraction):
			if f.Percent > 0 {
				ratio = f.Percent / 100.0
			} else if v, err := toFloat(f.Value); err == nil {
				ratio = v
			}
		default:
			// Plain fields may still carry a raw 0-1 ratio.
			if v, err := toFloat(f.Value); err == nil {
				ratio = v
			}
		}

		if cfg.Remaining {
			ratio = 1 - ratio
		}
		if ratio < 0 || ratio > 1 {
			return 0, false
		}
		return ratio, true
	}
	return 0, false
}

// estimateResetTime resolves the limit's next reset: the configured datetime
// field when present, otherwise the earliest future datetime the monitor
// reports (the same default the generic derivation uses).
func estimateResetTime(fields []ParsedField, resetFieldKey string) *time.Time {
	if resetFieldKey != "" {
		if t := findFieldTime(fields, resetFieldKey); t != nil {
			return t
		}
	}
	return earliestDatetime(fields)
}
