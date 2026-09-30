package biz

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/oauthclient"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

const (
	testOAuthClientID          = "ahc_test-client"
	testOAuthClientSecret      = "ahs_test-client-secret"
	testOAuthRedirectURI       = "https://app.example.com/oauth/callback"
	testOAuthProviderPublicURL = "https://axonhub.example.com"
)

func setupTestOAuthProviderService(t *testing.T) (*OAuthProviderService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	systemService := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: cacheConfig})
	userService := NewUserService(UserServiceParams{Ent: client, CacheConfig: cacheConfig, SystemService: systemService})

	svc := NewOAuthProviderService(OAuthProviderServiceParams{
		Ent:           client,
		CacheConfig:   cacheConfig,
		SystemService: systemService,
		UserService:   userService,
		PublicURL:     testOAuthProviderPublicURL,
	})

	return svc, client
}

func setupTestOAuthUserAndClient(t *testing.T, client *ent.Client, ctx context.Context) (*ent.User, *ent.OAuthClient) {
	t.Helper()

	u, err := client.User.Create().
		SetEmail("ada@example.com").
		SetPassword("hashed").
		SetStatus(user.StatusActivated).
		SetFirstName("Ada").
		SetLastName("Lovelace").
		SetEmailVerifiedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	clientEntity, err := client.OAuthClient.Create().
		SetName("Test Application").
		SetDescription("test").
		SetClientID(testOAuthClientID).
		SetClientSecretHash(hashOAuthSecret(testOAuthClientSecret)).
		SetRedirectUris([]string{testOAuthRedirectURI}).
		SetClientType(oauthclient.ClientTypeConfidential).
		SetUserID(u.ID).
		Save(ctx)
	require.NoError(t, err)

	return u, clientEntity
}

func createTestAuthorizationCode(
	t *testing.T,
	svc *OAuthProviderService,
	clientEntity *ent.OAuthClient,
	ctx context.Context,
	u *ent.User,
	codeVerifier string,
) string {
	t.Helper()

	sum := sha256.Sum256([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	requestID, err := svc.CreateAuthorizationRequest(ctx, clientEntity, AuthorizationRequestParams{
		ClientID:            testOAuthClientID,
		RedirectURI:         testOAuthRedirectURI,
		ResponseType:        "code",
		Scope:               "openid email profile",
		State:               "state-123",
		Nonce:               "nonce-123",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	redirectURL, err := svc.ApproveAuthorization(ctx, requestID, u)
	require.NoError(t, err)

	parsed, err := url.Parse(redirectURL)
	require.NoError(t, err)
	require.Equal(t, "state-123", parsed.Query().Get("state"))

	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)

	return code
}

func TestOAuthProviderAuthorizationCodeFlow(t *testing.T) {
	svc, client := setupTestOAuthProviderService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	u, clientEntity := setupTestOAuthUserAndClient(t, client, ctx)

	codeVerifier := "test-code-verifier-1234567890abcdef"

	// Happy path: full authorization code exchange.
	code := createTestAuthorizationCode(t, svc, clientEntity, ctx, u, codeVerifier)

	tokenResponse, err := svc.ExchangeAuthorizationCode(ctx, TokenExchangeParams{
		ClientID:     testOAuthClientID,
		ClientSecret: testOAuthClientSecret,
		Code:         code,
		RedirectURI:  testOAuthRedirectURI,
		CodeVerifier: codeVerifier,
	})
	require.NoError(t, err)
	require.NotEmpty(t, tokenResponse.AccessToken)
	require.NotEmpty(t, tokenResponse.IDToken)
	require.Equal(t, "Bearer", tokenResponse.TokenType)
	require.Equal(t, "openid email profile", tokenResponse.Scope)

	// Authorization codes are single-use.
	_, err = svc.ExchangeAuthorizationCode(ctx, TokenExchangeParams{
		ClientID:     testOAuthClientID,
		ClientSecret: testOAuthClientSecret,
		Code:         code,
		RedirectURI:  testOAuthRedirectURI,
		CodeVerifier: codeVerifier,
	})
	require.Error(t, err)

	// The id_token must be verifiable with the published JWKS.
	claims := verifyTestIDToken(t, svc, ctx, tokenResponse.IDToken)
	require.Equal(t, "1", claims["sub"])
	require.Equal(t, testOAuthClientID, claims["aud"])
	require.Equal(t, testOAuthProviderPublicURL, claims["iss"])
	require.Equal(t, "nonce-123", claims["nonce"])
	require.Equal(t, "ada@example.com", claims["email"])
	require.Equal(t, true, claims["email_verified"])
	require.Equal(t, "Ada Lovelace", claims["name"])

	// userinfo returns the same subject and profile claims.
	userInfo, err := svc.UserInfo(ctx, tokenResponse.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "1", userInfo["sub"])
	require.Equal(t, "ada@example.com", userInfo["email"])
	require.Equal(t, "Ada Lovelace", userInfo["name"])
}

func TestOAuthProviderExchangeRejectsInvalidRequests(t *testing.T) {
	svc, client := setupTestOAuthProviderService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	u, clientEntity := setupTestOAuthUserAndClient(t, client, ctx)

	codeVerifier := "test-code-verifier-1234567890abcdef"

	assertExchangeFails := func(t *testing.T, params TokenExchangeParams) {
		t.Helper()

		_, err := svc.ExchangeAuthorizationCode(ctx, params)
		require.Error(t, err)
	}

	// Wrong client secret.
	code := createTestAuthorizationCode(t, svc, clientEntity, ctx, u, codeVerifier)
	assertExchangeFails(t, TokenExchangeParams{
		ClientID:     testOAuthClientID,
		ClientSecret: "ahs_wrong-secret",
		Code:         code,
		RedirectURI:  testOAuthRedirectURI,
		CodeVerifier: codeVerifier,
	})

	// Wrong PKCE verifier.
	code = createTestAuthorizationCode(t, svc, clientEntity, ctx, u, codeVerifier)
	assertExchangeFails(t, TokenExchangeParams{
		ClientID:     testOAuthClientID,
		ClientSecret: testOAuthClientSecret,
		Code:         code,
		RedirectURI:  testOAuthRedirectURI,
		CodeVerifier: "a-different-verifier",
	})

	// redirect_uri mismatch.
	code = createTestAuthorizationCode(t, svc, clientEntity, ctx, u, codeVerifier)
	assertExchangeFails(t, TokenExchangeParams{
		ClientID:     testOAuthClientID,
		ClientSecret: testOAuthClientSecret,
		Code:         code,
		RedirectURI:  "https://app.example.com/other",
		CodeVerifier: codeVerifier,
	})
}

func TestOAuthProviderClientValidation(t *testing.T) {
	svc, client := setupTestOAuthProviderService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	setupTestOAuthUserAndClient(t, client, ctx)

	// Exact match required.
	_, err := svc.ValidateAuthorizeClient(ctx, testOAuthClientID, testOAuthRedirectURI)
	require.NoError(t, err)

	_, err = svc.ValidateAuthorizeClient(ctx, testOAuthClientID, "https://app.example.com/oauth/callback/extra")
	require.ErrorIs(t, err, ErrOAuthInvalidRedirectURI)

	_, err = svc.ValidateAuthorizeClient(ctx, "ahc_unknown", testOAuthRedirectURI)
	require.ErrorIs(t, err, ErrOAuthUnknownClient)

	// Disabled clients are rejected.
	_, err = client.OAuthClient.UpdateOneID(1).SetStatus(oauthclient.StatusDisabled).Save(ctx)
	require.NoError(t, err)

	_, err = svc.ValidateAuthorizeClient(ctx, testOAuthClientID, testOAuthRedirectURI)
	require.ErrorIs(t, err, ErrOAuthUnknownClient)
}

func TestOAuthProviderScopeAndRedirectURIValidation(t *testing.T) {
	scopes, err := ParseOAuthScopes("openid profile email")
	require.NoError(t, err)
	require.Equal(t, []string{"openid", "profile", "email"}, scopes)

	scopes, err = ParseOAuthScopes("openid openid profile")
	require.NoError(t, err)
	require.Equal(t, []string{"openid", "profile"}, scopes)

	_, err = ParseOAuthScopes("profile")
	require.Error(t, err, "openid scope is required")

	_, err = ParseOAuthScopes("openid admin")
	require.Error(t, err, "unknown scopes must be rejected")

	_, err = normalizeRedirectURIs([]string{"https://app.example.com/cb"})
	require.NoError(t, err)

	_, err = normalizeRedirectURIs([]string{"http://app.example.com/cb"})
	require.Error(t, err, "http is only allowed for localhost")

	_, err = normalizeRedirectURIs([]string{"http://localhost:5173/cb"})
	require.NoError(t, err)

	_, err = normalizeRedirectURIs([]string{"https://app.example.com/cb#fragment"})
	require.Error(t, err, "fragments are not allowed")

	_, err = normalizeRedirectURIs([]string{"not-a-uri"})
	require.Error(t, err)

	_, err = normalizeRedirectURIs(nil)
	require.Error(t, err)
}

func verifyTestIDToken(t *testing.T, svc *OAuthProviderService, ctx context.Context, idToken string) jwt.MapClaims {
	t.Helper()

	jwks, err := svc.JWKS(ctx)
	require.NoError(t, err)

	keys, ok := jwks["keys"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, keys, 1)

	modulusBytes, err := base64.RawURLEncoding.DecodeString(keys[0]["n"].(string))
	require.NoError(t, err)

	exponentBytes, err := base64.RawURLEncoding.DecodeString(keys[0]["e"].(string))
	require.NoError(t, err)

	publicKey := rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: int(new(big.Int).SetBytes(exponentBytes).Int64()),
	}

	token, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		return &publicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err)
	require.True(t, token.Valid)

	claims, ok := token.Claims.(jwt.MapClaims)
	require.True(t, ok)

	return claims
}

func TestOAuthProviderSigningKeyIsPersisted(t *testing.T) {
	svc, client := setupTestOAuthProviderService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	jwksFirst, err := svc.JWKS(ctx)
	require.NoError(t, err)

	jwksSecond, err := svc.JWKS(ctx)
	require.NoError(t, err)

	first, err := json.Marshal(jwksFirst)
	require.NoError(t, err)
	second, err := json.Marshal(jwksSecond)
	require.NoError(t, err)
	require.JSONEq(t, string(first), string(second), "signing key must be stable across calls")

	stored, err := svc.systemService.getSystemValue(ctx, SystemKeyOAuthSigningKey)
	require.NoError(t, err)
	require.Contains(t, stored, "PRIVATE KEY")
}
