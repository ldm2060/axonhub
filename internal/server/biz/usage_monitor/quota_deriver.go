package usage_monitor

import (
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
	case "zhipu":
		return deriveZhipu(fields)
	case "antigravity":
		return deriveAntigravity(fields)
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
	pct := findFieldPercent(fields, "primary_used_pct")
	secondaryPct := findFieldPercent(fields, "secondary_used_pct")
	// Use the worse of primary and secondary windows
	if secondaryPct > pct {
		pct = secondaryPct
	}

	status := "available"
	if pct >= 1.0 {
		status = "exhausted"
	} else if pct >= warningThreshold {
		status = "warning"
	}

	nextReset := earliestDatetime(fields)
	return QuotaDerivedStatus{
		Status: status,
		Ready:  status != "exhausted",
		Limits: []provider_quota.QuotaLimitStatus{
			provider_quota.NewTokenLimitStatus(status, pct, nextReset),
		},
		NextResetAt: nextReset,
	}
}

func deriveGitHubCopilot(fields []ParsedField) QuotaDerivedStatus {
	// GitHub Copilot template only has text fields (plan, access_type), no percentage data
	return QuotaDerivedStatus{Status: "available", Ready: true}
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
	pct := findFieldPercent(fields, "used_pct")
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
			provider_quota.NewTokenLimitStatus(status, pct/100.0, nil),
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
