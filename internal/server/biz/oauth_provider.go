package biz

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/oauthclient"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

const (
	SystemKeyOAuthSigningKey = "system_oauth_signing_key"

	oauthClientIDPrefix     = "ahc_"
	oauthClientSecretPrefix = "ahs_"
	oauthAccessTokenPrefix  = "aho_"

	oauthAuthRequestTTL = 10 * time.Minute
	oauthCodeTTL        = 5 * time.Minute
	oauthAccessTokenTTL = time.Hour

	// OAuthErrorInvalidRequest and the codes below are the standard OAuth2
	// authorization request and token error codes (RFC 6749 §5.2).
	OAuthErrorInvalidRequest          = "invalid_request"
	OAuthErrorInvalidClient           = "invalid_client"
	OAuthErrorInvalidGrant            = "invalid_grant"
	OAuthErrorInvalidScope            = "invalid_scope"
	OAuthErrorUnsupportedResponseType = "unsupported_response_type"
	OAuthErrorAccessDenied            = "access_denied"
	OAuthErrorServerError             = "server_error"
)

// SupportedOAuthScopes lists the OIDC scopes this provider issues.
var SupportedOAuthScopes = []string{"openid", "profile", "email"}

// ErrOAuthUnknownClient is returned when the client_id is unknown or disabled.
// The authorize endpoint must render an error page instead of redirecting.
var ErrOAuthUnknownClient = errors.New("unknown or disabled oauth client")

// ErrOAuthInvalidRedirectURI is returned when redirect_uri is not registered.
// The authorize endpoint must render an error page instead of redirecting.
var ErrOAuthInvalidRedirectURI = errors.New("redirect_uri is not registered for this client")

// OAuthError is a protocol-level error carrying a standard OAuth2 error code.
type OAuthError struct {
	Code        string
	Description string
}

func (e *OAuthError) Error() string {
	return e.Code + ": " + e.Description
}

func newOAuthError(code, description string) *OAuthError {
	return &OAuthError{Code: code, Description: description}
}

type OAuthProviderService struct {
	*AbstractService

	cache         xcache.Cache[[]byte]
	systemService *SystemService
	userService   *UserService
	publicURL     string

	signingKeyMu sync.Mutex
}

type OAuthProviderServiceParams struct {
	fx.In

	Ent           *ent.Client
	CacheConfig   xcache.Config
	SystemService *SystemService
	UserService   *UserService
	PublicURL     string `name:"public_url"`
}

func NewOAuthProviderService(params OAuthProviderServiceParams) *OAuthProviderService {
	return &OAuthProviderService{
		AbstractService: &AbstractService{db: params.Ent},
		cache:           xcache.NewFromConfig[[]byte](params.CacheConfig),
		systemService:   params.SystemService,
		userService:     params.UserService,
		publicURL:       strings.TrimSuffix(strings.TrimSpace(params.PublicURL), "/"),
	}
}

func (s *OAuthProviderService) PublicURL() string {
	return s.publicURL
}

// ---------------------------------------------------------------------------
// Client management (admin)
// ---------------------------------------------------------------------------

type CreateOAuthClientInput struct {
	Name         string
	Description  string
	RedirectURIs []string
	ClientType   string
}

type UpdateOAuthClientInput struct {
	Name         *string
	Description  *string
	RedirectURIs []string
	Status       *string
}

func (s *OAuthProviderService) CreateClient(ctx context.Context, in CreateOAuthClientInput) (*ent.OAuthClient, string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, "", errors.New("client name is required")
	}

	redirectURIs, err := normalizeRedirectURIs(in.RedirectURIs)
	if err != nil {
		return nil, "", err
	}

	clientType := oauthclient.ClientType(strings.TrimSpace(in.ClientType))
	if clientType == "" {
		clientType = oauthclient.DefaultClientType
	}
	if err := oauthclient.ClientTypeValidator(clientType); err != nil {
		return nil, "", fmt.Errorf("invalid client type: %s", in.ClientType)
	}

	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, "", errors.New("authentication required")
	}

	clientID, err := generateOAuthToken(oauthClientIDPrefix, 16)
	if err != nil {
		return nil, "", err
	}

	clientSecret, err := generateOAuthToken(oauthClientSecretPrefix, 32)
	if err != nil {
		return nil, "", err
	}

	client, err := s.entFromContext(ctx).OAuthClient.Create().
		SetName(name).
		SetDescription(strings.TrimSpace(in.Description)).
		SetClientID(clientID).
		SetClientSecretHash(hashOAuthSecret(clientSecret)).
		SetRedirectUris(redirectURIs).
		SetClientType(clientType).
		SetUserID(user.ID).
		Save(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create oauth client: %w", err)
	}

	return client, clientSecret, nil
}

func (s *OAuthProviderService) UpdateClient(ctx context.Context, id int, in UpdateOAuthClientInput) (*ent.OAuthClient, error) {
	update := s.entFromContext(ctx).OAuthClient.UpdateOneID(id)

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errors.New("client name is required")
		}

		update.SetName(name)
	}

	if in.Description != nil {
		update.SetDescription(strings.TrimSpace(*in.Description))
	}

	if in.RedirectURIs != nil {
		redirectURIs, err := normalizeRedirectURIs(in.RedirectURIs)
		if err != nil {
			return nil, err
		}

		update.SetRedirectUris(redirectURIs)
	}

	if in.Status != nil {
		status := oauthclient.Status(strings.TrimSpace(*in.Status))
		if err := oauthclient.StatusValidator(status); err != nil {
			return nil, fmt.Errorf("invalid status: %s", *in.Status)
		}

		update.SetStatus(status)
	}

	client, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("oauth client not found")
		}

		return nil, fmt.Errorf("failed to update oauth client: %w", err)
	}

	return client, nil
}

func (s *OAuthProviderService) DeleteClient(ctx context.Context, id int) error {
	err := s.entFromContext(ctx).OAuthClient.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("oauth client not found")
		}

		return fmt.Errorf("failed to delete oauth client: %w", err)
	}

	return nil
}

func (s *OAuthProviderService) RotateClientSecret(ctx context.Context, id int) (*ent.OAuthClient, string, error) {
	clientSecret, err := generateOAuthToken(oauthClientSecretPrefix, 32)
	if err != nil {
		return nil, "", err
	}

	client, err := s.entFromContext(ctx).OAuthClient.UpdateOneID(id).
		SetClientSecretHash(hashOAuthSecret(clientSecret)).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, "", fmt.Errorf("oauth client not found")
		}

		return nil, "", fmt.Errorf("failed to rotate oauth client secret: %w", err)
	}

	return client, clientSecret, nil
}

// LookupClient loads an enabled client by client_id using a system bypass, for
// use by the public OAuth endpoints where no user principal exists.
func (s *OAuthProviderService) LookupClient(ctx context.Context, clientID string) (*ent.OAuthClient, error) {
	if strings.TrimSpace(clientID) == "" {
		return nil, ErrOAuthUnknownClient
	}

	client, err := authz.RunWithSystemBypass(ctx, "oauth-provider-client-lookup", func(bypassCtx context.Context) (*ent.OAuthClient, error) {
		return s.entFromContext(bypassCtx).OAuthClient.Query().
			Where(oauthclient.ClientIDEQ(clientID)).
			Only(bypassCtx)
	})
	if err != nil {
		return nil, ErrOAuthUnknownClient
	}

	if client.Status != oauthclient.StatusEnabled {
		return nil, ErrOAuthUnknownClient
	}

	return client, nil
}

// ---------------------------------------------------------------------------
// Authorization endpoint support
// ---------------------------------------------------------------------------

type AuthorizationRequestParams struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type AuthorizationRequestInfo struct {
	RequestID         string
	ClientID          string
	ClientName        string
	ClientDescription string
	RedirectURI       string
	RedirectURIHost   string
	Scopes            []string
}

type storedAuthorizationRequest struct {
	ClientID            string   `json:"client_id"`
	RedirectURI         string   `json:"redirect_uri"`
	Scopes              []string `json:"scopes"`
	State               string   `json:"state"`
	Nonce               string   `json:"nonce"`
	CodeChallenge       string   `json:"code_challenge"`
	CodeChallengeMethod string   `json:"code_challenge_method"`
}

type storedAuthorizationCode struct {
	ClientID            string   `json:"client_id"`
	UserID              int      `json:"user_id"`
	RedirectURI         string   `json:"redirect_uri"`
	Scopes              []string `json:"scopes"`
	Nonce               string   `json:"nonce"`
	CodeChallenge       string   `json:"code_challenge"`
	CodeChallengeMethod string   `json:"code_challenge_method"`
	AuthTime            int64    `json:"auth_time"`
}

type storedAccessToken struct {
	UserID    int      `json:"user_id"`
	ClientID  string   `json:"client_id"`
	Scopes    []string `json:"scopes"`
	ExpiresAt int64    `json:"expires_at"`
}

// ValidateAuthorizeClient checks client_id and redirect_uri. Errors returned
// here must be shown to the user directly; the request must not be redirected.
func (s *OAuthProviderService) ValidateAuthorizeClient(ctx context.Context, clientID, redirectURI string) (*ent.OAuthClient, error) {
	client, err := s.LookupClient(ctx, clientID)
	if err != nil {
		return nil, err
	}

	if !IsRegisteredRedirectURI(client, redirectURI) {
		return nil, ErrOAuthInvalidRedirectURI
	}

	return client, nil
}

// CreateAuthorizationRequest validates the remaining OAuth parameters and
// stores the request for the consent page. The returned id is the request_id
// handed to the frontend consent route.
func (s *OAuthProviderService) CreateAuthorizationRequest(
	ctx context.Context,
	client *ent.OAuthClient,
	params AuthorizationRequestParams,
) (string, error) {
	if params.ResponseType != "code" {
		return "", newOAuthError(OAuthErrorUnsupportedResponseType, "only response_type=code is supported")
	}

	scopes, err := ParseOAuthScopes(params.Scope)
	if err != nil {
		return "", err
	}

	if params.CodeChallengeMethod != "S256" || params.CodeChallenge == "" {
		return "", newOAuthError(OAuthErrorInvalidRequest, "PKCE with code_challenge_method=S256 is required")
	}

	requestID, err := generateOAuthToken("", 24)
	if err != nil {
		return "", err
	}

	payload := storedAuthorizationRequest{
		ClientID:            client.ClientID,
		RedirectURI:         params.RedirectURI,
		Scopes:              scopes,
		State:               params.State,
		Nonce:               params.Nonce,
		CodeChallenge:       params.CodeChallenge,
		CodeChallengeMethod: params.CodeChallengeMethod,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode authorization request: %w", err)
	}

	if err := s.cache.Set(ctx, oauthAuthRequestCacheKey(requestID), raw, xcache.WithExpiration(oauthAuthRequestTTL)); err != nil {
		return "", newOAuthError(OAuthErrorServerError, "failed to store authorization request")
	}

	return requestID, nil
}

func (s *OAuthProviderService) GetAuthorizationRequest(ctx context.Context, requestID string) (*AuthorizationRequestInfo, error) {
	payload, err := s.loadAuthorizationRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	client, err := s.LookupClient(ctx, payload.ClientID)
	if err != nil {
		return nil, err
	}

	host := payload.RedirectURI
	if parsed, err := url.Parse(payload.RedirectURI); err == nil && parsed.Host != "" {
		host = parsed.Host
	}

	return &AuthorizationRequestInfo{
		RequestID:         requestID,
		ClientID:          client.ClientID,
		ClientName:        client.Name,
		ClientDescription: client.Description,
		RedirectURI:       payload.RedirectURI,
		RedirectURIHost:   host,
		Scopes:            payload.Scopes,
	}, nil
}

// ApproveAuthorization issues a one-time authorization code and returns the
// redirect URL back to the third-party client.
func (s *OAuthProviderService) ApproveAuthorization(ctx context.Context, requestID string, u *ent.User) (string, error) {
	payload, err := s.loadAuthorizationRequest(ctx, requestID)
	if err != nil {
		return "", err
	}

	if err := s.cache.Delete(ctx, oauthAuthRequestCacheKey(requestID)); err != nil {
		log.Warn(ctx, "failed to delete oauth authorization request", log.String("request_id", requestID))
	}

	code, err := generateOAuthToken("", 32)
	if err != nil {
		return "", err
	}

	codePayload := storedAuthorizationCode{
		ClientID:            payload.ClientID,
		UserID:              u.ID,
		RedirectURI:         payload.RedirectURI,
		Scopes:              payload.Scopes,
		Nonce:               payload.Nonce,
		CodeChallenge:       payload.CodeChallenge,
		CodeChallengeMethod: payload.CodeChallengeMethod,
		AuthTime:            time.Now().Unix(),
	}

	raw, err := json.Marshal(codePayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode authorization code: %w", err)
	}

	if err := s.cache.Set(ctx, oauthCodeCacheKey(code), raw, xcache.WithExpiration(oauthCodeTTL)); err != nil {
		return "", newOAuthError(OAuthErrorServerError, "failed to store authorization code")
	}

	return buildRedirectURL(payload.RedirectURI, url.Values{
		"code":  {code},
		"state": {payload.State},
	}), nil
}

func (s *OAuthProviderService) DenyAuthorization(ctx context.Context, requestID string) (string, error) {
	payload, err := s.loadAuthorizationRequest(ctx, requestID)
	if err != nil {
		return "", err
	}

	if err := s.cache.Delete(ctx, oauthAuthRequestCacheKey(requestID)); err != nil {
		log.Warn(ctx, "failed to delete oauth authorization request", log.String("request_id", requestID))
	}

	return buildRedirectURL(payload.RedirectURI, url.Values{
		"error":             {OAuthErrorAccessDenied},
		"error_description": {"The user denied the authorization request"},
		"state":             {payload.State},
	}), nil
}

func (s *OAuthProviderService) loadAuthorizationRequest(ctx context.Context, requestID string) (*storedAuthorizationRequest, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, newOAuthError(OAuthErrorInvalidRequest, "authorization request is missing")
	}

	raw, err := s.cache.Get(ctx, oauthAuthRequestCacheKey(requestID))
	if err != nil || len(raw) == 0 {
		return nil, newOAuthError(OAuthErrorInvalidRequest, "authorization request is invalid or expired")
	}

	var payload storedAuthorizationRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, newOAuthError(OAuthErrorInvalidRequest, "authorization request is invalid")
	}

	return &payload, nil
}

// ---------------------------------------------------------------------------
// Token endpoint
// ---------------------------------------------------------------------------

type TokenExchangeParams struct {
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type TokenResponse struct {
	AccessToken string
	IDToken     string
	TokenType   string
	ExpiresIn   int
	Scope       string
}

func (s *OAuthProviderService) ExchangeAuthorizationCode(ctx context.Context, params TokenExchangeParams) (*TokenResponse, error) {
	if strings.TrimSpace(params.Code) == "" {
		return nil, newOAuthError(OAuthErrorInvalidRequest, "code is required")
	}

	raw, err := s.cache.Get(ctx, oauthCodeCacheKey(params.Code))
	if err != nil || len(raw) == 0 {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "authorization code is invalid or expired")
	}

	// Authorization codes are single-use; consume before validation so a
	// concurrent replay cannot succeed.
	if err := s.cache.Delete(ctx, oauthCodeCacheKey(params.Code)); err != nil {
		log.Warn(ctx, "failed to delete oauth authorization code")
	}

	var payload storedAuthorizationCode
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "authorization code is invalid")
	}

	if payload.ClientID != params.ClientID {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "authorization code was not issued to this client")
	}

	client, err := s.LookupClient(ctx, payload.ClientID)
	if err != nil {
		return nil, newOAuthError(OAuthErrorInvalidClient, "client is unknown or disabled")
	}

	if client.ClientType == oauthclient.ClientTypeConfidential {
		if subtle.ConstantTimeCompare([]byte(hashOAuthSecret(params.ClientSecret)), []byte(client.ClientSecretHash)) != 1 {
			return nil, newOAuthError(OAuthErrorInvalidClient, "client authentication failed")
		}
	}

	if params.RedirectURI != payload.RedirectURI {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "redirect_uri does not match the authorization request")
	}

	if !verifyPKCE(params.CodeVerifier, payload.CodeChallenge, payload.CodeChallengeMethod) {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "PKCE verification failed")
	}

	u, err := authz.RunWithSystemBypass(ctx, "oauth-provider-user-lookup", func(bypassCtx context.Context) (*ent.User, error) {
		return s.userService.GetUserByID(bypassCtx, payload.UserID)
	})
	if err != nil {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "user is not available")
	}

	if u.Status != user.StatusActivated {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "user account is not activated")
	}

	accessToken, err := generateOAuthToken(oauthAccessTokenPrefix, 32)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(oauthAccessTokenTTL)

	accessRaw, err := json.Marshal(storedAccessToken{
		UserID:    u.ID,
		ClientID:  client.ClientID,
		Scopes:    payload.Scopes,
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode access token: %w", err)
	}

	if err := s.cache.Set(ctx, oauthAccessTokenCacheKey(accessToken), accessRaw, xcache.WithExpiration(oauthAccessTokenTTL)); err != nil {
		return nil, newOAuthError(OAuthErrorServerError, "failed to store access token")
	}

	idToken, err := s.signIDToken(ctx, client.ClientID, u, payload.Scopes, payload.Nonce, payload.AuthTime)
	if err != nil {
		return nil, err
	}

	s.touchClientLastUsed(ctx, client.ID)

	return &TokenResponse{
		AccessToken: accessToken,
		IDToken:     idToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(oauthAccessTokenTTL.Seconds()),
		Scope:       strings.Join(payload.Scopes, " "),
	}, nil
}

func (s *OAuthProviderService) touchClientLastUsed(ctx context.Context, clientID int) {
	_, err := authz.RunWithSystemBypass(ctx, "oauth-provider-client-touch", func(bypassCtx context.Context) (*ent.OAuthClient, error) {
		return s.entFromContext(bypassCtx).OAuthClient.UpdateOneID(clientID).
			SetLastUsedAt(time.Now()).
			Save(bypassCtx)
	})
	if err != nil {
		log.Warn(ctx, "failed to update oauth client last used at", log.Cause(err))
	}
}

// ---------------------------------------------------------------------------
// UserInfo endpoint
// ---------------------------------------------------------------------------

func (s *OAuthProviderService) UserInfo(ctx context.Context, accessToken string) (map[string]any, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, newOAuthError(OAuthErrorInvalidRequest, "access token is required")
	}

	raw, err := s.cache.Get(ctx, oauthAccessTokenCacheKey(accessToken))
	if err != nil || len(raw) == 0 {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "access token is invalid or expired")
	}

	var payload storedAccessToken
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "access token is invalid")
	}

	if time.Now().Unix() >= payload.ExpiresAt {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "access token is expired")
	}

	u, err := authz.RunWithSystemBypass(ctx, "oauth-provider-userinfo-lookup", func(bypassCtx context.Context) (*ent.User, error) {
		return s.userService.GetUserByID(bypassCtx, payload.UserID)
	})
	if err != nil {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "user is not available")
	}

	if u.Status != user.StatusActivated {
		return nil, newOAuthError(OAuthErrorInvalidGrant, "user account is not activated")
	}

	return s.buildClaims(u, payload.Scopes), nil
}

// ---------------------------------------------------------------------------
// ID token signing & JWKS
// ---------------------------------------------------------------------------

func (s *OAuthProviderService) signIDToken(
	ctx context.Context,
	clientID string,
	u *ent.User,
	scopes []string,
	nonce string,
	authTime int64,
) (string, error) {
	privateKey, kid, err := s.signingKey(ctx)
	if err != nil {
		return "", err
	}

	now := time.Now()

	claims := jwt.MapClaims{
		"iss":       s.publicURL,
		"sub":       strconv.Itoa(u.ID),
		"aud":       clientID,
		"iat":       now.Unix(),
		"exp":       now.Add(oauthAccessTokenTTL).Unix(),
		"auth_time": authTime,
	}

	for k, v := range s.buildClaims(u, scopes) {
		if k == "sub" {
			continue
		}

		claims[k] = v
	}

	if nonce != "" {
		claims["nonce"] = nonce
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign id token: %w", err)
	}

	return signed, nil
}

func (s *OAuthProviderService) JWKS(ctx context.Context) (map[string]any, error) {
	privateKey, kid, err := s.signingKey(ctx)
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.PublicKey

	return map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
			},
		},
	}, nil
}

func (s *OAuthProviderService) Metadata(ctx context.Context) map[string]any {
	return map[string]any{
		"issuer":                                s.publicURL,
		"authorization_endpoint":                s.publicURL + "/oauth2/authorize",
		"token_endpoint":                        s.publicURL + "/oauth2/token",
		"userinfo_endpoint":                     s.publicURL + "/oauth2/userinfo",
		"jwks_uri":                              s.publicURL + "/oauth2/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      SupportedOAuthScopes,
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
		"code_challenge_methods_supported":      []string{"S256"},
		"claims_supported": []string{
			"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce",
			"email", "email_verified", "name", "given_name", "family_name",
			"preferred_username", "picture",
		},
	}
}

// signingKey returns the RSA signing key used for id_tokens, generating and
// persisting one on first use.
func (s *OAuthProviderService) signingKey(ctx context.Context) (*rsa.PrivateKey, string, error) {
	s.signingKeyMu.Lock()
	defer s.signingKeyMu.Unlock()

	var (
		pemValue string
		err      error
	)

	pemValue, err = authz.RunWithSystemBypass(ctx, "oauth-provider-signing-key", func(bypassCtx context.Context) (string, error) {
		return s.systemService.getSystemValue(bypassCtx, SystemKeyOAuthSigningKey)
	})

	switch {
	case err == nil:
	case ent.IsNotFound(err):
		pemValue, err = s.generateSigningKey(ctx)
		if err != nil {
			return nil, "", err
		}
	default:
		return nil, "", fmt.Errorf("failed to load oauth signing key: %w", err)
	}

	privateKey, err := parseSigningKey(pemValue)
	if err != nil {
		return nil, "", err
	}

	return privateKey, signingKeyID(&privateKey.PublicKey), nil
}

func (s *OAuthProviderService) generateSigningKey(ctx context.Context) (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", fmt.Errorf("failed to generate oauth signing key: %w", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal oauth signing key: %w", err)
	}

	pemValue := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	_, err = authz.RunWithSystemBypass(ctx, "oauth-provider-signing-key", func(bypassCtx context.Context) (struct{}, error) {
		return struct{}{}, s.systemService.setSystemValue(bypassCtx, SystemKeyOAuthSigningKey, pemValue)
	})
	if err != nil {
		return "", fmt.Errorf("failed to persist oauth signing key: %w", err)
	}

	return pemValue, nil
}

func parseSigningKey(pemValue string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, errors.New("invalid oauth signing key: not PEM encoded")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid oauth signing key: %w", err)
	}

	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("invalid oauth signing key: not an RSA private key")
	}

	return privateKey, nil
}

func signingKeyID(publicKey *rsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "default"
	}

	sum := sha256.Sum256(der)

	return hex.EncodeToString(sum[:8])
}

func (s *OAuthProviderService) buildClaims(u *ent.User, scopes []string) map[string]any {
	claims := map[string]any{
		"sub": strconv.Itoa(u.ID),
	}

	if slices.Contains(scopes, "email") {
		claims["email"] = u.Email
		claims["email_verified"] = u.EmailVerifiedAt != nil
	}

	if slices.Contains(scopes, "profile") {
		name := strings.TrimSpace(u.FirstName + " " + u.LastName)
		if name != "" {
			claims["name"] = name
		}

		if u.FirstName != "" {
			claims["given_name"] = u.FirstName
		}

		if u.LastName != "" {
			claims["family_name"] = u.LastName
		}

		if local, _, ok := strings.Cut(u.Email, "@"); ok && local != "" {
			claims["preferred_username"] = local
		}

		claims["picture"] = s.publicURL + "/avatars/" + strconv.Itoa(u.ID)
	}

	return claims
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// IsRegisteredRedirectURI reports whether redirectURI exactly matches one of
// the client's registered redirect URIs (RFC 6749 §3.1.2.3).
func IsRegisteredRedirectURI(client *ent.OAuthClient, redirectURI string) bool {
	if redirectURI == "" {
		return false
	}

	return slices.Contains(client.RedirectUris, redirectURI)
}

// ParseOAuthScopes splits and validates a space-delimited scope string.
func ParseOAuthScopes(scope string) ([]string, error) {
	raw := strings.Fields(scope)
	if len(raw) == 0 {
		return nil, newOAuthError(OAuthErrorInvalidScope, "scope is required and must include openid")
	}

	scopes := make([]string, 0, len(raw))
	for _, s := range raw {
		if !slices.Contains(SupportedOAuthScopes, s) {
			return nil, newOAuthError(OAuthErrorInvalidScope, "unsupported scope: "+s)
		}

		if !slices.Contains(scopes, s) {
			scopes = append(scopes, s)
		}
	}

	if !slices.Contains(scopes, "openid") {
		return nil, newOAuthError(OAuthErrorInvalidScope, "openid scope is required")
	}

	return scopes, nil
}

func normalizeRedirectURIs(uris []string) ([]string, error) {
	if len(uris) == 0 {
		return nil, errors.New("at least one redirect URI is required")
	}

	normalized := make([]string, 0, len(uris))
	for _, raw := range uris {
		uri := strings.TrimSpace(raw)
		if uri == "" {
			continue
		}

		if err := validateRedirectURI(uri); err != nil {
			return nil, err
		}

		if !slices.Contains(normalized, uri) {
			normalized = append(normalized, uri)
		}
	}

	if len(normalized) == 0 {
		return nil, errors.New("at least one redirect URI is required")
	}

	return normalized, nil
}

func validateRedirectURI(uri string) error {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("invalid redirect URI: %s", uri)
	}

	if parsed.Fragment != "" {
		return fmt.Errorf("redirect URI must not contain a fragment: %s", uri)
	}

	switch parsed.Scheme {
	case "https":
		if parsed.Host == "" {
			return fmt.Errorf("redirect URI must include a host: %s", uri)
		}
	case "http":
		switch parsed.Hostname() {
		case "localhost", "127.0.0.1", "::1":
		default:
			return fmt.Errorf("redirect URI must use https (http is only allowed for localhost): %s", uri)
		}
	default:
		// Native applications may use a custom scheme, e.g. myapp://callback.
		if parsed.Host == "" {
			return fmt.Errorf("redirect URI must include a host: %s", uri)
		}
	}

	return nil
}

func verifyPKCE(verifier, challenge, method string) bool {
	if method != "S256" || verifier == "" || challenge == "" {
		return false
	}

	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])

	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}

func hashOAuthSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))

	return hex.EncodeToString(sum[:])
}

func generateOAuthToken(prefix string, numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	return prefix + hex.EncodeToString(buf), nil
}

// BuildOAuthErrorRedirect builds the error redirect back to the client, as
// required for authorization endpoint parameter errors (RFC 6749 §4.1.2.1).
func BuildOAuthErrorRedirect(redirectURI, state string, oauthErr *OAuthError) string {
	return buildRedirectURL(redirectURI, url.Values{
		"error":             {oauthErr.Code},
		"error_description": {oauthErr.Description},
		"state":             {state},
	})
}

func buildRedirectURL(rawURL string, params url.Values) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	for key, values := range params {
		if len(values) == 0 || values[0] == "" {
			continue
		}

		query.Set(key, values[0])
	}

	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func oauthAuthRequestCacheKey(requestID string) string {
	return "oauth_provider:auth_request:" + requestID
}

func oauthCodeCacheKey(code string) string {
	return "oauth_provider:code:" + code
}

func oauthAccessTokenCacheKey(token string) string {
	return "oauth_provider:access_token:" + token
}
