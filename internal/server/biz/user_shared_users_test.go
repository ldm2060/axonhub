package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/user"
)

func setupSharedUserTestContext(t *testing.T, client *ent.Client) context.Context {
	t.Helper()

	ctx := authz.WithTestBypass(context.Background())

	return ent.NewContext(ctx, client)
}

func TestUserService_SharedUsersByIDs_KeepsSharedWithOrderAndDropsMissingUsers(t *testing.T) {
	svc, client := setupTestUserService(t)
	defer client.Close()

	ctx := setupSharedUserTestContext(t, client)

	first, err := client.User.Create().
		SetEmail("share-first@example.com").
		SetPassword("password").
		SetFirstName("First").
		SetLastName("Shared").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	second, err := client.User.Create().
		SetEmail("share-second@example.com").
		SetPassword("password").
		SetFirstName("Second").
		SetLastName("Shared").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	owner, err := client.User.Create().
		SetEmail("share-owner@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// 99999 is not in the table: a stale shared_with entry must simply be skipped.
	infos, err := svc.SharedUsersByIDs(contexts.WithUser(ctx, owner), []int{second.ID, 99999, first.ID, second.ID})
	require.NoError(t, err)
	require.Len(t, infos, 2)
	require.Equal(t, second.ID, infos[0].ID)
	require.Equal(t, first.ID, infos[1].ID)
	require.Equal(t, "share-second@example.com", infos[0].Email)
	require.Equal(t, "First", infos[1].FirstName)
	require.Equal(t, "Shared", infos[1].LastName)
}

func TestUserService_SharedUsersByIDs_EmptyListReturnsEmptySlice(t *testing.T) {
	svc, client := setupTestUserService(t)
	defer client.Close()

	ctx := setupSharedUserTestContext(t, client)

	infos, err := svc.SharedUsersByIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, infos)
}

func TestUserService_SearchShareableUsers_MatchesNameAndEmailAndSkipsDeactivated(t *testing.T) {
	svc, client := setupTestUserService(t)
	defer client.Close()

	ctx := setupSharedUserTestContext(t, client)

	alice, err := client.User.Create().
		SetEmail("alice@example.com").
		SetPassword("password").
		SetFirstName("Alice").
		SetLastName("Anderson").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.User.Create().
		SetEmail("bob@example.com").
		SetPassword("password").
		SetFirstName("Bob").
		SetLastName("Brown").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.User.Create().
		SetEmail("alice.pending@example.com").
		SetPassword("password").
		SetFirstName("Alice").
		SetLastName("Pending").
		SetStatus(user.StatusPending).
		Save(ctx)
	require.NoError(t, err)

	byEmail, err := svc.SearchShareableUsers(ctx, "alice@example.com", 0)
	require.NoError(t, err)
	require.Len(t, byEmail, 1)
	require.Equal(t, alice.ID, byEmail[0].ID)

	byLastName, err := svc.SearchShareableUsers(ctx, "brown", 0)
	require.NoError(t, err)
	require.Len(t, byLastName, 1)
	require.Equal(t, "bob@example.com", byLastName[0].Email)

	// The search must never surface a deactivated or pending account.
	all, err := svc.SearchShareableUsers(ctx, "", 0)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.NotContains(t, []string{all[0].Email, all[1].Email}, "alice.pending@example.com")

	limited, err := svc.SearchShareableUsers(ctx, "", 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}
