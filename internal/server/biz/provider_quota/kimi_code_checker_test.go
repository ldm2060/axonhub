package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

func TestKimiCode_CheckQuota(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://api.kimi.com/coding/v1/usages", req.URL.String())
			require.Equal(t, "Bearer kimi-token", req.Header.Get("Authorization"))

			body := `{
				"usage":{"name":"Weekly limit","used":80,"limit":100,"resetAt":"2099-07-20T00:00:00.123456Z"},
				"limits":[
					{"detail":{"remaining":"0","limit":"20"},"window":{"duration":300,"timeUnit":"MINUTE"}},
					{"detail":{"used":1,"limit":10,"title":"Daily limit"}}
				],
				"boosterWallet":{
					"balance":{"type":"BOOSTER","amount":1250000000,"amountLeft":250000000},
					"monthlyChargeLimitEnabled":true,
					"monthlyChargeLimit":{"priceInCents":5000,"currency":"USD"},
					"monthlyUsed":{"priceInCents":1234,"currency":"USD"}
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewKimiCodeQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeMoonshotCoding,
		BaseURL:     "https://api.kimi.com/coding",
		Credentials: objects.ChannelCredentials{APIKey: " kimi-token "},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Equal(t, "kimi_code", quota.ProviderType)
	require.Len(t, quota.Limits, 3)
	require.InDelta(t, 0.8, quota.Limits[0].UsageRatio, 0.001)
	require.InDelta(t, 1, quota.Limits[1].UsageRatio, 0.001)
	require.Equal(t, time.Date(2099, 7, 20, 0, 0, 0, 123456000, time.UTC), *quota.NextResetAt)

	rows, ok := quota.RawData["rows"].([]kimiCodeUsageRow)
	require.True(t, ok)
	require.Equal(t, "5h limit", rows[1].Label)
	require.Equal(t, int64(20), rows[1].Used)
	wallet, ok := quota.RawData["boosterWallet"].(kimiCodeBoosterWallet)
	require.True(t, ok)
	require.Equal(t, int64(250), wallet.BalanceCents)
	require.Equal(t, int64(1250), wallet.TotalCents)
}

func TestBuildKimiCodeUsageURL(t *testing.T) {
	require.Equal(t, "https://api.kimi.com/coding/v1/usages", buildKimiCodeUsageURL(""))
	require.Equal(t, "https://example.com/kimi/v1/usages", buildKimiCodeUsageURL("https://example.com/kimi"))
	require.Equal(t, "https://example.com/kimi/v1/usages", buildKimiCodeUsageURL("https://example.com/kimi/v1/"))
	require.Equal(t, "https://api.kimi.com/coding/v1/usages", buildKimiCodeUsageURL("not a URL"))
}

func TestKimiCode_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewKimiCodeQuotaChecker(nil)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeMoonshotCoding})
	require.ErrorContains(t, err, "channel has no API key")
}

// TestParseKimiCodeUsageResponseCurrentShape covers the ratio-based usages
// payload served by the current Kimi Code backend.
func TestParseKimiCodeUsageResponseCurrentShape(t *testing.T) {
	body := `{
		"usages":{
			"limit_5h":{"used_ratio":0.23,"reset_time":"2099-07-20T12:00:00Z"},
			"limit_7d":{"used_ratio":"0.42","reset_time":"2099-07-24T00:00:00Z"},
			"limit_month_total":{"used_ratio":0.95,"reset_time":"2099-07-31T00:00:00Z"},
			"limit_month_code":{"used_ratio":1.05,"reset_time":"2099-07-31T00:00:00Z"}
		},
		"boosterWallet":{
			"balance":{"type":"BOOSTER","amount":1250000000,"amountLeft":250000000},
			"monthlyChargeLimitEnabled":true,
			"monthlyChargeLimit":{"priceInCents":5000,"currency":"USD"},
			"monthlyUsed":{"priceInCents":1234,"currency":"USD"}
		}
	}`
	quota, err := parseKimiCodeUsageResponse([]byte(body))
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.Equal(t, "kimi_code", quota.ProviderType)
	// The two monthly windows share the 30d window key, so normalization
	// collapses them into the worse one and caps the ratio at 1.
	require.Len(t, quota.Limits, 3)

	require.InDelta(t, 0.23, quota.Limits[0].UsageRatio, 0.001)
	require.Equal(t, "5h", quota.Limits[0].Window)
	require.InDelta(t, 0.42, quota.Limits[1].UsageRatio, 0.001)
	require.Equal(t, "7d", quota.Limits[1].Window)
	require.InDelta(t, 1, quota.Limits[2].UsageRatio, 0.001)
	require.Equal(t, "30d", quota.Limits[2].Window)
	require.Equal(t, "exhausted", quota.Limits[2].Status)

	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, time.Date(2099, 7, 20, 12, 0, 0, 0, time.UTC), *quota.NextResetAt)

	rows, ok := quota.RawData["rows"].([]kimiCodeUsageRow)
	require.True(t, ok)
	require.Len(t, rows, 4)
	require.Equal(t, "5h limit", rows[0].Label)
	require.Equal(t, int64(23), rows[0].Used)
	require.Equal(t, int64(100), rows[0].Limit)
	wallet, ok := quota.RawData["boosterWallet"].(kimiCodeBoosterWallet)
	require.True(t, ok)
	require.Equal(t, int64(250), wallet.BalanceCents)
}

// TestParseKimiCodeUsageResponseIgnoresUnknownEntries verifies unknown usage
// keys and malformed ratios are skipped instead of failing the whole payload.
func TestParseKimiCodeUsageResponseIgnoresUnknownEntries(t *testing.T) {
	body := `{"usages":{"limit_5h":{"used_ratio":0.5},"future_window":{"used_ratio":0.1},"limit_7d":{"used_ratio":"not-a-number"}}}`
	quota, err := parseKimiCodeUsageResponse([]byte(body))
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "5h", quota.Limits[0].Window)
}
