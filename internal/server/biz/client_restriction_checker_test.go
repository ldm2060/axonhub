package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientRestrictionChecker_CheckClientRestriction(t *testing.T) {
	checker := NewClientRestrictionChecker()

	tests := []struct {
		name               string
		userAgent          string
		channelType        string
		channelRestriction *ClientRestrictionLevel
		globalRestriction  ClientRestrictionLevel
		expected           bool
	}{
		{
			name:              "Off mode allows all",
			userAgent:         "Mozilla/5.0",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionOff,
			expected:          true,
		},
		{
			name:              "Lenient allows any supported client",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionLenient,
			expected:          true,
		},
		{
			name:              "Lenient allows codex client on coding channel",
			userAgent:         "codex-cli/1.0",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionLenient,
			expected:          true,
		},
		{
			name:              "Lenient rejects non-coding client",
			userAgent:         "Mozilla/5.0 Chrome",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionLenient,
			expected:          false,
		},
		{
			name:              "Lenient rejects unsupported coding client",
			userAgent:         "cursor/0.41.0",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionLenient,
			expected:          false,
		},
		{
			name:              "Strict allows matching client",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		{
			name:              "Strict rejects mismatched client",
			userAgent:         "codex-cli/1.0",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionStrict,
			expected:          false,
		},
		{
			name:               "Channel restriction off overrides global strict on non-coding channel",
			userAgent:          "Mozilla/5.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionOff),
			globalRestriction:  ClientRestrictionStrict,
			expected:           true,
		},
		// Non-coding channel (openai) - restrictions now apply globally
		{
			name:              "Lenient rejects ordinary browser client on openai",
			userAgent:         "Mozilla/5.0 Chrome",
			channelType:       "openai",
			globalRestriction: ClientRestrictionLenient,
			expected:          false,
		},
		{
			name:              "Lenient rejects random client on openai",
			userAgent:         "SomeRandomClient/1.0",
			channelType:       "openai",
			globalRestriction: ClientRestrictionLenient,
			expected:          false,
		},
		{
			name:              "Lenient allows supported coding client on openai",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "openai",
			globalRestriction: ClientRestrictionLenient,
			expected:          true,
		},
		{
			name:              "Lenient allows codex client on openai",
			userAgent:         "codex-cli/1.0",
			channelType:       "openai",
			globalRestriction: ClientRestrictionLenient,
			expected:          true,
		},
		{
			name:              "Strict rejects ordinary browser client on openai",
			userAgent:         "Mozilla/5.0 Chrome",
			channelType:       "openai",
			globalRestriction: ClientRestrictionStrict,
			expected:          false,
		},
		{
			name:              "Strict rejects random client on openai",
			userAgent:         "SomeRandomClient/1.0",
			channelType:       "openai",
			globalRestriction: ClientRestrictionStrict,
			expected:          false,
		},
		{
			name:              "Strict rejects claude client on openai (cross-family)",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "openai",
			globalRestriction: ClientRestrictionStrict,
			expected:          false,
		},
		{
			name:              "Strict allows codex client on openai (same family)",
			userAgent:         "codex-cli/1.0",
			channelType:       "openai",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		{
			name:              "Strict allows claude client on anthropic channel (same family)",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "anthropic",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		{
			name:              "Strict allows claude client on deepseek_anthropic (same family)",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "deepseek_anthropic",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		{
			name:              "Strict rejects antigravity on openai (cross-family)",
			userAgent:         "Antigravity/1.0",
			channelType:       "openai",
			globalRestriction: ClientRestrictionStrict,
			expected:          false,
		},
		{
			name:              "Strict allows antigravity on gemini channel (same family)",
			userAgent:         "Antigravity/1.0",
			channelType:       "gemini",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		{
			name:               "Channel-level off override still allows on openai when global strict would reject",
			userAgent:          "Mozilla/5.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionOff),
			globalRestriction:  ClientRestrictionStrict,
			expected:           true,
		},
		{
			name:               "Channel-level lenient override on openai rejects ordinary clients when global is off",
			userAgent:          "Mozilla/5.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLenient),
			globalRestriction:  ClientRestrictionOff,
			expected:           false,
		},
		{
			name:               "Channel-level lenient override on openai allows supported coding client when global is off",
			userAgent:          "claude-cli/2.1.158",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLenient),
			globalRestriction:  ClientRestrictionOff,
			expected:           true,
		},
		// Existing strict same-family behavior for coding channels
		{
			name:              "Coding channel still enforces strict restriction",
			userAgent:         "codex-cli/1.0",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionStrict,
			expected:          false,
		},
		{
			name:              "Strict allows matching client on coding channel",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "claudecode",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		// strict_<family> modes force a specific family regardless of channel
		// type or ChannelClientMapping. Channel-level only.
		{
			name:               "strict_anthropic allows claude on generic openai channel",
			userAgent:          "claude-cli/2.1.158",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLevel("strict_anthropic")),
			globalRestriction:  ClientRestrictionOff,
			expected:           true,
		},
		{
			name:               "strict_anthropic rejects codex on generic openai channel",
			userAgent:          "codex-cli/1.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLevel("strict_anthropic")),
			globalRestriction:  ClientRestrictionOff,
			expected:           false,
		},
		{
			name:               "strict_openai allows codex on anthropic-named channel (overrides name inference)",
			userAgent:          "codex-cli/1.0",
			channelType:        "anthropic",
			channelRestriction: ptr(ClientRestrictionLevel("strict_openai")),
			globalRestriction:  ClientRestrictionOff,
			expected:           true,
		},
		{
			name:               "strict_openai allows opencode",
			userAgent:          "opencode/1.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLevel("strict_openai")),
			globalRestriction:  ClientRestrictionOff,
			expected:           true,
		},
		{
			name:               "strict_openai overrides ChannelClientMapping on claudecode",
			userAgent:          "codex-cli/1.0",
			channelType:        "claudecode",
			channelRestriction: ptr(ClientRestrictionLevel("strict_openai")),
			globalRestriction:  ClientRestrictionOff,
			expected:           true,
		},
		{
			name:               "strict_openai rejects claude even on claudecode (mapping overridden)",
			userAgent:          "claude-cli/2.1.158",
			channelType:        "claudecode",
			channelRestriction: ptr(ClientRestrictionLevel("strict_openai")),
			globalRestriction:  ClientRestrictionOff,
			expected:           false,
		},
		{
			name:               "strict_gemini allows antigravity on openai channel",
			userAgent:          "Antigravity/1.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLevel("strict_gemini")),
			globalRestriction:  ClientRestrictionOff,
			expected:           true,
		},
		{
			name:               "strict_gemini rejects claude",
			userAgent:          "claude-cli/2.1.158",
			channelType:        "gemini",
			channelRestriction: ptr(ClientRestrictionLevel("strict_gemini")),
			globalRestriction:  ClientRestrictionOff,
			expected:           false,
		},
		{
			name:               "strict_anthropic rejects unknown client",
			userAgent:          "SomeRandomClient/1.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionLevel("strict_anthropic")),
			globalRestriction:  ClientRestrictionOff,
			expected:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckClientRestriction(
				tt.userAgent,
				"",
				tt.channelType,
				tt.channelRestriction,
				tt.globalRestriction,
			)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestClientRestrictionChecker_GetRejectionReason(t *testing.T) {
	checker := NewClientRestrictionChecker()

	tests := []struct {
		name        string
		channelType string
		restriction ClientRestrictionLevel
		expected    string
	}{
		{
			name:        "Lenient restriction message",
			channelType: "claudecode",
			restriction: ClientRestrictionLenient,
			expected:    "This channel requires requests from supported coding agent clients (Claude, Codex, Antigravity, OpenCode)",
		},
		{
			name:        "Strict restriction with valid channel",
			channelType: "claudecode",
			restriction: ClientRestrictionStrict,
			expected:    "This channel only accepts requests from: claude",
		},
		{
			name:        "Strict restriction with multiple allowed clients",
			channelType: "github_copilot",
			restriction: ClientRestrictionStrict,
			expected:    "This channel only accepts requests from: codex",
		},
		{
			name:        "Strict restriction with unknown openai-family channel type",
			channelType: "unknown_channel",
			restriction: ClientRestrictionStrict,
			expected:    "This channel only accepts requests from OpenAI-family clients (Codex, OpenCode)",
		},
		{
			name:        "Strict restriction with anthropic-family channel type",
			channelType: "deepseek_anthropic",
			restriction: ClientRestrictionStrict,
			expected:    "This channel only accepts requests from Anthropic-family clients (Claude)",
		},
		{
			name:        "Strict restriction with gemini-family channel type",
			channelType: "gemini",
			restriction: ClientRestrictionStrict,
			expected:    "This channel only accepts requests from Gemini-family clients (Antigravity)",
		},
		{
			name:        "Unknown restriction level",
			channelType: "claudecode",
			restriction: "invalid",
			expected:    "Client restriction check failed",
		},
		{
			name:        "strict_anthropic rejection message",
			channelType: "openai",
			restriction: ClientRestrictionLevel("strict_anthropic"),
			expected:    "This channel only accepts requests from Anthropic-family clients (Claude)",
		},
		{
			name:        "strict_openai rejection message",
			channelType: "openai",
			restriction: ClientRestrictionLevel("strict_openai"),
			expected:    "This channel only accepts requests from OpenAI-family clients (Codex, OpenCode)",
		},
		{
			name:        "strict_gemini rejection message",
			channelType: "openai",
			restriction: ClientRestrictionLevel("strict_gemini"),
			expected:    "This channel only accepts requests from Gemini-family clients (Antigravity)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.GetRejectionReason(tt.channelType, tt.restriction)
			require.Equal(t, tt.expected, result)
		})
	}
}

func ptr[T any](v T) *T { return &v }
