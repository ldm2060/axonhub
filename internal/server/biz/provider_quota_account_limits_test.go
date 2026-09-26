package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

// Regression: the per-account limits of a multi-key channel have to survive the
// JSON round trip. Without the availability group a restart would AND the
// accounts together and drop the channel from routing even though one key still
// has quota.
func TestMergeAndExtractLimitsKeepsAccountAlternatives(t *testing.T) {
	svc := &ProviderQuotaService{}
	resetAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	quotaData := provider_quota.QuotaData{
		Status:       "available",
		ProviderType: "zhipu",
		Limits: []provider_quota.QuotaLimitStatus{
			{
				Type: provider_quota.QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1,
				Window: "weekly", Account: "0001", AvailabilityGroup: "zhipu_accounts", NextResetAt: &resetAt,
			},
			{
				Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.31, Ready: true,
				Window: "weekly", Account: "0002", AvailabilityGroup: "zhipu_accounts", NextResetAt: &resetAt,
			},
		},
	}

	extracted := extractLimitsFromQuotaData(svc.mergeLimitsIntoQuotaData(quotaData))

	assert.Len(t, extracted, 2)
	assert.Equal(t, "0001", extracted[0].Account)
	assert.Equal(t, "0002", extracted[1].Account)
	assert.Equal(t, "zhipu_accounts", extracted[0].AvailabilityGroup)
	assert.Equal(t, "zhipu_accounts", extracted[1].AvailabilityGroup)

	status, ready := provider_quota.EffectiveStatus(extracted, "available", true, provider_quota.QuotaLimitTypeToken)
	assert.Equal(t, "available", string(status))
	assert.True(t, ready)
}
