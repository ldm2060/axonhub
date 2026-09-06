package zcode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

// TokenProvider manages the two-layer ZCode credential: the z.ai OAuth
// access/refresh token pair plus the business JWT inference calls use. It
// implements the same surface as oauth.TokenProvider (Get / StartAutoRefresh /
// StopAutoRefresh) so channel_llm can wire it identically.
type TokenProvider struct {
	httpClient  *httpclient.HttpClient
	tokenURL    string
	businessURL string
	sf          singleflight.Group
	mu          sync.RWMutex
	creds       *oauth.OAuthCredentials
	onRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error

	autoMu     sync.Mutex
	autoCancel context.CancelFunc
}

type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	// HTTPClient should be pre-configured with proxy settings if needed
	HTTPClient  *httpclient.HttpClient
	OnRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

type ExchangeParams struct {
	Code        string
	RedirectURI string
	State       string
}

func NewTokenProvider(params TokenProviderParams) *TokenProvider {
	return &TokenProvider{
		httpClient:  params.HTTPClient,
		tokenURL:    TokenURL,
		businessURL: BusinessLoginURL,
		sf:          singleflight.Group{},
		mu:          sync.RWMutex{},
		creds:       params.Credentials,
		onRefreshed: params.OnRefreshed,
		autoMu:      sync.Mutex{},
		autoCancel:  nil,
	}
}

// tokenEnvelope is the z.ai business envelope around the token response:
// {"code":0,"msg":"","data":{"zai":{"access_token":"...","refresh_token":"..."},"expires_in":3600}}.
type tokenEnvelope struct {
	Code *int   `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ExpiresIn int64 `json:"expires_in"`
		ZAI       struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"zai"`
	} `json:"data"`
}

// businessLoginEnvelope is the api.z.ai business-login response:
// {"success":true,"data":{"access_token":"<JWT>"}} — the token key appears
// as both snake_case and camelCase across observed responses.
type businessLoginEnvelope struct {
	Success *bool `json:"success"`
	Data    struct {
		AccessTokenSnake string `json:"access_token"`
		AccessTokenCamel string `json:"accessToken"`
	} `json:"data"`
}

func zcodeHeaders() http.Header {
	header := http.Header{}
	// The token endpoints accept the plain client fingerprint; the versioned
	// User-Agent matters on the inference endpoint (see anthropic outbound).
	header.Set("User-Agent", "ZCode/unknown")
	header.Set("Http-Referer", "https://zcode.z.ai")
	header.Set("X-Title", "Z Code@electron")
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return header
}

func wrapHttpError(err error) error {
	if err == nil {
		return nil
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && len(httpErr.Body) > 0 {
		return fmt.Errorf("%w (response body: %s)", err, string(httpErr.Body))
	}

	return err
}

func decodeJSONResponse[T any](body []byte) (*T, error) {
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// exchangeToken exchanges an authorization code (grant == "") or a refresh
// token (grant == "refresh_token") for the OAuth access token pair.
func exchangeToken(ctx context.Context, client *httpclient.HttpClient, tokenURL string, code, refreshToken, redirectURI, state string) (*tokenEnvelope, error) {
	if client == nil {
		return nil, errors.New("http client is nil")
	}

	reqBody := map[string]string{
		"provider": Provider,
	}
	if refreshToken != "" {
		reqBody["grant_type"] = "refresh_token"
		reqBody["refresh_token"] = refreshToken
	} else {
		reqBody["code"] = code
		reqBody["redirect_uri"] = redirectURI
		reqBody["state"] = state
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal token request: %w", err)
	}

	req := &httpclient.Request{ //nolint:exhaustruct_v5 // Only HTTP plumbing fields matter for token requests.
		Method:  http.MethodPost,
		URL:     tokenURL,
		Headers: zcodeHeaders(),
		Body:    bodyBytes,
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return nil, wrapHttpError(err)
	}

	envelope, err := decodeJSONResponse[tokenEnvelope](resp.Body)
	if err != nil {
		return nil, err
	}

	// The envelope is successful when code is absent, 0, or 200.
	if envelope.Code != nil && *envelope.Code != 0 && *envelope.Code != http.StatusOK {
		return nil, fmt.Errorf("token request failed: code=%d msg=%s", *envelope.Code, envelope.Msg)
	}

	if envelope.Data.ZAI.AccessToken == "" {
		return nil, errors.New("token response missing data.zai.access_token")
	}

	return envelope, nil
}

// exchangeBusinessJWT trades the OAuth access token for the inference JWT.
func exchangeBusinessJWT(ctx context.Context, client *httpclient.HttpClient, businessLoginURL, accessToken string) (string, error) {
	if client == nil {
		return "", errors.New("http client is nil")
	}

	bodyBytes, err := json.Marshal(map[string]string{"token": accessToken})
	if err != nil {
		return "", fmt.Errorf("marshal business login request: %w", err)
	}

	req := &httpclient.Request{ //nolint:exhaustruct_v5 // Only HTTP plumbing fields matter for token requests.
		Method:  http.MethodPost,
		URL:     businessLoginURL,
		Headers: zcodeHeaders(),
		Body:    bodyBytes,
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return "", wrapHttpError(err)
	}

	envelope, err := decodeJSONResponse[businessLoginEnvelope](resp.Body)
	if err != nil {
		return "", err
	}

	if envelope.Success != nil && !*envelope.Success {
		return "", errors.New("business login failed")
	}

	jwt := envelope.Data.AccessTokenSnake
	if jwt == "" {
		jwt = envelope.Data.AccessTokenCamel
	}
	if jwt == "" {
		return "", errors.New("business login response missing access token")
	}

	return jwt, nil
}

// buildCreds assembles the persisted credential set from a token envelope
// plus a freshly exchanged business JWT.
func buildCreds(envelope *tokenEnvelope, jwt string, previous *oauth.OAuthCredentials) *oauth.OAuthCredentials {
	creds := &oauth.OAuthCredentials{
		ClientID:     ClientID,
		AccessToken:  envelope.Data.ZAI.AccessToken,
		RefreshToken: envelope.Data.ZAI.RefreshToken,
		IDToken:      "",
		ExpiresAt:    time.Now().Add(expiresIn(envelope)),
		TokenType:    "",
		Scopes:       nil,
		KimiCode:     nil,
		ZCode:        &oauth.ZCodeMetadata{BusinessJWT: jwt},
	}

	// Refresh responses do not always return a new refresh token.
	if creds.RefreshToken == "" && previous != nil {
		creds.RefreshToken = previous.RefreshToken
	}

	return creds
}

func expiresIn(envelope *tokenEnvelope) time.Duration {
	if envelope.Data.ExpiresIn > 0 {
		return time.Duration(envelope.Data.ExpiresIn) * time.Second
	}
	return 1 * time.Hour
}

// Exchange performs the full login flow: authorization code → OAuth access
// token → business JWT. The credentials are cached in the provider.
func (p *TokenProvider) Exchange(ctx context.Context, params ExchangeParams) (*oauth.OAuthCredentials, error) {
	if params.Code == "" {
		return nil, errors.New("code is empty")
	}

	redirectURI := params.RedirectURI
	if redirectURI == "" {
		redirectURI = RedirectURI
	}

	envelope, err := exchangeToken(ctx, p.httpClient, p.tokenURL, params.Code, "", redirectURI, params.State)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	jwt, err := exchangeBusinessJWT(ctx, p.httpClient, p.businessURL, envelope.Data.ZAI.AccessToken)
	if err != nil {
		return nil, err
	}

	creds := buildCreds(envelope, jwt, nil)

	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// jwtExp decodes the business JWT's exp claim without verifying the
// signature. This is only used as a client-side refresh heuristic for a token
// z.ai handed us; the actual authorization decision happens server-side.
func jwtExp(jwt string) (time.Time, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}

	return time.Unix(claims.Exp, 0), true
}

// effectiveExpiry returns when the credential set needs a refresh: whichever
// of the OAuth access token expiry and the business JWT exp comes first.
func effectiveExpiry(creds *oauth.OAuthCredentials) time.Time {
	deadline := creds.ExpiresAt

	if creds.ZCode != nil && creds.ZCode.BusinessJWT != "" {
		if exp, ok := jwtExp(creds.ZCode.BusinessJWT); ok && (deadline.IsZero() || exp.Before(deadline)) {
			deadline = exp
		}
	}

	return deadline
}

func needsRefresh(creds *oauth.OAuthCredentials, refreshBefore time.Duration) bool {
	if creds == nil || creds.RefreshToken == "" {
		return false
	}

	deadline := effectiveExpiry(creds)
	if deadline.IsZero() {
		return true
	}

	return time.Now().Add(refreshBefore).After(deadline)
}

// Get returns credentials whose AccessToken is the business JWT used for
// inference, refreshing the OAuth token (and re-exchanging the JWT) when
// expired.
func (p *TokenProvider) Get(ctx context.Context) (*oauth.OAuthCredentials, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return nil, errors.New("credentials is nil")
	}

	if !needsRefresh(creds, 3*time.Minute) {
		return withBusinessJWT(creds), nil
	}

	v, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		onRefreshed := p.onRefreshed
		p.mu.RUnlock()

		if current == nil {
			return nil, errors.New("credentials is nil")
		}

		if !needsRefresh(current, 3*time.Minute) {
			return current, nil
		}

		fresh, err := p.refresh(ctx, current)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.creds = fresh
		p.mu.Unlock()

		if onRefreshed != nil {
			if err := onRefreshed(ctx, fresh); err != nil {
				slog.WarnContext(ctx, "failed to persist refreshed zcode credentials", slog.Any("error", err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*oauth.OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return withBusinessJWT(fresh), nil
}

// withBusinessJWT copies creds and replaces AccessToken with the business JWT
// so callers cannot accidentally use the intermediate OAuth token for
// inference.
func withBusinessJWT(creds *oauth.OAuthCredentials) *oauth.OAuthCredentials {
	out := *creds
	if creds.ZCode != nil && creds.ZCode.BusinessJWT != "" {
		out.AccessToken = creds.ZCode.BusinessJWT
	}
	return &out
}

func (p *TokenProvider) refresh(ctx context.Context, creds *oauth.OAuthCredentials) (*oauth.OAuthCredentials, error) {
	if creds.RefreshToken == "" {
		return nil, errors.New("refresh_token is empty")
	}

	envelope, err := exchangeToken(ctx, p.httpClient, p.tokenURL, "", creds.RefreshToken, "", "")
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	jwt, err := exchangeBusinessJWT(ctx, p.httpClient, p.businessURL, envelope.Data.ZAI.AccessToken)
	if err != nil {
		return nil, err
	}

	return buildCreds(envelope, jwt, creds), nil
}

// EnsureFresh refreshes the credentials early when they expire within
// refreshBefore, mirroring oauth.TokenProvider.EnsureFresh.
func (p *TokenProvider) EnsureFresh(ctx context.Context, refreshBefore time.Duration) (*oauth.OAuthCredentials, error) {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil {
		return nil, errors.New("credentials is nil")
	}

	if refreshBefore <= 0 {
		refreshBefore = 5 * time.Minute
	}

	if !needsRefresh(creds, refreshBefore) {
		return creds, nil
	}

	v, err, _ := p.sf.Do("refresh", func() (any, error) {
		p.mu.RLock()
		current := p.creds
		onRefreshed := p.onRefreshed
		p.mu.RUnlock()

		if current == nil {
			return nil, errors.New("credentials is nil")
		}

		if !needsRefresh(current, refreshBefore) {
			return current, nil
		}

		fresh, err := p.refresh(ctx, current)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.creds = fresh
		p.mu.Unlock()

		if onRefreshed != nil {
			if err := onRefreshed(ctx, fresh); err != nil {
				slog.WarnContext(ctx, "failed to persist refreshed zcode credentials", slog.Any("error", err))
			}
		}

		return fresh, nil
	})
	if err != nil {
		return nil, err
	}

	fresh, ok := v.(*oauth.OAuthCredentials)
	if !ok {
		return nil, fmt.Errorf("singleflight returned unexpected type %T", v)
	}

	return fresh, nil
}

// StartAutoRefresh keeps the credentials fresh in the background so requests
// do not pay the refresh latency. Implements the AutoRefresher interface used
// by channel_llm.setupAutoRefresh.
func (p *TokenProvider) StartAutoRefresh(ctx context.Context, opts oauth.AutoRefreshOptions) {
	fallbackInterval := opts.Interval
	if fallbackInterval <= 0 {
		fallbackInterval = 1 * time.Minute
	}

	refreshBefore := opts.RefreshBefore
	if refreshBefore <= 0 {
		refreshBefore = 5 * time.Minute
	}

	p.autoMu.Lock()

	if p.autoCancel != nil {
		p.autoMu.Unlock()
		return
	}

	autoCtx, cancel := context.WithCancel(ctx)
	p.autoCancel = cancel
	p.autoMu.Unlock()

	go p.runAutoRefresh(autoCtx, refreshBefore, fallbackInterval)
}

func (p *TokenProvider) StopAutoRefresh() {
	p.autoMu.Lock()
	cancel := p.autoCancel
	p.autoCancel = nil
	p.autoMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (p *TokenProvider) runAutoRefresh(autoCtx context.Context, refreshBefore, fallbackInterval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(autoCtx, "zcode auto refresh goroutine panicked", slog.Any("cause", r))
		}
	}()

	for {
		if !sleepUntilRefresh(autoCtx, p.nextAutoRefreshDelay(refreshBefore, fallbackInterval)) {
			return
		}

		if _, err := p.EnsureFresh(autoCtx, refreshBefore); err != nil {
			slog.WarnContext(autoCtx, "failed to auto refresh zcode token", slog.Any("error", err))
		}

		if autoCtx.Err() != nil {
			return
		}
	}
}

func (p *TokenProvider) nextAutoRefreshDelay(refreshBefore, fallbackInterval time.Duration) time.Duration {
	p.mu.RLock()
	creds := p.creds
	p.mu.RUnlock()

	if creds == nil || creds.RefreshToken == "" {
		return fallbackInterval
	}

	deadline := effectiveExpiry(creds)
	if deadline.IsZero() {
		return fallbackInterval
	}

	delay := time.Until(deadline.Add(-refreshBefore))
	if delay < 0 {
		return 0
	}

	return delay
}

func sleepUntilRefresh(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
