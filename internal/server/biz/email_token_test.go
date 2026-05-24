package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
)

func setupTestEmailTokenService(t *testing.T) (*EmailTokenService, *ent.Client, context.Context) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)
	svc := &EmailTokenService{AbstractService: &AbstractService{db: client}}
	return svc, client, ctx
}

func TestEmailTokenService_CreateAndValidateEmailCode(t *testing.T) {
	svc, client, ctx := setupTestEmailTokenService(t)
	defer client.Close()

	code, err := svc.CreateEmailCode(ctx, "User@Example.COM", emailtoken.TypeVerifyEmail, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, code, 6)

	token, err := svc.ValidateEmailCode(ctx, "user@example.com", code, emailtoken.TypeVerifyEmail)
	require.NoError(t, err)
	require.NotNil(t, token.Email)
	require.Equal(t, "user@example.com", *token.Email)
	require.Nil(t, token.UserID)
}

func TestEmailTokenService_CreateEmailCodeSupersedesPreviousCode(t *testing.T) {
	svc, client, ctx := setupTestEmailTokenService(t)
	defer client.Close()

	oldCode, err := svc.CreateEmailCode(ctx, "user@example.com", emailtoken.TypeVerifyEmail, 5*time.Minute)
	require.NoError(t, err)
	newCode, err := svc.CreateEmailCode(ctx, "user@example.com", emailtoken.TypeVerifyEmail, 5*time.Minute)
	require.NoError(t, err)

	_, err = svc.ValidateEmailCode(ctx, "user@example.com", oldCode, emailtoken.TypeVerifyEmail)
	require.Error(t, err)

	_, err = svc.ValidateEmailCode(ctx, "user@example.com", newCode, emailtoken.TypeVerifyEmail)
	require.NoError(t, err)

	count, err := client.EmailToken.Query().
		Where(
			emailtoken.EmailEQ("user@example.com"),
			emailtoken.TypeEQ(emailtoken.TypeVerifyEmail),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestEmailTokenService_ValidateEmailCodeRejectsWrongExpiredAndConsumedCodes(t *testing.T) {
	svc, client, ctx := setupTestEmailTokenService(t)
	defer client.Close()

	code, err := svc.CreateEmailCode(ctx, "user@example.com", emailtoken.TypeVerifyEmail, 5*time.Minute)
	require.NoError(t, err)

	_, err = svc.ValidateEmailCode(ctx, "other@example.com", code, emailtoken.TypeVerifyEmail)
	require.Error(t, err)

	_, err = svc.ValidateEmailCode(ctx, "user@example.com", "000000", emailtoken.TypeVerifyEmail)
	require.Error(t, err)

	err = svc.ConsumeEmailCode(ctx, "user@example.com", code, emailtoken.TypeVerifyEmail)
	require.NoError(t, err)

	_, err = svc.ValidateEmailCode(ctx, "user@example.com", code, emailtoken.TypeVerifyEmail)
	require.Error(t, err)
}

func TestEmailTokenService_ValidateEmailCodeRejectsExpiredCode(t *testing.T) {
	svc, client, ctx := setupTestEmailTokenService(t)
	defer client.Close()

	code, err := svc.CreateEmailCode(ctx, "user@example.com", emailtoken.TypeVerifyEmail, 5*time.Minute)
	require.NoError(t, err)

	persistedToken, err := client.EmailToken.Query().
		Where(emailtoken.Token(code)).
		Only(ctx)
	require.NoError(t, err)

	err = client.EmailToken.UpdateOne(persistedToken).
		SetExpiresAt(time.Now().UTC().Add(-time.Minute)).
		Exec(ctx)
	require.NoError(t, err)

	_, err = svc.ValidateEmailCode(ctx, "user@example.com", code, emailtoken.TypeVerifyEmail)
	require.Error(t, err)
}
