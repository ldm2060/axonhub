package scopes

import (
	"context"
	"github.com/ldm2060/axonhub/internal/authz/identity"

	"github.com/ldm2060/axonhub/internal/ent/privacy"
)

// OwnerRule allows owner users to access all functionality.
func OwnerRule() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		if identity.IsPlugin(ctx) {
			return privacy.Skip
		}
		user, err := getUserFromContext(ctx)
		if err != nil {
			return privacy.Skipf("User not found in context")
		}

		// Owner users have all permissions
		if user.IsOwner {
			return privacy.Allow
		}

		return privacy.Skip
	})
}
