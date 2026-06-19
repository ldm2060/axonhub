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
			channelType:       "openai",
			globalRestriction: ClientRestrictionOff,
			expected:          true,
		},
		{
			name:              "Lenient allows any supported client",
			userAgent:         "claude-cli/2.1.158",
			channelType:       "openai",
			globalRestriction: ClientRestrictionLenient,
			expected:          true,
		},
		{
			name:              "Lenient allows codex client on openai channel",
			userAgent:         "codex-cli/1.0",
			channelType:       "openai",
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
			name:               "Channel restriction overrides global",
			userAgent:          "Mozilla/5.0",
			channelType:        "openai",
			channelRestriction: ptr(ClientRestrictionOff),
			globalRestriction:  ClientRestrictionStrict,
			expected:           true,
		},
		{
			name:              "Restriction applies to non-coding channels too",
			userAgent:         "Mozilla/5.0",
			channelType:       "openai",
			globalRestriction: ClientRestrictionLenient,
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckClientRestriction(
				tt.userAgent,
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
			expected:    "This channel has strict client restriction but no allowed clients are defined",
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
