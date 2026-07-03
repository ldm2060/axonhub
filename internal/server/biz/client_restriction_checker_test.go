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
			name:              "Strict allows supported coding client on openai via fallback",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "openai",
			globalRestriction: ClientRestrictionStrict,
			expected:          true,
		},
		{
			name:              "Strict allows codex client on openai via fallback",
			userAgent:         "codex-cli/1.0",
			channelType:       "openai",
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
			name:        "Strict restriction with unknown channel type",
			channelType: "unknown_channel",
			restriction: ClientRestrictionStrict,
			expected:    "This channel requires requests from supported coding agent clients (Claude, Codex, Antigravity, OpenCode)",
		},
		{
			name:        "Unknown restriction level",
			channelType: "claudecode",
			restriction: "invalid",
			expected:    "Client restriction check failed",
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
