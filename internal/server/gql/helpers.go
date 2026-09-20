package gql

import (
	"context"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/scopes"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func guidSliceToIntSlice(guids []*objects.GUID) []int {
	ids := make([]int, 0, len(guids))
	for _, g := range guids {
		ids = append(ids, g.ID)
	}
	return ids
}

// sharedUsersFromInfos maps biz identities onto the GraphQL SharedUser type.
func sharedUsersFromInfos(infos []*biz.SharedUserInfo) []*SharedUser {
	return lo.Map(infos, func(info *biz.SharedUserInfo, _ int) *SharedUser {
		return &SharedUser{
			ID:        objects.GUID{Type: ent.TypeUser, ID: info.ID},
			Email:     info.Email,
			FirstName: optionalStr(info.FirstName),
			LastName:  optionalStr(info.LastName),
		}
	})
}

func optionalStr(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// canManageChannelSharing reports whether the caller may see and change who a
// channel is shared with. The read_channels privacy bypass lets every channel
// reader see all channels, so the owner check has to happen here instead.
func canManageChannelSharing(ctx context.Context, ch *ent.Channel) bool {
	if ch == nil {
		return false
	}

	if scopes.UserHasScope(ctx, scopes.ScopeWriteChannels) {
		return true
	}

	user, ok := contexts.GetUser(ctx)

	return ok && user != nil && ch.OwnerID != 0 && ch.OwnerID == user.ID
}

// canShareResources reports whether the caller may use the sharing picker: either
// they can read the user directory, or they own resources that can be shared.
func canShareResources(ctx context.Context) bool {
	return scopes.UserHasScope(ctx, scopes.ScopeReadUsers) ||
		scopes.UserHasScope(ctx, scopes.ScopeManageOwnChannels) ||
		scopes.UserHasScope(ctx, scopes.ScopeManageOwnModels)
}
