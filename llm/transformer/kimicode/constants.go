package kimicode

const (
	DefaultOAuthHost = "https://auth.kimi.com"
	DefaultBaseURL   = "https://api.kimi.com/coding/v1"
	// GlobalOAuthHost and GlobalBaseURL serve Kimi Code accounts outside
	// mainland China (region profile "global" in the official CLI).
	GlobalOAuthHost = "https://auth.kimi.ai"
	GlobalBaseURL   = "https://api.kimi.ai/coding/v1"

	// CLIUserAgentProduct and CLIVersion match the official MoonshotAI/kimi-code
	// repository identity used by the Kimi Code CLI.
	CLIUserAgentProduct = "kimi-code-cli"
	CLIVersion          = "2.3.1"

	DeviceAuthorizationPath = "/api/oauth/device_authorization"
	// TokenPath is the Kimi Code public OAuth token endpoint.
	TokenPath     = "/api/oauth/token" //nolint:gosec // URL path, not a credential.
	ModelsPath    = "/models"
	MessagesPath  = "/messages"
	ResponsesPath = "/responses"

	ClientID        = "17e5f671-d194-4dfb-9706-5516cb48c098"
	DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

	// ProtocolKimi, ProtocolAnthropic, and ProtocolResponse are the model
	// protocols declared by the /models catalog. The official CLI maps an
	// absent protocol to the Kimi OpenAI-compatible chat completions wire.
	ProtocolKimi      = "kimi"
	ProtocolAnthropic = "anthropic"
	// ProtocolResponse is the catalog spelling of the OpenAI Responses wire
	// ("response" in the raw /models payload).
	ProtocolResponse = "response"

	// ContextManagementBeta and InterleavedThinkingBeta are carried in the
	// request body of the beta Messages API (/messages?beta=true).
	// Thinking-configured requests carry context management; requests without
	// any thinking configuration fall back to the interleaved-thinking default.
	ContextManagementBeta   = "context-management-2025-06-27"
	InterleavedThinkingBeta = "interleaved-thinking-2025-05-14"
)
