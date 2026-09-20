package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/scopes"
)

// actingUserForRouting returns the user a request is routed on behalf of: the
// signed-in user for session requests, otherwise the owner of the request's API
// key. Keys without a user (service account, noauth) yield nil and stay on the
// system-level path.
func actingUserForRouting(ctx context.Context) *ent.User {
	if user, ok := contexts.GetActingUser(ctx); ok && user != nil {
		return user
	}

	return nil
}

// filterCandidatesByChannelAccess drops candidates whose channel the acting user
// may not route through. The enabled-channel cache holds every enabled channel,
// so private channels of other users - and shared channels the user is not on
// the list for - have to be dropped here rather than at load time.
//
// Fork-specific: see .agent/rules/channel-access.md. Do not move this filter into
// resolveAssociations or the channel cache - both are keyed on channel count and
// timestamps and are shared by every request, so a user-scoped filter there would
// poison the cache for whoever asks next.
func filterCandidatesByChannelAccess(ctx context.Context, candidates []*ChannelModelsCandidate) []*ChannelModelsCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	user := actingUserForRouting(ctx)
	if user == nil {
		return candidates
	}

	allowed := lo.Filter(candidates, func(candidate *ChannelModelsCandidate, _ int) bool {
		return candidate != nil && candidate.Channel != nil && scopes.CanRouteThroughChannel(user, candidate.Channel.Channel)
	})

	if len(allowed) != len(candidates) && log.DebugEnabled(ctx) {
		log.Debug(ctx, "dropped channel candidates the request user cannot route through",
			log.Int("user_id", user.ID),
			log.Int("dropped", len(candidates)-len(allowed)),
			log.Int("remaining", len(allowed)),
		)
	}

	return allowed
}
