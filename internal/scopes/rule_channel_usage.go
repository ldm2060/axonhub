package scopes

import (
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
)

// CanRouteThroughChannel reports whether a request acting as user may be routed
// through ch.
//
// Fork-specific rule (upstream has no channel access control): see
// .agent/rules/channel-access.md and
// .agent/summary/2026-09-20-channel-sharing-and-access-design.md. Keep this the
// single source of truth for routing and model listing, and do not fold
// read_channels into it - self-registered users hold that scope by default.
//
// Visibility decides who may see a channel; this decides who may use it:
//   - a nil user is a system-level principal (service account, noauth key,
//     background task) and keeps unrestricted access
//   - the system owner and users holding write_channels manage every channel
//   - the owner of a channel may always route through it
//   - published channels are open to everyone
//   - shared channels are open to the users listed in shared_with
//   - an unowned channel (owner_id unset, e.g. created before ownership existed)
//     is a system channel and stays open to everyone
func CanRouteThroughChannel(user *ent.User, ch *ent.Channel) bool {
	if user == nil || ch == nil {
		return true
	}

	if user.IsOwner || HasSystemScope(user, ScopeWriteChannels) {
		return true
	}

	if ch.OwnerID == 0 {
		return true
	}

	if ch.OwnerID == user.ID {
		return true
	}

	switch ch.Visibility {
	case channel.VisibilityPublished:
		return true
	case channel.VisibilityShared:
		return IsSharedWithUser(ch.SharedWith, user.ID)
	default:
		return false
	}
}
