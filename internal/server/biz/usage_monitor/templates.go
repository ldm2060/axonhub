package usage_monitor

// ChannelTemplate defines a pre-configured provider template for usage monitoring.
type ChannelTemplate struct {
	ProviderType          string
	Name                  string
	Description           string
	ApiURL                string
	ApiMethod             string
	HeaderFormat          string // "bearer" | "x-api-key"
	ApiBody               string
	AuthType              string // "api_key" | "oauth" | "device_flow" — matches channel dialog auth flow
	CredentialLabel       string // label for the credential input, e.g. "API Key", "Session Token"
	CredentialPlaceholder string // placeholder for the credential input
	Variables             []Variable
	DisplayFields         []DisplayField
}

// QuotaMonitorTemplate is deprecated. Use ChannelTemplate instead.
// Kept for backward compatibility with the GraphQL resolver until regenerated.
type QuotaMonitorTemplate struct {
	ProviderType          string        `json:"providerType"`
	Name                  string        `json:"name"`
	Description           string        `json:"description,omitempty"`
	ApiURL                string        `json:"apiUrl"`
	ApiMethod             string        `json:"apiMethod"`
	HeaderFormat          string        `json:"headerFormat"` // "bearer" or "x-api-key"
	ApiBody               string        `json:"apiBody,omitempty"`
	AuthType              string        `json:"authType,omitempty"` // "api_key" | "oauth" | "device_flow"
	CredentialLabel       string        `json:"credentialLabel,omitempty"`
	CredentialPlaceholder string        `json:"credentialPlaceholder,omitempty"`
	Fields                []FieldConfig `json:"fields"`
}

var channelTemplates = []ChannelTemplate{
	{
		ProviderType:          "claudecode",
		Name:                  "Claude Code",
		Description:           "Monitor Claude Code rate limits and usage windows",
		ApiURL:                "https://api.anthropic.com/v1/messages",
		ApiMethod:             "POST",
		HeaderFormat:          "x-api-key",
		ApiBody:               `{"model":"claude-haiku-4-5","messages":[{"role":"user","content":"limit"}],"max_tokens":1}`,
		AuthType:              "oauth",
		CredentialLabel:       "API Key",
		CredentialPlaceholder: "sk-ant-api03-...",
		Variables: []Variable{
			{Key: "5h_utilization", Path: "$.headers.anthropic-ratelimit-unified-5h-utilization", Type: "jsonpath"},
			{Key: "7d_utilization", Path: "$.headers.anthropic-ratelimit-unified-7d-utilization", Type: "jsonpath"},
			{Key: "unified_status", Path: "$.headers.anthropic-ratelimit-unified-status", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "5h_utilization", Label: "5h Window Utilization", ValueRef: "5h_utilization", Format: "percentage", DisplayOrder: 0},
			{Key: "7d_utilization", Label: "7d Window Utilization", ValueRef: "7d_utilization", Format: "percentage", DisplayOrder: 1},
			{Key: "unified_status", Label: "Unified Status", ValueRef: "unified_status", Format: "text", DisplayOrder: 2, Badge: "status", BadgePresets: `{"closed":"sapphire","paused":"rosegold"}`},
		},
	},
	{
		ProviderType:          "codex",
		Name:                  "Codex / ChatGPT",
		Description:           "Monitor ChatGPT usage limits and rate windows",
		ApiURL:                "https://chatgpt.com/backend-api/wham/usage",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "oauth",
		CredentialLabel:       "Session Token",
		CredentialPlaceholder: "eyJhbGciOi...",
		Variables: []Variable{
			{Key: "primary_used_pct", Path: "$.rate_limit.primary_window.used_percent", Type: "jsonpath"},
			{Key: "primary_reset", Path: "$.rate_limit.primary_window.reset_at", Type: "jsonpath"},
			{Key: "plan_type", Path: "$.plan_type", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "primary_used_pct", Label: "Primary Window Used %", ValueRef: "primary_used_pct", Format: "percentage", DisplayOrder: 0},
			{Key: "primary_reset", Label: "Primary Reset At", ValueRef: "primary_reset", Format: "datetime", DisplayOrder: 1},
			{Key: "plan_type", Label: "Plan Type", ValueRef: "plan_type", Format: "text", DisplayOrder: 2, Badge: "plan", BadgePresets: `{"free":"sapphire","plus":"rosegold","pro":"champagne","team":"champagne"}`},
		},
	},
	{
		ProviderType:          "github_copilot",
		Name:                  "GitHub Copilot",
		Description:           "Monitor GitHub Copilot quota limits and remaining usage",
		ApiURL:                "https://api.github.com/copilot_internal/user",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "device_flow",
		CredentialLabel:       "GitHub Token",
		CredentialPlaceholder: "ghu_...",
		Variables: []Variable{
			{Key: "plan", Path: "$.copilot_plan", Type: "jsonpath"},
			{Key: "access_type", Path: "$.access_type_sku", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "plan", Label: "Plan", ValueRef: "plan", Format: "text", DisplayOrder: 0, Badge: "plan", BadgePresets: `{"individual":"sapphire","business":"rosegold","enterprise":"champagne"}`},
			{Key: "access_type", Label: "Access Type", ValueRef: "access_type", Format: "text", DisplayOrder: 1, Badge: "access_type", BadgePresets: `{"stable":"sapphire","beta":"rosegold"}`},
		},
	},
	{
		ProviderType:          "nanogpt",
		Name:                  "NanoGPT",
		Description:           "Monitor NanoGPT subscription usage and token/image limits",
		ApiURL:                "https://nano-gpt.com/api/subscription/v1/usage",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "api_key",
		CredentialLabel:       "API Key",
		CredentialPlaceholder: "sk-...",
		Variables: []Variable{
			{Key: "weekly_tokens_pct", Path: "$.weeklyInputTokens.percentUsed", Type: "jsonpath"},
			{Key: "daily_tokens_pct", Path: "$.dailyInputTokens.percentUsed", Type: "jsonpath"},
			{Key: "daily_images_pct", Path: "$.dailyImages.percentUsed", Type: "jsonpath"},
			{Key: "state", Path: "$.state", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "weekly_tokens_pct", Label: "Weekly Tokens Used %", ValueRef: "weekly_tokens_pct", Format: "percentage", DisplayOrder: 0},
			{Key: "daily_tokens_pct", Label: "Daily Tokens Used %", ValueRef: "daily_tokens_pct", Format: "percentage", DisplayOrder: 1},
			{Key: "daily_images_pct", Label: "Daily Images Used %", ValueRef: "daily_images_pct", Format: "percentage", DisplayOrder: 2},
			{Key: "state", Label: "State", ValueRef: "state", Format: "text", DisplayOrder: 3, Badge: "state", BadgePresets: `{"active":"sapphire","expired":"rosegold","cancelled":"rosegold"}`},
		},
	},
	{
		ProviderType:          "wafer",
		Name:                  "Wafer",
		Description:           "Monitor Wafer.ai inference quota and request limits",
		ApiURL:                "https://pass.wafer.ai/v1/inference/quota",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "api_key",
		CredentialLabel:       "API Key",
		CredentialPlaceholder: "wf-...",
		Variables: []Variable{
			{Key: "used_pct", Path: "$.current_period_used_percent", Type: "jsonpath"},
			{Key: "remaining", Path: "$.remaining_included_requests", Type: "jsonpath"},
			{Key: "total", Path: "$.included_request_limit", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "used_pct", Label: "Period Used %", ValueRef: "used_pct", Format: "percentage", DisplayOrder: 0},
			{Key: "remaining", Label: "Remaining Requests", ValueRef: "remaining", Format: "number", Unit: "requests", DisplayOrder: 1},
			{Key: "total", Label: "Total Requests", ValueRef: "total", Format: "number", Unit: "requests", DisplayOrder: 2},
		},
	},
	{
		ProviderType:          "synthetic",
		Name:                  "Synthetic",
		Description:           "Monitor Synthetic.new weekly token and 5h rolling limits",
		ApiURL:                "https://api.synthetic.new/v2/quotas",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "api_key",
		CredentialLabel:       "API Key",
		CredentialPlaceholder: "sk-...",
		Variables: []Variable{
			{Key: "weekly_remaining_pct", Path: "$.weeklyTokenLimit.percentRemaining", Type: "jsonpath"},
			{Key: "5h_remaining", Path: "$.rollingFiveHourLimit.remaining", Type: "jsonpath"},
			{Key: "5h_max", Path: "$.rollingFiveHourLimit.max", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "weekly_remaining_pct", Label: "Weekly Tokens Remaining %", ValueRef: "weekly_remaining_pct", Format: "percentage", DisplayOrder: 0},
			{Key: "5h_remaining", Label: "5h Rolling Remaining", ValueRef: "5h_remaining", Format: "number", DisplayOrder: 1},
			{Key: "5h_max", Label: "5h Rolling Max", ValueRef: "5h_max", Format: "number", DisplayOrder: 2},
		},
	},
	{
		ProviderType:          "neuralwatt",
		Name:                  "NeuralWatt",
		Description:           "Monitor NeuralWatt kWh subscription usage and credits",
		ApiURL:                "https://api.neuralwatt.com/v1/quota",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "api_key",
		CredentialLabel:       "API Key",
		CredentialPlaceholder: "nw-...",
		Variables: []Variable{
			{Key: "kwh_used", Path: "$.subscription.kwh_used", Type: "jsonpath"},
			{Key: "kwh_included", Path: "$.subscription.kwh_included", Type: "jsonpath"},
			{Key: "credits_remaining", Path: "$.balance.credits_remaining_usd", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "kwh_used", Label: "kWh Used", ValueRef: "kwh_used", Format: "number", Unit: "kWh", DisplayOrder: 0},
			{Key: "kwh_included", Label: "kWh Included", ValueRef: "kwh_included", Format: "number", Unit: "kWh", DisplayOrder: 1},
			{Key: "credits_remaining", Label: "Credits Remaining", ValueRef: "credits_remaining", Format: "number", Unit: "USD", DisplayOrder: 2},
		},
	},
	{
		ProviderType:          "zhipu",
		Name:                  "Zhipu / Z.ai",
		Description:           "Monitor Zhipu AI quota limits for tokens and MCP usage",
		ApiURL:                "https://open.bigmodel.cn/api/monitor/usage/quota/limit",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "api_key",
		CredentialLabel:       "JWT Token",
		CredentialPlaceholder: "xxx.xxx...",
		Variables: []Variable{
			{Key: "level", Path: "$.data.level", Type: "jsonpath"},
			// ZhiPu API returns limits in order: [0]=TIME_LIMIT, [1]=TOKENS_LIMIT
			{Key: "token_pct", Path: "$.data.limits[1].percentage", Type: "jsonpath"},
			{Key: "token_reset", Path: "$.data.limits[1].nextResetTime", Type: "jsonpath"},
			{Key: "time_pct", Path: "$.data.limits[0].percentage", Type: "jsonpath"},
			{Key: "time_remaining", Path: "$.data.limits[0].remaining", Type: "jsonpath"},
			{Key: "time_reset", Path: "$.data.limits[0].nextResetTime", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "level", Label: "Account Level", ValueRef: "level", Format: "text", DisplayOrder: 0, Badge: "level", BadgePresets: `{"lite":"sapphire","pro":"rosegold","max":"champagne"}`},
			{Key: "token_pct", Label: "Token Usage %", ValueRef: "token_pct", Format: "percentage", DisplayOrder: 1},
			{Key: "token_reset", Label: "Token Reset At", ValueRef: "token_reset", Format: "datetime", DisplayOrder: 2},
			{Key: "time_pct", Label: "MCP Time Usage %", ValueRef: "time_pct", Format: "percentage", DisplayOrder: 3},
			{Key: "time_remaining", Label: "MCP Remaining", ValueRef: "time_remaining", Format: "number", Unit: "units", DisplayOrder: 4},
			{Key: "time_reset", Label: "MCP Reset At", ValueRef: "time_reset", Format: "datetime", DisplayOrder: 5},
		},
	},
}

// GetChannelTemplates returns all available channel templates.
func GetChannelTemplates() []ChannelTemplate {
	return channelTemplates
}

// GetChannelTemplate returns a channel template by provider type.
func GetChannelTemplate(providerType string) *ChannelTemplate {
	for i := range channelTemplates {
		if channelTemplates[i].ProviderType == providerType {
			return &channelTemplates[i]
		}
	}
	return nil
}

// ToLegacy converts a ChannelTemplate to the deprecated QuotaMonitorTemplate
// for backward compatibility with the GraphQL resolver.
func (t *ChannelTemplate) ToLegacy() QuotaMonitorTemplate {
	fields := make([]FieldConfig, 0, len(t.Variables))
	for _, v := range t.Variables {
		fields = append(fields, FieldConfig{
			Key:  v.Key,
			Path: v.Path,
			Type: v.Type,
		})
	}
	return QuotaMonitorTemplate{
		ProviderType:          t.ProviderType,
		Name:                  t.Name,
		Description:           t.Description,
		ApiURL:                t.ApiURL,
		ApiMethod:             t.ApiMethod,
		HeaderFormat:          t.HeaderFormat,
		ApiBody:               t.ApiBody,
		AuthType:              t.AuthType,
		CredentialLabel:       t.CredentialLabel,
		CredentialPlaceholder: t.CredentialPlaceholder,
		Fields:                fields,
	}
}

// GetQuotaMonitorTemplates returns all templates as legacy QuotaMonitorTemplate.
// Deprecated: Use GetChannelTemplates() instead.
func GetQuotaMonitorTemplates() []QuotaMonitorTemplate {
	templates := GetChannelTemplates()
	result := make([]QuotaMonitorTemplate, len(templates))
	for i := range templates {
		result[i] = templates[i].ToLegacy()
	}
	return result
}

// GetQuotaMonitorTemplate returns a single template as legacy QuotaMonitorTemplate.
// Deprecated: Use GetChannelTemplate() instead.
func GetQuotaMonitorTemplate(providerType string) *QuotaMonitorTemplate {
	tmpl := GetChannelTemplate(providerType)
	if tmpl == nil {
		return nil
	}
	legacy := tmpl.ToLegacy()
	return &legacy
}
