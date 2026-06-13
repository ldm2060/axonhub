package biz

import "strings"

type ClientDetector struct{}

var SupportedCodingClients = []string{
	"claude-cli",
	"codex-cli",
	"cursor",
	"antigravity",
	"opencode",
	"aider",
	"cline",
	"continue",
	"copilot",
	"github-copilot",
	"windsurf",
	"cody",
}

var ChannelClientMapping = map[string][]string{
	"claudecode":             {"claude-cli"},
	"codex":                  {"codex-cli"},
	"github_copilot":         {"copilot", "github-copilot"},
	"antigravity":            {"antigravity"},
	"opencode_go":            {"opencode"},
	"opencode_go_anthropic":  {"opencode"},
	"moonshot_coding":        {"moonshot-cli"},
}

func (d *ClientDetector) DetectClient(userAgent string) string {
	if userAgent == "" {
		return ""
	}

	ua := strings.ToLower(userAgent)

	// Check for each supported client in order
	// More specific patterns should be checked first
	patterns := map[string]string{
		"claude-cli":     "claude-cli",
		"claudecode":     "claude-cli",
		"codex-cli":      "codex-cli",
		"codex":          "codex-cli",
		"cursor":         "cursor",
		"antigravity":    "antigravity",
		"opencode":       "opencode",
		"aider":          "aider",
		"cline":          "cline",
		"continue":       "continue",
		"github-copilot": "github-copilot",
		"copilot":        "copilot",
		"windsurf":       "windsurf",
		"cody":           "cody",
	}

	// Check in order of specificity
	orderedPatterns := []string{
		"claude-cli", "claudecode",
		"codex-cli", "codex",
		"github-copilot", "copilot",
		"antigravity", "opencode",
		"cursor", "windsurf", "cody",
		"aider", "cline", "continue",
	}

	for _, pattern := range orderedPatterns {
		if strings.Contains(ua, pattern) {
			return patterns[pattern]
		}
	}

	return ""
}

func (d *ClientDetector) IsLenientClientAllowed(userAgent string) bool {
	client := d.DetectClient(userAgent)
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

func (d *ClientDetector) IsStrictClientAllowed(userAgent string, channelType string) bool {
	client := d.DetectClient(userAgent)
	if client == "" {
		return false
	}

	allowedClients, exists := ChannelClientMapping[channelType]
	if !exists {
		return false
	}

	for _, allowedClient := range allowedClients {
		if client == allowedClient {
			return true
		}
	}

	return false
}
