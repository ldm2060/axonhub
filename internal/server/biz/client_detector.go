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

func (d *ClientDetector) DetectClient(userAgent string) string {
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
