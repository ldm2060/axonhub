package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func TestCalculateMaxUsageRatio(t *testing.T) {
	tests := []struct {
		name     string
		limits   []provider_quota.QuotaLimitStatus
		expected float64
	}{
		{
			name:     "empty limits",
			limits:   []provider_quota.QuotaLimitStatus{},
			expected: 0.0,
		},
		{
			name:     "nil limits",
			limits:   nil,
			expected: 0.0,
		},
		{
			name: "single limit",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 0.75},
			},
			expected: 0.75,
		},
		{
			name: "multiple limits - max in middle",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 0.50},
				{UsageRatio: 0.95},
				{UsageRatio: 0.30},
			},
			expected: 0.95,
		},
		{
			name: "multiple limits - max at end",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 0.30},
				{UsageRatio: 0.50},
				{UsageRatio: 0.98},
			},
			expected: 0.98,
		},
		{
			name: "ratio exactly at disable threshold",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 0.95},
			},
			expected: 0.95,
		},
		{
			name: "ratio at 100%",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 1.0},
			},
			expected: 1.0,
		},
		{
			name: "ratio above 100%",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 1.2},
			},
			expected: 1.2,
		},
		{
			name: "all zeros",
			limits: []provider_quota.QuotaLimitStatus{
				{UsageRatio: 0.0},
				{UsageRatio: 0.0},
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMaxUsageRatio(tt.limits)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateQuotaReady(t *testing.T) {
	tests := []struct {
		name             string
		currentReady     bool
		ratio            float64
		disableThreshold float64
		enableThreshold  float64
		expectedNewReady bool
		description      string
	}{
		// Ready → stays ready (below disable threshold)
		{
			name:             "ready, ratio below disable threshold",
			currentReady:     true,
			ratio:            0.80,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: true,
			description:      "should remain ready",
		},
		{
			name:             "ready, ratio at enable threshold",
			currentReady:     true,
			ratio:            0.80,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: true,
			description:      "should remain ready",
		},
		// Ready → becomes not ready (at or above disable threshold)
		{
			name:             "ready, ratio exactly at disable threshold",
			currentReady:     true,
			ratio:            0.95,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should disable",
		},
		{
			name:             "ready, ratio above disable threshold",
			currentReady:     true,
			ratio:            0.98,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should disable",
		},
		{
			name:             "ready, ratio at 100%",
			currentReady:     true,
			ratio:            1.0,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should disable",
		},
		// Not ready → stays not ready (at or above enable threshold)
		{
			name:             "not ready, ratio at enable threshold",
			currentReady:     false,
			ratio:            0.80,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should remain disabled (hysteresis)",
		},
		{
			name:             "not ready, ratio above enable threshold",
			currentReady:     false,
			ratio:            0.85,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should remain disabled",
		},
		{
			name:             "not ready, ratio below disable but above enable",
			currentReady:     false,
			ratio:            0.90,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should remain disabled (in hysteresis zone)",
		},
		// Not ready → becomes ready (below enable threshold)
		{
			name:             "not ready, ratio below enable threshold",
			currentReady:     false,
			ratio:            0.75,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: true,
			description:      "should enable",
		},
		{
			name:             "not ready, ratio at zero",
			currentReady:     false,
			ratio:            0.0,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: true,
			description:      "should enable",
		},
		// Edge cases
		{
			name:             "ready, ratio slightly below disable threshold",
			currentReady:     true,
			ratio:            0.9499,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: true,
			description:      "should remain ready (just under threshold)",
		},
		{
			name:             "not ready, ratio slightly above enable threshold",
			currentReady:     false,
			ratio:            0.8001,
			disableThreshold: 0.95,
			enableThreshold:  0.80,
			expectedNewReady: false,
			description:      "should remain disabled (just over threshold)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateQuotaReady(tt.currentReady, tt.ratio, tt.disableThreshold, tt.enableThreshold)
			assert.Equal(t, tt.expectedNewReady, result, tt.description)
		})
	}
}

func TestGetDisableThreshold(t *testing.T) {
	globalDefault := 0.95

	tests := []struct {
		name              string
		monitorThreshold  float64
		expectedThreshold float64
		description       string
	}{
		{
			name:              "zero threshold - use global",
			monitorThreshold:  0.0,
			expectedThreshold: 0.95,
			description:       "should use global default when threshold is 0",
		},
		{
			name:              "negative threshold - use global",
			monitorThreshold:  -0.1,
			expectedThreshold: 0.95,
			description:       "should use global default when threshold is negative",
		},
		{
			name:              "valid monitor threshold",
			monitorThreshold:  0.90,
			expectedThreshold: 0.90,
			description:       "should use monitor-specific threshold",
		},
		{
			name:              "monitor threshold at 1.0",
			monitorThreshold:  1.0,
			expectedThreshold: 1.0,
			description:       "should use monitor-specific threshold",
		},
		{
			name:              "monitor threshold above 1.0",
			monitorThreshold:  1.5,
			expectedThreshold: 1.5,
			description:       "should use monitor-specific threshold even if > 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := &ent.UsageMonitorChannel{
				AutoDisableThreshold: tt.monitorThreshold,
			}
			result := getDisableThreshold(monitor, globalDefault)
			assert.Equal(t, tt.expectedThreshold, result, tt.description)
		})
	}
}

func TestGetEnableThreshold(t *testing.T) {
	globalDefault := 0.80

	tests := []struct {
		name              string
		monitorThreshold  float64
		expectedThreshold float64
		description       string
	}{
		{
			name:              "zero threshold - use global",
			monitorThreshold:  0.0,
			expectedThreshold: 0.80,
			description:       "should use global default when threshold is 0",
		},
		{
			name:              "negative threshold - use global",
			monitorThreshold:  -0.1,
			expectedThreshold: 0.80,
			description:       "should use global default when threshold is negative",
		},
		{
			name:              "valid monitor threshold",
			monitorThreshold:  0.70,
			expectedThreshold: 0.70,
			description:       "should use monitor-specific threshold",
		},
		{
			name:              "monitor threshold at 1.0",
			monitorThreshold:  1.0,
			expectedThreshold: 1.0,
			description:       "should use monitor-specific threshold",
		},
		{
			name:              "very low monitor threshold",
			monitorThreshold:  0.01,
			expectedThreshold: 0.01,
			description:       "should use monitor-specific threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := &ent.UsageMonitorChannel{
				AutoEnableThreshold: tt.monitorThreshold,
			}
			result := getEnableThreshold(monitor, globalDefault)
			assert.Equal(t, tt.expectedThreshold, result, tt.description)
		})
	}
}
