package biz

import "strings"

type ClientDetector struct{}

var SupportedCodingClients = []string{
	"claude",
	"codex",
	"antigravity",
	"opencode",
}

var ChannelClientMapping = map[string][]string{
	"claudecode":            {"claude"},
	"codex":                 {"codex"},
	"github_copilot":        {"codex"},
	"antigravity":           {"antigravity"},
	"opencode_go":           {"opencode"},
	"opencode_go_anthropic": {"opencode"},
	"moonshot_coding":       {"opencode"},
}

func (d *ClientDetector) DetectClient(userAgent string, referer string) string {
	if userAgent == "" {
		return ""
	}

	ua := strings.ToLower(userAgent)

	patterns := map[string]string{
		"claude-cli":  "claude",
		"claudecode":  "claude",
		"codex-cli":   "codex",
		"codex":       "codex",
		"antigravity": "antigravity",
		"opencode":    "opencode",
	}

	orderedPatterns := []string{
		"claude-cli", "claudecode",
		"codex-cli", "codex",
		"antigravity", "opencode",
	}

	for _, pattern := range orderedPatterns {
		if strings.Contains(ua, pattern) {
			return patterns[pattern]
		}
	}

	// Claude Office (web/desktop app): browser-like UA coming from claude.ai.
	// It has no Claude marker in its UA, so it is identified by the referer.
	if isClaudeOffice(userAgent, referer) {
		return "claude"
	}

	return ""
}

// isClaudeOffice reports whether the request looks like the Claude Office /
// claude.ai web client: a Mozilla/5.0 browser UA with a claude.ai referer.
func isClaudeOffice(userAgent string, referer string) bool {
	return strings.HasPrefix(userAgent, "Mozilla/5.0") &&
		strings.Contains(strings.ToLower(referer), "claude.ai")
}

func (d *ClientDetector) IsLenientClientAllowed(userAgent string, referer string) bool {
	client := d.DetectClient(userAgent, referer)
	if client == "" {
		return false
	}

	for _, supportedClient := range SupportedCodingClients {
		if client == supportedClient {
			return true
		}
	}

	return false
}

func (d *ClientDetector) IsStrictClientAllowed(userAgent string, referer string, channelType string) bool {
	client := d.DetectClient(userAgent, referer)
	if client == "" {
		return false
	}

	allowedClients, exists := ChannelClientMapping[channelType]
	if !exists {
		// Unmapped channel types fall back to lenient supported-client check
		return d.IsLenientClientAllowed(userAgent, referer)
	}

	for _, allowedClient := range allowedClients {
		if client == allowedClient {
			return true
		}
	}

	return false
}
