package scopes

import (
	"context"
	"encoding/json"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/entql"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/privacy"
)

// ChannelVisibilityQueryRule allows reading channels/models based on visibility:
//  1. published → all users
//  2. owner_id = current user → own resources
//
// shared_with is checked at application level since JSON array containment is DB-specific.
func ChannelVisibilityQueryRule(readScope ScopeSlug) privacy.QueryRule {
	return privacy.FilterFunc(channelVisibilityFilter(readScope))
}

func channelVisibilityFilter(readScope ScopeSlug) func(ctx context.Context, f privacy.Filter) error {
	return func(ctx context.Context, f privacy.Filter) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if userHasSystemScope(user, readScope) {
			return privacy.Allow
		}

		f.Where(entql.Or(
			entql.FieldEQ("visibility", "published"),
			entql.FieldEQ("owner_id", user.ID),
		))

		return privacy.Allowf("User %d can query visible channels/models", user.ID)
	}
}

// OwnerMutation is implemented by Channel and Model mutations.
type OwnerMutation interface {
	ent.Mutation
	OwnerID() (r int, exists bool)
	WhereP(ps ...func(*sql.Selector))
}

// ChannelManageOwnMutationRule allows users with manage_own_channels/models to
// create/update/delete resources where they are the owner.
func ChannelManageOwnMutationRule(manageScope ScopeSlug) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if !userHasSystemScope(user, manageScope) {
			return privacy.Skipf("User %d does not have scope %s", user.ID, manageScope)
		}

		switch mutation := m.(type) {
		case OwnerMutation:
			switch mutation.Op() {
			case ent.OpCreate:
				ownerID, ok := mutation.OwnerID()
				if !ok || ownerID != user.ID {
					return privacy.Skipf("User %d can only create own resources", user.ID)
				}
				return privacy.Allowf("User %d can create own resources", user.ID)
			case ent.OpUpdateOne, ent.OpDeleteOne, ent.OpUpdate, ent.OpDelete:
				mutation.WhereP(func(s *sql.Selector) {
					s.Where(sql.EQ("owner_id", user.ID))
				})
				return privacy.Allowf("User %d can modify own resources", user.ID)
			default:
				return privacy.Denyf("Unsupported operation %s", mutation.Op())
			}
		default:
			return privacy.Skipf("Not an owner mutation")
		}
	})
}

// IsSharedWithUser checks if a user ID is in the shared_with JSON array.
func IsSharedWithUser(sharedWith []int, userID int) bool {
	if len(sharedWith) == 0 {
		return false
	}
	for _, id := range sharedWith {
		if id == userID {
			return true
		}
	}
	return false
}

// IsSharedWithUserJSON checks if a user ID is in the shared_with raw JSON.
func IsSharedWithUserJSON(sharedWithJSON []byte, userID int) bool {
	if len(sharedWithJSON) == 0 {
		return false
	}
	var ids []int
	if err := json.Unmarshal(sharedWithJSON, &ids); err != nil {
		return false
	}
	return IsSharedWithUser(ids, userID)
}
