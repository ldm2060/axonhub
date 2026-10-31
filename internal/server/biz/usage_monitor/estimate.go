package usage_monitor

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// Estimate window values accepted for a custom monitor's estimate config. Each
// maps onto a window label the frontend renders via quota.window.*.
const (
	EstimateWindow5h      = "5h"
	EstimateWindowDaily   = "daily"
	EstimateWindowWeekly  = "weekly"
	EstimateWindowMonthly = "monthly"
)

// EstimateConfig selects which parsed values drive a custom monitor's period
// quota estimate: which field reports the usage ratio, whether that field
// expresses remaining quota (inverted to used), which datetime field marks the
// reset (empty = earliest future datetime), and how long a quota window lasts.
// Without a config the generic derivation keeps its max-ratio, windowless
// limit, which no period estimate can be priced against.
type EstimateConfig struct {
	FieldKey      string `json:"fieldKey"`
	Remaining     bool   `json:"remaining,omitempty"`
	ResetFieldKey string `json:"resetFieldKey,omitempty"`
	Window        string `json:"window"`
}

// EstimateConfigFromMap decodes the estimate_config JSON column. A nil or
// keyless map decodes to a nil config, the signal that no estimate is
// configured.
func EstimateConfigFromMap(m map[string]any) (*EstimateConfig, error) {
	if len(m) == 0 {
		return nil, nil
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to encode estimate config: %w", err)
	}
	var cfg EstimateConfig
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode estimate config: %w", err)
	}
	if cfg.FieldKey == "" {
		return nil, nil
	}
	return &cfg, nil
}

// ToMap encodes the config for the estimate_config JSON column.
func (c EstimateConfig) ToMap() map[string]any {
	m := map[string]any{
		"fieldKey": c.FieldKey,
		"window":   c.Window,
	}
	if c.Remaining {
		m["remaining"] = true
	}
	if c.ResetFieldKey != "" {
		m["resetFieldKey"] = c.ResetFieldKey
	}
	return m
}

// ValidateEstimateConfig checks a submitted config: the usage field and window
// are mandatory, and the referenced fields must exist among the monitor's
// display fields when those are known.
func ValidateEstimateConfig(cfg EstimateConfig, displayFieldKeys []string) error {
	if cfg.FieldKey == "" {
		return fmt.Errorf("estimateConfig.fieldKey is required")
	}
	switch cfg.Window {
	case EstimateWindow5h, EstimateWindowDaily, EstimateWindowWeekly, EstimateWindowMonthly:
	default:
		return fmt.Errorf("estimateConfig.window must be one of 5h, daily, weekly, monthly")
	}
	if len(displayFieldKeys) > 0 {
		if !slices.Contains(displayFieldKeys, cfg.FieldKey) {
			return fmt.Errorf("estimateConfig.fieldKey %q is not a display field of this monitor", cfg.FieldKey)
		}
		if cfg.ResetFieldKey != "" && !slices.Contains(displayFieldKeys, cfg.ResetFieldKey) {
			return fmt.Errorf("estimateConfig.resetFieldKey %q is not a display field of this monitor", cfg.ResetFieldKey)
		}
	}
	return nil
}

// ApplyEstimateConfig replaces the generic max-ratio limit with one driven by
// the user-selected usage field, so the period estimate prices the window the
// user actually meant and the monitor's routing status follows that field.
// When the configured field is missing or yields no usable ratio the derived
// status is left untouched, which is the generic fallback.
func ApplyEstimateConfig(fields []ParsedField, cfg EstimateConfig, derived *QuotaDerivedStatus) {
	ratio, ok := estimateUsageRatio(fields, cfg)
	if !ok {
		return
	}
	reset := estimateResetTime(fields, cfg.ResetFieldKey)

	var limit provider_quota.QuotaLimitStatus
	switch cfg.Window {
	case EstimateWindowMonthly:
		// Monthly windows reset on a calendar day, not a fixed length ago, so
		// the period start derives from the reset date itself.
		limit = provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindowMonthly, 0)
		limit.PeriodStart = provider_quota.PeriodStartFromMonthlyReset(reset)
	case EstimateWindowWeekly:
		limit = provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindowWeekly, 7*24*time.Hour)
	case EstimateWindowDaily:
		limit = provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindowDaily, 24*time.Hour)
	case EstimateWindow5h:
		limit = provider_quota.NewTokenLimitStatus("available", ratio, reset).WithWindow(EstimateWindow5h, 5*time.Hour)
	default:
		return
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

	derived.Limits = []provider_quota.QuotaLimitStatus{limit}
	derived.Status = status
	derived.Ready = limit.Ready
	derived.NextResetAt = limit.NextResetAt
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
