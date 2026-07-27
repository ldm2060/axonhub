package biz

import (
	"strings"
)

// ClientRestrictionChecker evaluates client restriction rules
type ClientRestrictionChecker struct {
	detector *ClientDetector
}

// NewClientRestrictionChecker creates a new checker instance
func NewClientRestrictionChecker() *ClientRestrictionChecker {
	return &ClientRestrictionChecker{
		detector: &ClientDetector{},
	}
}

// CheckClientRestriction checks if client satisfies channel's access restriction
// Returns true if allowed, false if rejected.
// Restrictions apply to all channel types. For strict mode on unmapped channel types,
// the check falls back to the lenient supported-client check.
func (c *ClientRestrictionChecker) CheckClientRestriction(
	userAgent string,
	referer string,
	channelType string,
	channelRestriction *ClientRestrictionLevel,
	globalRestriction ClientRestrictionLevel,
) bool {
	// Determine effective restriction (channel overrides global)
	effectiveRestriction := globalRestriction
	if channelRestriction != nil {
		effectiveRestriction = *channelRestriction
	}

	// Evaluate restriction
	switch effectiveRestriction {
	case ClientRestrictionOff:
		return true
	case ClientRestrictionLenient:
		return c.detector.IsLenientClientAllowed(userAgent, referer)
	case ClientRestrictionStrict:
		return c.detector.IsStrictClientAllowed(userAgent, referer, channelType)
	default:
		return false
	}
}

// GetRejectionReason returns human-readable rejection reason
func (c *ClientRestrictionChecker) GetRejectionReason(
	channelType string,
	restriction ClientRestrictionLevel,
) string {
	switch restriction {
	case ClientRestrictionLenient:
		return "This channel requires requests from supported coding agent clients (Claude, Codex, Antigravity, OpenCode)"
	case ClientRestrictionStrict:
		if allowedClients, exists := ChannelClientMapping[channelType]; exists && len(allowedClients) > 0 {
			return "This channel only accepts requests from: " + strings.Join(allowedClients, ", ")
		}
		switch channelFamily(channelType) {
		case familyAnthropic:
			return "This channel only accepts requests from Anthropic-family clients (Claude)"
		case familyGemini:
			return "This channel only accepts requests from Gemini-family clients (Antigravity)"
		default:
			return "This channel only accepts requests from OpenAI-family clients (Codex, OpenCode)"
		}
	default:
		return "Client restriction check failed"
	}
}
