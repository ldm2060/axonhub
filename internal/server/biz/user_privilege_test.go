package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

func newUserServiceForPrivilegeTest(t *testing.T) (*UserService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:userprivilege?mode=memory&_fk=1")

	svc := &UserService{
		AbstractService:     &AbstractService{db: client},
		UserCache:           xcache.NewFromConfig[ent.User](xcache.Config{Mode: "memory"}),
		permissionValidator: NewPermissionValidator(),
	}

	return svc, client
}

// A write_users holder must not be able to promote itself (or anyone else) to
// system owner: is_owner short-circuits every privacy policy in the codebase.
func TestUpdateUser_RejectsSelfPromotionToOwner(t *testing.T) {
	svc, client := newUserServiceForPrivilegeTest(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	setupCtx := authz.WithTestBypass(ctx)

	attacker, err := client.User.Create().
		SetEmail("usermanager@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SetScopes([]string{"write_users", "read_users"}).
		Save(setupCtx)
	require.NoError(t, err)

	attackerCtx := contexts.WithUser(ctx, attacker)

	isOwner := true
	_, err = svc.UpdateUser(attackerCtx, attacker.ID, ent.UpdateUserInput{IsOwner: &isOwner})
	require.Error(t, err, "write_users must not be able to grant ownership")
	require.Contains(t, err.Error(), "permission denied")

	reloaded, err := client.User.Get(setupCtx, attacker.ID)
	require.NoError(t, err)
	require.False(t, reloaded.IsOwner, "attacker must not have become an owner")
}

// An owner may still legitimately manage ownership.
func TestUpdateUser_AllowsOwnerToGrantOwnership(t *testing.T) {
	svc, client := newUserServiceForPrivilegeTest(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	setupCtx := authz.WithTestBypass(ctx)

	owner, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SetIsOwner(true).
		Save(setupCtx)
	require.NoError(t, err)

	target, err := client.User.Create().
		SetEmail("promoted@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	ownerCtx := contexts.WithUser(ctx, owner)

	isOwner := true
	updated, err := svc.UpdateUser(ownerCtx, target.ID, ent.UpdateUserInput{IsOwner: &isOwner})
	require.NoError(t, err)
	require.True(t, updated.IsOwner)
}

// Creating a user must not be a way to mint scopes the caller does not hold.
func TestCreateUser_RejectsGrantingUnheldScopes(t *testing.T) {
	svc, client := newUserServiceForPrivilegeTest(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	setupCtx := authz.WithTestBypass(ctx)

	attacker, err := client.User.Create().
		SetEmail("creator@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SetScopes([]string{"write_users"}).
		Save(setupCtx)
	require.NoError(t, err)

	attackerCtx := contexts.WithUser(ctx, attacker)

	_, err = svc.CreateUser(attackerCtx, ent.CreateUserInput{
		Email:    "mule@example.com",
		Password: "password123",
		Scopes:   []string{"write_settings", "write_channels"},
	})
	require.Error(t, err, "must not grant scopes the creator lacks")
	require.Contains(t, err.Error(), "permission denied")

	count, err := client.User.Query().Where(user.EmailEQ("mule@example.com")).Count(setupCtx)
	require.NoError(t, err)
	require.Zero(t, count, "the escalated account must not have been created")
}

// Self-registration and other internal flows run under a system bypass with no user
// in context, and must keep working.
func TestCreateUser_AllowsSystemFlowsWithoutUserInContext(t *testing.T) {
	svc, client := newUserServiceForPrivilegeTest(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	setupCtx := authz.WithTestBypass(ctx)

	created, err := svc.CreateUser(setupCtx, ent.CreateUserInput{
		Email:    "selfsignup@example.com",
		Password: "password123",
		Scopes:   DefaultUserScopes,
	})
	require.NoError(t, err)
	require.Equal(t, "selfsignup@example.com", created.Email)
}
