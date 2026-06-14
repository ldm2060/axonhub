package biz

import (
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// calculateMaxUsageRatio returns the maximum usage ratio from a list of quota limits.
// Returns 0.0 if limits is empty.
func calculateMaxUsageRatio(limits []provider_quota.QuotaLimitStatus) float64 {
	maxRatio := 0.0
	for _, limit := range limits {
		if limit.UsageRatio > maxRatio {
			maxRatio = limit.UsageRatio
		}
	}
	return maxRatio
}

// evaluateQuotaReady determines the new ready state based on current state and usage ratio.
// Implements hysteresis to prevent flapping:
// - If currently ready: disable if ratio >= disableThreshold
// - If currently not ready: enable if ratio < enableThreshold
// - Otherwise: maintain current state.
func evaluateQuotaReady(currentReady bool, ratio float64, disableThreshold float64, enableThreshold float64) bool {
	if currentReady {
		// Currently ready → check if should disable
		if ratio >= disableThreshold {
			return false // Disable
		}
	} else {
		// Currently not ready → check if should enable
		if ratio < enableThreshold {
			return true // Enable
		}
	}
	return currentReady // No change
}

// getDisableThreshold returns the effective disable threshold for a monitor.
// Uses monitor-specific threshold if > 0, otherwise falls back to globalDefault.
// Note: AutoDisableThreshold is Optional but not Nillable, so it defaults to 0.0 if not set.
func getDisableThreshold(monitor *ent.UsageMonitorChannel, globalDefault float64) float64 {
	if monitor.AutoDisableThreshold > 0 {
		return monitor.AutoDisableThreshold
	}
	return globalDefault
}

// getEnableThreshold returns the effective enable threshold for a monitor.
// Uses monitor-specific threshold if > 0, otherwise falls back to globalDefault.
// Note: AutoEnableThreshold is Optional but not Nillable, so it defaults to 0.0 if not set.
func getEnableThreshold(monitor *ent.UsageMonitorChannel, globalDefault float64) float64 {
	if monitor.AutoEnableThreshold > 0 {
		return monitor.AutoEnableThreshold
	}
	return globalDefault
}
