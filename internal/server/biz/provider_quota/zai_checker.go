package provider_quota

import (
	"context"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

const zaiDefaultQuotaBaseURL = "https://api.z.ai"

type ZaiQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewZaiQuotaChecker(httpClient *httpclient.HttpClient) *ZaiQuotaChecker {
	return &ZaiQuotaChecker{httpClient: httpClient}
}

func (c *ZaiQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	return collectZhipuFamilyQuota(ctx, c.httpClient, ch, "zai", zaiDefaultQuotaBaseURL+"/api/monitor/usage/quota/limit")
}

func (c *ZaiQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeZai ||
		ch.Type == channel.TypeZaiAnthropic
}

// parseZaiQuotaResponse parses a single-account Z.ai payload.
func parseZaiQuotaResponse(body []byte) (QuotaData, error) {
	return parseZhipuFamilyQuotaResponse(body, "zai")
}
