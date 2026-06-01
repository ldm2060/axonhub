package usage_monitor

type QuotaMonitorTemplate struct {
	ProviderType string        `json:"providerType"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ApiURL       string        `json:"apiUrl"`
	ApiMethod    string        `json:"apiMethod"`
	HeaderFormat string        `json:"headerFormat"` // "bearer" or "x-api-key"
	ApiBody      string        `json:"apiBody,omitempty"`
	Fields       []FieldConfig `json:"fields"`
}

var quotaMonitorTemplates = []QuotaMonitorTemplate{
	{
		ProviderType: "claudecode",
		Name:         "Claude Code",
		Description:  "Monitor Claude Code rate limits and usage windows",
		ApiURL:       "https://api.anthropic.com/v1/messages",
		ApiMethod:    "POST",
		HeaderFormat: "bearer",
		ApiBody:      `{"model":"claude-haiku-4-5","messages":[{"role":"user","content":"limit"}],"max_tokens":1}`,
		Fields: []FieldConfig{
			{Key: "5h_utilization", Label: "5h Window Utilization", Path: "$.headers.anthropic-ratelimit-unified-5h-utilization", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "7d_utilization", Label: "7d Window Utilization", Path: "$.headers.anthropic-ratelimit-unified-7d-utilization", Type: "jsonpath", Format: "percentage", DisplayOrder: 1},
			{Key: "unified_status", Label: "Unified Status", Path: "$.headers.anthropic-ratelimit-unified-status", Type: "jsonpath", Format: "text", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "codex",
		Name:         "Codex / ChatGPT",
		Description:  "Monitor ChatGPT usage limits and rate windows",
		ApiURL:       "https://chatgpt.com/backend-api/wham/usage",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "primary_used_pct", Label: "Primary Window Used %", Path: "$.rate_limit.primary_window.used_percent", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "primary_reset", Label: "Primary Reset At", Path: "$.rate_limit.primary_window.reset_at", Type: "jsonpath", Format: "datetime", DisplayOrder: 1},
			{Key: "plan_type", Label: "Plan Type", Path: "$.plan_type", Type: "jsonpath", Format: "text", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "github_copilot",
		Name:         "GitHub Copilot",
		Description:  "Monitor GitHub Copilot quota limits and remaining usage",
		ApiURL:       "https://api.github.com/copilot_internal/user",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "plan", Label: "Plan", Path: "$.copilot_plan", Type: "jsonpath", Format: "text", DisplayOrder: 0},
			{Key: "access_type", Label: "Access Type", Path: "$.access_type_sku", Type: "jsonpath", Format: "text", DisplayOrder: 1},
		},
	},
	{
		ProviderType: "nanogpt",
		Name:         "NanoGPT",
		Description:  "Monitor NanoGPT subscription usage and token/image limits",
		ApiURL:       "https://nano-gpt.com/api/subscription/v1/usage",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "weekly_tokens_pct", Label: "Weekly Tokens Used %", Path: "$.weeklyInputTokens.percentUsed", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "daily_tokens_pct", Label: "Daily Tokens Used %", Path: "$.dailyInputTokens.percentUsed", Type: "jsonpath", Format: "percentage", DisplayOrder: 1},
			{Key: "daily_images_pct", Label: "Daily Images Used %", Path: "$.dailyImages.percentUsed", Type: "jsonpath", Format: "percentage", DisplayOrder: 2},
			{Key: "state", Label: "State", Path: "$.state", Type: "jsonpath", Format: "text", DisplayOrder: 3},
		},
	},
	{
		ProviderType: "wafer",
		Name:         "Wafer",
		Description:  "Monitor Wafer.ai inference quota and request limits",
		ApiURL:       "https://pass.wafer.ai/v1/inference/quota",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "used_pct", Label: "Period Used %", Path: "$.current_period_used_percent", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "remaining", Label: "Remaining Requests", Path: "$.remaining_included_requests", Type: "jsonpath", Format: "number", Unit: "requests", DisplayOrder: 1},
			{Key: "total", Label: "Total Requests", Path: "$.included_request_limit", Type: "jsonpath", Format: "number", Unit: "requests", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "synthetic",
		Name:         "Synthetic",
		Description:  "Monitor Synthetic.new weekly token and 5h rolling limits",
		ApiURL:       "https://api.synthetic.new/v2/quotas",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "weekly_remaining_pct", Label: "Weekly Tokens Remaining %", Path: "$.weeklyTokenLimit.percentRemaining", Type: "jsonpath", Format: "percentage", DisplayOrder: 0},
			{Key: "5h_remaining", Label: "5h Rolling Remaining", Path: "$.rollingFiveHourLimit.remaining", Type: "jsonpath", Format: "number", DisplayOrder: 1},
			{Key: "5h_max", Label: "5h Rolling Max", Path: "$.rollingFiveHourLimit.max", Type: "jsonpath", Format: "number", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "neuralwatt",
		Name:         "NeuralWatt",
		Description:  "Monitor NeuralWatt kWh subscription usage and credits",
		ApiURL:       "https://api.neuralwatt.com/v1/quota",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "kwh_used", Label: "kWh Used", Path: "$.subscription.kwh_used", Type: "jsonpath", Format: "number", Unit: "kWh", DisplayOrder: 0},
			{Key: "kwh_included", Label: "kWh Included", Path: "$.subscription.kwh_included", Type: "jsonpath", Format: "number", Unit: "kWh", DisplayOrder: 1},
			{Key: "credits_remaining", Label: "Credits Remaining", Path: "$.balance.credits_remaining_usd", Type: "jsonpath", Format: "number", Unit: "USD", DisplayOrder: 2},
		},
	},
	{
		ProviderType: "zhipu",
		Name:         "Zhipu / Z.ai",
		Description:  "Monitor Zhipu AI quota limits for tokens and MCP usage",
		ApiURL:       "https://open.bigmodel.cn/api/monitor/usage/quota/limit",
		ApiMethod:    "GET",
		HeaderFormat: "bearer",
		Fields: []FieldConfig{
			{Key: "level", Label: "Account Level", Path: "$.data.level", Type: "jsonpath", Format: "text", DisplayOrder: 0},
			// ZhiPu API returns limits in order: [0]=TIME_LIMIT, [1]=TOKENS_LIMIT
			{Key: "token_pct", Label: "Token Usage %", Path: "$.data.limits[1].percentage", Type: "jsonpath", Format: "percentage", DisplayOrder: 1},
			{Key: "token_reset", Label: "Token Reset At", Path: "$.data.limits[1].nextResetTime", Type: "jsonpath", Format: "datetime", DisplayOrder: 2},
			{Key: "time_pct", Label: "MCP Time Usage %", Path: "$.data.limits[0].percentage", Type: "jsonpath", Format: "percentage", DisplayOrder: 3},
			{Key: "time_remaining", Label: "MCP Remaining", Path: "$.data.limits[0].remaining", Type: "jsonpath", Format: "number", Unit: "units", DisplayOrder: 4},
			{Key: "time_reset", Label: "MCP Reset At", Path: "$.data.limits[0].nextResetTime", Type: "jsonpath", Format: "datetime", DisplayOrder: 5},
		},
	},
}

func GetQuotaMonitorTemplates() []QuotaMonitorTemplate {
	return quotaMonitorTemplates
}

func GetQuotaMonitorTemplate(providerType string) *QuotaMonitorTemplate {
	for i := range quotaMonitorTemplates {
		if quotaMonitorTemplates[i].ProviderType == providerType {
			return &quotaMonitorTemplates[i]
		}
	}
	return nil
}
