package biz

import (
	"slices"
	"strings"
)

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
	"kimi_code":             {"claude", "codex", "opencode"},
}

// API-format families used for strict-mode matching on generic channels
// (channel types without a precise ChannelClientMapping entry).
const (
	familyAnthropic = "anthropic"
	familyOpenAI    = "openai"
	familyGemini    = "gemini"
)

func (d *ClientDetector) DetectClient(userAgent string, referer string) string {
	if userAgent == "" {
		return ""
	}

	ua := strings.ToLower(userAgent)

	// Match by bare keyword so suffix variants (-cli, -tui, -desktop, …) are
	// all detected, not just the "-cli" form.
	for _, client := range SupportedCodingClients {
		if strings.Contains(ua, client) {
			return client
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

	// Coding-agent channels keep precise per-channel client lists; every other
	// (generic) channel matches the client by API-format family.
	if allowedClients, exists := ChannelClientMapping[channelType]; exists {
		return slices.Contains(allowedClients, client)
	}

	return clientFamily(client) == channelFamily(channelType)
}

func clientFamily(client string) string {
	switch client {
	case "claude":
		return familyAnthropic
	case "codex", "opencode":
		return familyOpenAI
	case "antigravity":
		return familyGemini
	default:
		return ""
	}
}

func channelFamily(channelType string) string {
	lt := strings.ToLower(channelType)
	switch {
	case strings.Contains(lt, "anthropic"):
		return familyAnthropic
	case strings.Contains(lt, "gemini"):
		return familyGemini
	default:
		return familyOpenAI
	}
}
