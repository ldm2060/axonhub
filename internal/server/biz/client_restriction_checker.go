package biz

import (
	"strings"

	"github.com/ldm2060/axonhub/internal/objects"
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
// Returns true if allowed, false if rejected
func (c *ClientRestrictionChecker) CheckClientRestriction(
	userAgent string,
	channelType string,
	channelRestriction *ClientRestrictionLevel,
	globalRestriction ClientRestrictionLevel,
) bool {
	// Non-coding channels are not subject to client restrictions
	if !objects.IsCodingChannel(channelType) {
		return true
	}

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
		return c.detector.IsLenientClientAllowed(userAgent)
	case ClientRestrictionStrict:
		return c.detector.IsStrictClientAllowed(userAgent, channelType)
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
		return "This channel requires requests from supported coding agent clients (Claude Code, Codex, Cursor, Aider, etc.)"
	case ClientRestrictionStrict:
		allowedClients := ChannelClientMapping[channelType]
		if len(allowedClients) == 0 {
			return "This channel has strict client restriction but no allowed clients are defined"
		}
		return "This channel only accepts requests from: " + strings.Join(allowedClients, ", ")
	default:
		return "Client restriction check failed"
	}
}
