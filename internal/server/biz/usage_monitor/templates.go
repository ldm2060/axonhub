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
		Description:           "Monitor Claude Code usage via organizations API (OAuth)",
		ApiURL:                "https://api.anthropic.com/v1/organizations/{org_uuid}/usage",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "oauth",
		CredentialLabel:       "OAuth Token",
		CredentialPlaceholder: "eyJhbGciOi...",
		Variables: []Variable{
			{Key: "5h_utilization", Path: "$.five_hour.utilization", Type: "jsonpath"},
			{Key: "5h_reset", Path: "$.five_hour.resets_at", Type: "jsonpath"},
			{Key: "7d_utilization", Path: "$.seven_day.utilization", Type: "jsonpath"},
			{Key: "7d_reset", Path: "$.seven_day.resets_at", Type: "jsonpath"},
			{Key: "7d_opus_utilization", Path: "$.seven_day_opus.utilization", Type: "jsonpath"},
			{Key: "7d_sonnet_utilization", Path: "$.seven_day_sonnet.utilization", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "5h_utilization", Label: "5h Window Utilization", ValueRef: "5h_utilization", Format: "fraction", DisplayOrder: 0},
			{Key: "5h_reset", Label: "5h Reset At", ValueRef: "5h_reset", Format: "datetime", DisplayOrder: 1},
			{Key: "7d_utilization", Label: "7d Window Utilization", ValueRef: "7d_utilization", Format: "fraction", DisplayOrder: 2},
			{Key: "7d_reset", Label: "7d Reset At", ValueRef: "7d_reset", Format: "datetime", DisplayOrder: 3},
			{Key: "7d_opus_utilization", Label: "7d Opus Utilization", ValueRef: "7d_opus_utilization", Format: "fraction", DisplayOrder: 4},
			{Key: "7d_sonnet_utilization", Label: "7d Sonnet Utilization", ValueRef: "7d_sonnet_utilization", Format: "fraction", DisplayOrder: 5},
		},
	},
	{
		ProviderType:          "codex",
		Name:                  "Codex / ChatGPT",
		Description:           "Monitor ChatGPT usage limits, rate windows, and additional metered features",
		ApiURL:                "https://chatgpt.com/backend-api/wham/usage",
		ApiMethod:             "GET",
		HeaderFormat:          "bearer",
		AuthType:              "oauth",
		CredentialLabel:       "Session Token",
		CredentialPlaceholder: "eyJhbGciOi...",
			Variables: []Variable{
				{Key: "primary_used_pct", Path: "$.rate_limit.primary_window.used_percent", Type: "jsonpath"},
				{Key: "primary_reset", Path: "$.rate_limit.primary_window.reset_at", Type: "jsonpath"},
				{Key: "secondary_used_pct", Path: "$.rate_limit.secondary_window.used_percent", Type: "jsonpath"},
				{Key: "secondary_reset", Path: "$.rate_limit.secondary_window.reset_at", Type: "jsonpath"},
				{Key: "limit_reached", Path: "$.rate_limit.limit_reached", Type: "jsonpath"},
				{Key: "plan_type", Path: "$.plan_type", Type: "jsonpath"},
				// Additional rate limits for Pro/Pro-Lite plans (metered features like images, extended context)
				{Key: "additional_0_name", Path: "$.additional_rate_limits[0].metered_feature", Type: "jsonpath"},
				{Key: "additional_0_primary_pct", Path: "$.additional_rate_limits[0].rate_limit.primary_window.used_percent", Type: "jsonpath"},
				{Key: "additional_0_primary_reset", Path: "$.additional_rate_limits[0].rate_limit.primary_window.reset_at", Type: "jsonpath"},
				{Key: "additional_0_secondary_pct", Path: "$.additional_rate_limits[0].rate_limit.secondary_window.used_percent", Type: "jsonpath"},
				{Key: "additional_0_secondary_reset", Path: "$.additional_rate_limits[0].rate_limit.secondary_window.reset_at", Type: "jsonpath"},
				{Key: "additional_1_name", Path: "$.additional_rate_limits[1].metered_feature", Type: "jsonpath"},
				{Key: "additional_1_primary_pct", Path: "$.additional_rate_limits[1].rate_limit.primary_window.used_percent", Type: "jsonpath"},
				{Key: "additional_1_primary_reset", Path: "$.additional_rate_limits[1].rate_limit.primary_window.reset_at", Type: "jsonpath"},
				{Key: "additional_1_secondary_pct", Path: "$.additional_rate_limits[1].rate_limit.secondary_window.used_percent", Type: "jsonpath"},
				{Key: "additional_1_secondary_reset", Path: "$.additional_rate_limits[1].rate_limit.secondary_window.reset_at", Type: "jsonpath"},
			},
			DisplayFields: []DisplayField{
				{Key: "primary_used_pct", Label: "5h Window Used %", ValueRef: "primary_used_pct", Format: "percentage", DisplayOrder: 0},
				{Key: "primary_reset", Label: "5h Reset At", ValueRef: "primary_reset", Format: "datetime", DisplayOrder: 1},
				{Key: "secondary_used_pct", Label: "7d Window Used %", ValueRef: "secondary_used_pct", Format: "percentage", DisplayOrder: 2},
				{Key: "secondary_reset", Label: "7d Reset At", ValueRef: "secondary_reset", Format: "datetime", DisplayOrder: 3},
				{Key: "plan_type", Label: "Plan Type", ValueRef: "plan_type", Format: "text", DisplayOrder: 4, Badge: "plan", BadgePresets: `{"free":"sapphire","plus":"rosegold","pro_lite":"freshgreen","pro":"champagne","team":"champagne"}`},
				{Key: "limit_reached", Label: "Limit Reached", ValueRef: "limit_reached", Format: "text", DisplayOrder: 5, Badge: "limit", BadgePresets: `{"true":"rosegold","false":"sapphire"}`},
				{Key: "additional_0_name", Label: "Extra Feature", ValueRef: "additional_0_name", Format: "text", DisplayOrder: 6},
				{Key: "additional_0_primary_pct", Label: "Extra 5h Used %", ValueRef: "additional_0_primary_pct", Format: "percentage", DisplayOrder: 7},
				{Key: "additional_0_primary_reset", Label: "Extra 5h Reset", ValueRef: "additional_0_primary_reset", Format: "datetime", DisplayOrder: 8},
				{Key: "additional_0_secondary_pct", Label: "Extra 7d Used %", ValueRef: "additional_0_secondary_pct", Format: "percentage", DisplayOrder: 9},
				{Key: "additional_0_secondary_reset", Label: "Extra 7d Reset", ValueRef: "additional_0_secondary_reset", Format: "datetime", DisplayOrder: 10},
				{Key: "additional_1_name", Label: "Extra Feature 2", ValueRef: "additional_1_name", Format: "text", DisplayOrder: 11},
				{Key: "additional_1_primary_pct", Label: "Extra 2 5h Used %", ValueRef: "additional_1_primary_pct", Format: "percentage", DisplayOrder: 12},
				{Key: "additional_1_primary_reset", Label: "Extra 2 5h Reset", ValueRef: "additional_1_primary_reset", Format: "datetime", DisplayOrder: 13},
				{Key: "additional_1_secondary_pct", Label: "Extra 2 7d Used %", ValueRef: "additional_1_secondary_pct", Format: "percentage", DisplayOrder: 14},
				{Key: "additional_1_secondary_reset", Label: "Extra 2 7d Reset", ValueRef: "additional_1_secondary_reset", Format: "datetime", DisplayOrder: 15},
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
				// Limited quotas (Free accounts): remaining counts per feature
				{Key: "limited_quotas", Path: "$.limited_user_quotas", Type: "jsonpath"},
				// Monthly quotas (total counts per feature)
				{Key: "monthly_quotas", Path: "$.monthly_quotas", Type: "jsonpath"},
				// Quota snapshots (EDU/Premium accounts): percent_remaining per feature
				{Key: "quota_snapshots", Path: "$.quota_snapshots", Type: "jsonpath"},
				// Reset dates
				{Key: "limited_reset_date", Path: "$.limited_user_reset_date", Type: "jsonpath"},
				{Key: "quota_reset_date_utc", Path: "$.quota_reset_date_utc", Type: "jsonpath"},
				{Key: "quota_reset_date", Path: "$.quota_reset_date", Type: "jsonpath"},
			},
			DisplayFields: []DisplayField{
				{Key: "plan", Label: "Plan", ValueRef: "plan", Format: "text", DisplayOrder: 0, Badge: "plan", BadgePresets: `{"individual":"sapphire","business":"rosegold","enterprise":"champagne"}`},
				{Key: "access_type", Label: "Access Type", ValueRef: "access_type", Format: "text", DisplayOrder: 1, Badge: "access_type", BadgePresets: `{"copilot_free":"sapphire","free_limited_copilot":"sapphire","copilot_pro":"rosegold","copilot_pro_plus":"champagne","copilot_business":"champagne","copilot_enterprise":"champagne","free_educational_quota":"freshgreen"}`},
				{Key: "limited_quotas", Label: "Limited Quotas", ValueRef: "limited_quotas", Format: "text", DisplayOrder: 2},
				{Key: "monthly_quotas", Label: "Monthly Quotas", ValueRef: "monthly_quotas", Format: "text", DisplayOrder: 3},
				{Key: "quota_snapshots", Label: "Quota Snapshots", ValueRef: "quota_snapshots", Format: "text", DisplayOrder: 4},
				{Key: "limited_reset_date", Label: "Limited Reset", ValueRef: "limited_reset_date", Format: "text", DisplayOrder: 5},
				{Key: "quota_reset_date_utc", Label: "Quota Reset (UTC)", ValueRef: "quota_reset_date_utc", Format: "datetime", DisplayOrder: 6},
				{Key: "quota_reset_date", Label: "Quota Reset", ValueRef: "quota_reset_date", Format: "text", DisplayOrder: 7},
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
			{Key: "state", Label: "State", ValueRef: "state", Format: "text", DisplayOrder: 3, Badge: "state", BadgePresets: `{"active":"sapphire","expired":"rosegold","canceled":"rosegold"}`},
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
	{
		ProviderType:          "antigravity",
		Name:                  "Antigravity",
		Description:           "Monitor Gemini Code Assist (Antigravity) quota via retrieveUserQuota API",
		ApiURL:                "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota",
		ApiMethod:             "POST",
		HeaderFormat:          "bearer",
		ApiBody:               `{}`,
		AuthType:              "oauth",
		CredentialLabel:       "OAuth Token",
		CredentialPlaceholder: "ya29...",
		Variables: []Variable{
			{Key: "bucket_0_remaining_fraction", Path: "$.buckets[0].remainingFraction", Type: "jsonpath"},
			{Key: "bucket_0_remaining_amount", Path: "$.buckets[0].remainingAmount", Type: "jsonpath"},
			{Key: "bucket_0_reset_time", Path: "$.buckets[0].resetTime", Type: "jsonpath"},
			{Key: "bucket_0_model_id", Path: "$.buckets[0].modelId", Type: "jsonpath"},
			{Key: "tier_name", Path: "$.currentTier.name", Type: "jsonpath"},
		},
		DisplayFields: []DisplayField{
			{Key: "tier_name", Label: "Current Tier", ValueRef: "tier_name", Format: "text", DisplayOrder: 0, Badge: "tier", BadgePresets: `{"free":"sapphire","payg":"rosegold","subscription":"champagne"}`},
			{Key: "bucket_0_remaining_fraction", Label: "Remaining Fraction", ValueRef: "bucket_0_remaining_fraction", Format: "fraction", DisplayOrder: 1},
			{Key: "bucket_0_remaining_amount", Label: "Remaining Amount", ValueRef: "bucket_0_remaining_amount", Format: "number", DisplayOrder: 2},
			{Key: "bucket_0_reset_time", Label: "Reset Time", ValueRef: "bucket_0_reset_time", Format: "datetime", DisplayOrder: 3},
			{Key: "bucket_0_model_id", Label: "Model", ValueRef: "bucket_0_model_id", Format: "text", DisplayOrder: 4},
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
