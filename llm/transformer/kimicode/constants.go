package kimicode

const (
	DefaultOAuthHost = "https://auth.kimi.com"
	DefaultBaseURL   = "https://api.kimi.com/coding/v1"

	// CLIUserAgentProduct and CLIVersion match the official @moonshot-ai/kimi-code
	// package identity used by the Kimi Code CLI.
	CLIUserAgentProduct = "kimi-code-cli"
	CLIVersion          = "0.26.0"

	DeviceAuthorizationPath = "/api/oauth/device_authorization"
	// TokenPath is the Kimi Code public OAuth token endpoint.
	TokenPath    = "/api/oauth/token" //nolint:gosec // URL path, not a credential.
	ModelsPath   = "/models"
	MessagesPath = "/messages"

	ClientID        = "17e5f671-d194-4dfb-9706-5516cb48c098"
	DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

	ProtocolKimi      = "kimi"
	ProtocolAnthropic = "anthropic"
)
