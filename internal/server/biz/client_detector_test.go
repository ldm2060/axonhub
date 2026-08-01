package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientDetector_DetectClient(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		// Claude CLI
		{
			name:      "Claude Code",
			userAgent: "ClaudeCode/1.0.0",
			want:      "claude",
		},
		{
			name:      "Claude Code - case insensitive",
			userAgent: "claudecode/2.5.1 (Windows)",
			want:      "claude",
		},
		{
			name:      "Claude CLI explicit",
			userAgent: "claude-cli/1.0.0",
			want:      "claude",
		},
		{
			name:      "Claude TUI variant",
			userAgent: "claude-tui/1.0.0",
			want:      "claude",
		},

		// Codex
		{
			name:      "Codex",
			userAgent: "Codex/1.0",
			want:      "codex",
		},
		{
			name:      "Codex CLI",
			userAgent: "codex-cli/1.0",
			want:      "codex",
		},
		{
			name:      "Codex TUI variant",
			userAgent: "codex-tui/1.0",
			want:      "codex",
		},

		// Antigravity
		{
			name:      "Antigravity",
			userAgent: "Antigravity/1.0",
			want:      "antigravity",
		},

		// OpenCode
		{
			name:      "OpenCode",
			userAgent: "OpenCode/1.0",
			want:      "opencode",
		},

		// Unknown clients
		{
			name:      "Standard browser",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			want:      "",
		},
		{
			name:      "Empty user agent",
			userAgent: "",
			want:      "",
		},
		{
			name:      "Generic SDK",
			userAgent: "MyApp/1.0.0",
			want:      "",
		},
		{
			name:      "Cursor - no longer supported",
			userAgent: "Cursor/0.32.0",
			want:      "",
		},
		{
			name:      "Aider - no longer supported",
			userAgent: "aider/0.45.0",
			want:      "",
		},
		{
			name:      "Windsurf - no longer supported",
			userAgent: "Windsurf/1.2.0",
			want:      "",
		},
		{
			name:      "Cline - no longer supported",
			userAgent: "Cline/3.1.0",
			want:      "",
		},
		{
			name:      "GitHub Copilot - no longer supported",
			userAgent: "GitHub-Copilot/1.0",
			want:      "",
		},
		{
			name:      "Cody - no longer supported",
			userAgent: "Cody/1.0",
			want:      "",
		},
		{
			name:      "Moonshot CLI - no longer supported",
			userAgent: "moonshot-cli/1.0.0",
			want:      "",
		},
	}

	detector := &ClientDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.DetectClient(tt.userAgent, "")
			assert.Equal(t, tt.want, got, "DetectClient(%q) = %v, want %v", tt.userAgent, got, tt.want)
		})
	}
}

func TestClientDetector_DetectClient_ClaudeOffice(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		referer   string
		want      string
	}{
		{
			name:      "Claude Office - claude.ai referer with Mozilla UA",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			referer:   "https://claude.ai/chat/abc-123",
			want:      "claude",
		},
		{
			name:      "Claude Office - bare Mozilla/5.0 UA",
			userAgent: "Mozilla/5.0",
			referer:   "https://claude.ai",
			want:      "claude",
		},
		{
			name:      "Claude Office plugin - pivot.claude.ai referer",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			referer:   "https://pivot.claude.ai/",
			want:      "claude",
		},
		{
			name:      "Mozilla UA without claude.ai referer is not detected",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			referer:   "https://example.com",
			want:      "",
		},
		{
			name:      "claude.ai referer without Mozilla UA is not detected",
			userAgent: "curl/8.0",
			referer:   "https://claude.ai",
			want:      "",
		},
		{
			name:      "empty referer not detected",
			userAgent: "Mozilla/5.0",
			referer:   "",
			want:      "",
		},
	}

	detector := &ClientDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.DetectClient(tt.userAgent, tt.referer)
			assert.Equal(t, tt.want, got, "DetectClient(%q, %q) = %v, want %v", tt.userAgent, tt.referer, got, tt.want)
		})
	}
}

func TestClientDetector_IsLenientClientAllowed(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      bool
	}{
		// Should allow supported clients
		{
			name:      "Claude Code",
			userAgent: "ClaudeCode/1.0.0",
			want:      true,
		},
		{
			name:      "Codex",
			userAgent: "Codex/1.0",
			want:      true,
		},
		{
			name:      "Antigravity",
			userAgent: "Antigravity/1.0",
			want:      true,
		},
		{
			name:      "OpenCode",
			userAgent: "OpenCode/1.0",
			want:      true,
		},

		// Should reject unsupported clients
		{
			name:      "Cursor - not supported",
			userAgent: "Cursor/0.32.0",
			want:      false,
		},
		{
			name:      "Windsurf - not supported",
			userAgent: "Windsurf/1.2.0",
			want:      false,
		},
		{
			name:      "Aider - not supported",
			userAgent: "aider/0.45.0",
			want:      false,
		},
		{
			name:      "GitHub Copilot - not supported",
			userAgent: "GitHub-Copilot/1.0",
			want:      false,
		},
		{
			name:      "Standard browser",
			userAgent: "Mozilla/5.0",
			want:      false,
		},
		{
			name:      "Empty user agent",
			userAgent: "",
			want:      false,
		},
		{
			name:      "Generic SDK",
			userAgent: "MyApp/1.0.0",
			want:      false,
		},
	}

	detector := &ClientDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.IsLenientClientAllowed(tt.userAgent, "")
			assert.Equal(t, tt.want, got, "IsLenientClientAllowed(%q) = %v, want %v", tt.userAgent, got, tt.want)
		})
	}
}

func TestClientDetector_IsStrictClientAllowed(t *testing.T) {
	tests := []struct {
		name        string
		userAgent   string
		channelType string
		want        bool
	}{
		// Claude Code channel tests
		{
			name:        "Claude CLI allowed for claudecode channel",
			userAgent:   "ClaudeCode/1.0.0",
			channelType: "claudecode",
			want:        true,
		},
		{
			name:        "Codex not allowed for claudecode channel",
			userAgent:   "Codex/1.0",
			channelType: "claudecode",
			want:        false,
		},

		// Codex channel tests
		{
			name:        "Codex CLI allowed for codex channel",
			userAgent:   "Codex/1.0",
			channelType: "codex",
			want:        true,
		},
		{
			name:        "Claude CLI not allowed for codex channel",
			userAgent:   "ClaudeCode/1.0.0",
			channelType: "codex",
			want:        false,
		},

		// GitHub Copilot channel tests
		{
			name:        "Codex allowed for github_copilot channel",
			userAgent:   "Codex/1.0",
			channelType: "github_copilot",
			want:        true,
		},
		{
			name:        "Claude CLI not allowed for github_copilot channel",
			userAgent:   "ClaudeCode/1.0.0",
			channelType: "github_copilot",
			want:        false,
		},

		// Antigravity channel tests
		{
			name:        "Antigravity allowed for antigravity channel",
			userAgent:   "Antigravity/1.0",
			channelType: "antigravity",
			want:        true,
		},

		// OpenCode channel tests
		{
			name:        "OpenCode allowed for opencode_go channel",
			userAgent:   "OpenCode/1.0",
			channelType: "opencode_go",
			want:        true,
		},
		{
			name:        "OpenCode allowed for opencode_go_anthropic channel",
			userAgent:   "OpenCode/1.0",
			channelType: "opencode_go_anthropic",
			want:        true,
		},

		// Moonshot coding channel tests
		{
			name:        "OpenCode allowed for moonshot_coding channel",
			userAgent:   "OpenCode/1.0",
			channelType: "moonshot_coding",
			want:        true,
		},
		{
			name:        "Claude CLI not allowed for moonshot_coding channel",
			userAgent:   "ClaudeCode/1.0.0",
			channelType: "moonshot_coding",
			want:        false,
		},

		// Generic channel types: strict mode matches by API-format family
		{
			name:        "Claude allowed on anthropic channel (same family)",
			userAgent:   "ClaudeCode/1.0.0",
			channelType: "anthropic",
			want:        true,
		},
		{
			name:        "Claude allowed on deepseek_anthropic channel (same family)",
			userAgent:   "claude-cli/2.1.158",
			channelType: "deepseek_anthropic",
			want:        true,
		},
		{
			name:        "Claude rejected on openai channel (cross-family)",
			userAgent:   "claude-cli/2.1.158",
			channelType: "openai",
			want:        false,
		},
		{
			name:        "Codex allowed on openai channel (same family)",
			userAgent:   "Codex/1.0",
			channelType: "openai",
			want:        true,
		},
		{
			name:        "Codex rejected on anthropic channel (cross-family)",
			userAgent:   "Codex/1.0",
			channelType: "anthropic",
			want:        false,
		},
		{
			name:        "OpenCode allowed on openai channel (OpenAI family)",
			userAgent:   "OpenCode/1.0",
			channelType: "openai",
			want:        true,
		},
		{
			name:        "OpenCode rejected on anthropic channel (cross-family)",
			userAgent:   "OpenCode/1.0",
			channelType: "anthropic",
			want:        false,
		},
		{
			name:        "Antigravity allowed on gemini channel (same family)",
			userAgent:   "Antigravity/1.0",
			channelType: "gemini",
			want:        true,
		},
		{
			name:        "Antigravity allowed on gemini_openai channel (Gemini family by name)",
			userAgent:   "Antigravity/1.0",
			channelType: "gemini_openai",
			want:        true,
		},
		{
			name:        "Antigravity rejected on openai channel (cross-family)",
			userAgent:   "Antigravity/1.0",
			channelType: "openai",
			want:        false,
		},
		{
			name:        "Unknown client rejected on generic channel",
			userAgent:   "Mozilla/5.0",
			channelType: "openai",
			want:        false,
		},

		// Unknown clients
		{
			name:        "Unknown client not allowed",
			userAgent:   "Mozilla/5.0",
			channelType: "claudecode",
			want:        false,
		},
		{
			name:        "Empty user agent not allowed",
			userAgent:   "",
			channelType: "claudecode",
			want:        false,
		},
	}

	detector := &ClientDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.IsStrictClientAllowed(tt.userAgent, "", tt.channelType)
			assert.Equal(t, tt.want, got, "IsStrictClientAllowed(%q, %q) = %v, want %v", tt.userAgent, got, tt.want, tt.channelType)
		})
	}
}

func TestClientDetector_EdgeCases(t *testing.T) {
	detector := &ClientDetector{}

	t.Run("Case insensitivity", func(t *testing.T) {
		userAgents := []string{
			"claudecode/1.0",
			"CLAUDECODE/1.0",
			"ClAuDeCoDe/1.0",
		}
		for _, ua := range userAgents {
			got := detector.DetectClient(ua, "")
			assert.Equal(t, "claude", got, "Should detect regardless of case: %s", ua)
		}
	})

	t.Run("Substring matching", func(t *testing.T) {
		userAgents := []string{
			"MyApp-ClaudeCode/1.0",
			"ClaudeCode-Extension/1.0",
			"App/1.0 (ClaudeCode)",
		}
		for _, ua := range userAgents {
			got := detector.DetectClient(ua, "")
			assert.Equal(t, "claude", got, "Should detect substring: %s", ua)
		}
	})
}

func TestSupportedCodingClients(t *testing.T) {
	expectedClients := []string{
		"claude",
		"codex",
		"antigravity",
		"opencode",
	}

	assert.Equal(t, len(expectedClients), len(SupportedCodingClients), "Should have correct number of supported clients")

	for _, expected := range expectedClients {
		found := false
		for _, client := range SupportedCodingClients {
			if client == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Should contain client: %s", expected)
	}
}

func TestChannelClientMapping(t *testing.T) {
	tests := []struct {
		channel string
		clients []string
	}{
		{"claudecode", []string{"claude"}},
		{"codex", []string{"codex"}},
		{"github_copilot", []string{"codex"}},
		{"antigravity", []string{"antigravity"}},
		{"opencode_go", []string{"opencode"}},
		{"opencode_go_anthropic", []string{"opencode"}},
		{"moonshot_coding", []string{"opencode"}},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			clients, exists := ChannelClientMapping[tt.channel]
			assert.True(t, exists, "Channel should exist: %s", tt.channel)
			assert.Equal(t, tt.clients, clients, "Channel %s should have correct client mapping", tt.channel)
		})
	}
}

func TestClientDetector_IsClientAllowedForFamily(t *testing.T) {
	detector := &ClientDetector{}

	tests := []struct {
		name      string
		userAgent string
		family    string
		want      bool
	}{
		{"claude on anthropic family", "claude-cli/2.1.158", familyAnthropic, true},
		{"codex on openai family", "codex-cli/1.0", familyOpenAI, true},
		{"opencode on openai family", "opencode/1.0", familyOpenAI, true},
		{"antigravity on gemini family", "Antigravity/1.0", familyGemini, true},
		{"claude rejected on openai family", "claude-cli/2.1.158", familyOpenAI, false},
		{"codex rejected on anthropic family", "codex-cli/1.0", familyAnthropic, false},
		{"antigravity rejected on anthropic family", "Antigravity/1.0", familyAnthropic, false},
		{"unknown client rejected on anthropic family", "SomeRandomClient/1.0", familyAnthropic, false},
		{"empty UA rejected", "", familyAnthropic, false},
		{"browser UA rejected on openai family", "Mozilla/5.0 Chrome", familyOpenAI, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detector.IsClientAllowedForFamily(tt.userAgent, "", tt.family))
		})
	}
}

func TestFamilyFromRestriction(t *testing.T) {
	tests := []struct {
		level  string
		family string
		ok     bool
	}{
		{"strict_anthropic", familyAnthropic, true},
		{"strict_openai", familyOpenAI, true},
		{"strict_gemini", familyGemini, true},
		{"strict", "", false},
		{"off", "", false},
		{"lenient", "", false},
		{"strict_unknown", "", false},
		{"", "", false},
		{"strict_", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			family, ok := familyFromRestriction(tt.level)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.family, family)
		})
	}
}
