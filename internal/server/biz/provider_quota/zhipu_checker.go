package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

type ZhipuAPIResponse[T any] struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    T      `json:"data"`
	Success bool   `json:"success"`
}

type ZhipuQuotaLimitItem struct {
	Type         string  `json:"type"`
	Unit         int     `json:"unit"`
	Number       int     `json:"number"`
	Usage        int     `json:"usage"`
	CurrentValue int     `json:"currentValue"`
	Remaining    int     `json:"remaining"`
	Percentage   float64 `json:"percentage"`
	NextResetTime int64  `json:"nextResetTime"`
	UsageDetails any     `json:"usageDetails,omitempty"`
}

type ZhipuQuotaLimitData struct {
	Limits []ZhipuQuotaLimitItem `json:"limits"`
	Level  string                `json:"level"`
}

type ZhipuQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewZhipuQuotaChecker(httpClient *httpclient.HttpClient) *ZhipuQuotaChecker {
	return &ZhipuQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *ZhipuQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = ch.Credentials.APIKeys[0]
	}

	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	baseDomain := buildZhipuBaseDomain(ch.BaseURL)
	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	var rawData = map[string]any{}
	var limits []QuotaLimitStatus
	normalizedStatus := "unknown"

	// 1. Query quota limits (primary data source)
	quotaLimitURL := baseDomain + "/api/monitor/usage/quota/limit"
	quotaReq := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaLimitURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	quotaResp, err := hc.Do(ctx, quotaReq)
	if err == nil && quotaResp.StatusCode == http.StatusOK {
		var apiResp ZhipuAPIResponse[ZhipuQuotaLimitData]
		if json.Unmarshal(quotaResp.Body, &apiResp) == nil && apiResp.Code == 200 {
			limits, normalizedStatus = c.parseQuotaLimitResponse(apiResp.Data)
			rawData["quotaLimits"] = apiResp.Data
			rawData["level"] = apiResp.Data.Level
		}
	}

	// 2. Query model usage
	startTime, endTime := zhipuTimeWindow()
	modelUsageURL := fmt.Sprintf("%s/api/monitor/usage/model-usage?startTime=%s&endTime=%s",
		baseDomain, url.QueryEscape(startTime), url.QueryEscape(endTime))
	modelReq := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(modelUsageURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	modelResp, err := hc.Do(ctx, modelReq)
	if err == nil && modelResp.StatusCode == http.StatusOK {
		var usageResp ZhipuAPIResponse[json.RawMessage]
		if json.Unmarshal(modelResp.Body, &usageResp) == nil && usageResp.Code == 200 {
			rawData["modelUsage"] = usageResp.Data
		}
	}

	// 3. Query tool usage
	toolUsageURL := fmt.Sprintf("%s/api/monitor/usage/tool-usage?startTime=%s&endTime=%s",
		baseDomain, url.QueryEscape(startTime), url.QueryEscape(endTime))
	toolReq := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(toolUsageURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	toolResp, err := hc.Do(ctx, toolReq)
	if err == nil && toolResp.StatusCode == http.StatusOK {
		var usageResp ZhipuAPIResponse[json.RawMessage]
		if json.Unmarshal(toolResp.Body, &usageResp) == nil && usageResp.Code == 200 {
			rawData["toolUsage"] = usageResp.Data
		}
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "zhipu",
		RawData:      rawData,
		NextResetAt:  nil,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}, nil
}

func (c *ZhipuQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	switch ch.Type {
	case channel.TypeZhipu, channel.TypeZhipuAnthropic, channel.TypeZai, channel.TypeZaiAnthropic:
		return true
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		return DetectProviderFromURL(ch.BaseURL) == "zhipu"
	default:
		return false
	}
}

func (c *ZhipuQuotaChecker) parseQuotaLimitResponse(data ZhipuQuotaLimitData) ([]QuotaLimitStatus, string) {
	if len(data.Limits) == 0 {
		return nil, "unknown"
	}

	var limits []QuotaLimitStatus
	worstStatus := "available"

	for _, item := range data.Limits {
		var limitType QuotaLimitType
		switch item.Type {
		case "TOKENS_LIMIT":
			limitType = QuotaLimitTypeToken
		case "TIME_LIMIT":
			limitType = QuotaLimitTypeTime
		default:
			continue
		}

		status := "available"
		usageRatio := item.Percentage / 100.0

		if usageRatio >= 1.0 {
			status = "exhausted"
		} else if usageRatio >= WarningThresholdRatio {
			status = "warning"
		}

		var nextResetAt *time.Time
		if item.NextResetTime > 0 {
			t := time.UnixMilli(item.NextResetTime)
			nextResetAt = &t
		}

		limits = append(limits, QuotaLimitStatus{
			Type:       limitType,
			Status:     status,
			UsageRatio: usageRatio,
			Ready:      IsReadyStatus(status),
			NextResetAt: nextResetAt,
		})

		if status == "exhausted" {
			worstStatus = "exhausted"
		} else if status == "warning" && worstStatus != "exhausted" {
			worstStatus = "warning"
		}
	}

	return limits, worstStatus
}

func buildZhipuBaseDomain(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://open.bigmodel.cn"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme == "" && parsed.Host == "") {
		return "https://open.bigmodel.cn"
	}

	scheme := parsed.Scheme
	if scheme == "http" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, parsed.Host)
}

func zhipuTimeWindow() (string, string) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day()-1, now.Hour(), 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 59, 59, 999000000, now.Location())
	format := "2006-01-02 15:04:05"
	return start.Format(format), end.Format(format)
}
