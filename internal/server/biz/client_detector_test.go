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
			want:      "claude-cli",
		},
		{
			name:      "Claude Code - case insensitive",
			userAgent: "claudecode/2.5.1 (Windows)",
			want:      "claude-cli",
		},
		{
			name:      "Claude CLI explicit",
			userAgent: "claude-cli/1.0.0",
			want:      "claude-cli",
		},

		// Codex
		{
			name:      "Codex",
			userAgent: "Codex/1.0",
			want:      "codex-cli",
		},
		{
			name:      "Codex CLI",
			userAgent: "codex-cli/1.0",
			want:      "codex-cli",
		},

		// Cursor
		{
			name:      "Cursor",
			userAgent: "Cursor/0.32.0",
			want:      "cursor",
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

		// Other coding agents
		{
			name:      "GitHub Copilot",
			userAgent: "GitHub-Copilot/1.0",
			want:      "github-copilot",
		},
		{
			name:      "Copilot",
			userAgent: "Copilot/1.0",
			want:      "copilot",
		},
		{
			name:      "Cline",
			userAgent: "Cline/3.1.0",
			want:      "cline",
		},
		{
			name:      "Aider",
			userAgent: "aider/0.45.0",
			want:      "aider",
		},
		{
			name:      "Continue",
			userAgent: "Continue-VSCode/1.0",
			want:      "continue",
		},
		{
			name:      "Windsurf",
			userAgent: "Windsurf/1.2.0",
			want:      "windsurf",
		},
		{
			name:      "Cody",
			userAgent: "Cody/1.0",
			want:      "cody",
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
	}

	detector := &ClientDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.DetectClient(tt.userAgent)
			assert.Equal(t, tt.want, got, "DetectClient(%q) = %v, want %v", tt.userAgent, got, tt.want)
		})
	}
}

func TestClientDetector_IsLenientClientAllowed(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      bool
	}{
		// Should allow all coding agents
		{
			name:      "Claude Code",
			userAgent: "ClaudeCode/1.0.0",
			want:      true,
		},
		{
			name:      "Cursor",
			userAgent: "Cursor/0.32.0",
			want:      true,
		},
		{
			name:      "Windsurf",
			userAgent: "Windsurf/1.2.0",
			want:      true,
		},
		{
			name:      "GitHub Copilot",
			userAgent: "GitHub-Copilot/1.0",
			want:      true,
		},
		{
			name:      "Cline",
			userAgent: "Cline/3.1.0",
			want:      true,
		},
		{
			name:      "Aider",
			userAgent: "aider/0.45.0",
			want:      true,
		},

		// Should reject non-coding-agent clients
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
			got := detector.IsLenientClientAllowed(tt.userAgent)
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
			name:        "Cursor not allowed for claudecode channel",
			userAgent:   "Cursor/0.32.0",
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
			name:        "GitHub Copilot allowed for github_copilot channel",
			userAgent:   "GitHub-Copilot/1.0",
			channelType: "github_copilot",
			want:        true,
		},
		{
			name:        "Copilot allowed for github_copilot channel",
			userAgent:   "Copilot/1.0",
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

		// Unknown channel type
		{
			name:        "Unknown channel type",
			userAgent:   "ClaudeCode/1.0.0",
			channelType: "unknown_channel",
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
			got := detector.IsStrictClientAllowed(tt.userAgent, tt.channelType)
			assert.Equal(t, tt.want, got, "IsStrictClientAllowed(%q, %q) = %v, want %v", tt.userAgent, tt.channelType, got, tt.want)
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
			got := detector.DetectClient(ua)
			assert.Equal(t, "claude-cli", got, "Should detect regardless of case: %s", ua)
		}
	})

	t.Run("Substring matching", func(t *testing.T) {
		userAgents := []string{
			"MyApp-ClaudeCode/1.0",
			"ClaudeCode-Extension/1.0",
			"App/1.0 (ClaudeCode)",
		}
		for _, ua := range userAgents {
			got := detector.DetectClient(ua)
			assert.Equal(t, "claude-cli", got, "Should detect substring: %s", ua)
		}
	})
}

func TestSupportedCodingClients(t *testing.T) {
	// Verify all supported clients are present
	expectedClients := []string{
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
	// Verify channel mappings
	tests := []struct {
		channel string
		clients []string
	}{
		{"claudecode", []string{"claude-cli"}},
		{"codex", []string{"codex-cli"}},
		{"github_copilot", []string{"copilot", "github-copilot"}},
		{"antigravity", []string{"antigravity"}},
		{"opencode_go", []string{"opencode"}},
		{"opencode_go_anthropic", []string{"opencode"}},
		{"moonshot_coding", []string{"moonshot-cli"}},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			clients, exists := ChannelClientMapping[tt.channel]
			assert.True(t, exists, "Channel should exist: %s", tt.channel)
			assert.Equal(t, tt.clients, clients, "Channel %s should have correct client mapping", tt.channel)
		})
	}
}
