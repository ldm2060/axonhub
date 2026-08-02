package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/privacy"
	"github.com/ldm2060/axonhub/internal/ent/user"
)

// A scopeless authenticated user must not be able to read another tenant's
// request executions, which carry upstream prompt and response bodies.
func TestRequestExecutionPolicy_DeniesUnscopedUser(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:reqexecpolicy?mode=memory&_fk=1")
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	setupCtx := authz.WithTestBypass(ctx)

	_, err := client.User.Create().
		SetEmail("victim@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(1).
		SetModelID("gpt-4").
		SetSource("api").
		SetStatus("completed").
		SetRequestBody([]byte(`{"secret":"victim-prompt"}`)).
		Save(setupCtx)
	require.NoError(t, err)

	_, err = client.RequestExecution.Create().
		SetRequestID(req.ID).
		SetProjectID(1).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetRequestBody([]byte(`{"secret":"victim-prompt"}`)).
		SetResponseBody([]byte(`{"secret":"victim-response"}`)).
		Save(setupCtx)
	require.NoError(t, err)

	attacker, err := client.User.Create().
		SetEmail("attacker@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SetScopes([]string{}).
		Save(setupCtx)
	require.NoError(t, err)

	attackerCtx := contexts.WithUser(ctx, attacker)

	execs, err := client.RequestExecution.Query().All(attackerCtx)
	require.Error(t, err, "scopeless user must not read request executions")
	require.Empty(t, execs)

	// The parent Request is already protected; the execution must behave the same way.
	_, reqErr := client.Request.Query().All(attackerCtx)
	require.Error(t, reqErr)
}

// Email tokens are password-reset credentials and must not be readable or writable
// through the API by any principal — only via the service's system bypass.
func TestEmailTokenPolicy_DeniesAllAPIAccess(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:emailtokenpolicy?mode=memory&_fk=1")
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

	svc := &EmailTokenService{AbstractService: &AbstractService{db: client}}

	token, err := svc.CreateToken(ctx, owner.ID, "reset_password")
	require.NoError(t, err, "service must still be able to mint tokens via bypass")
	require.NotEmpty(t, token)

	// Even a system owner must not read tokens through the ent API.
	ownerCtx := contexts.WithUser(ctx, owner)
	_, err = client.EmailToken.Query().All(ownerCtx)
	require.ErrorIs(t, err, privacy.Deny)

	// A read_users holder must not reach them through the User edge either.
	support, err := client.User.Create().
		SetEmail("support@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SetScopes([]string{"read_users"}).
		Save(setupCtx)
	require.NoError(t, err)

	supportCtx := contexts.WithUser(ctx, support)
	_, err = client.User.Query().
		Where(user.IDEQ(owner.ID)).
		QueryEmailTokens().
		All(supportCtx)
	require.Error(t, err, "email tokens must not be reachable via the user edge")

	// The service path still validates correctly.
	userID, err := svc.ValidateToken(ctx, token, "reset_password")
	require.NoError(t, err)
	require.Equal(t, owner.ID, userID)
}
