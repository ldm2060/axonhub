package zcode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	// Provider selects the ZCode login provider: Provider (z.ai, default when
	// empty) or BigModelProvider (bigmodel.cn coding plan).
	Provider string
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

// TokenEnvelopeData is the payload the z.ai business envelope carries around
// the token response. Both providers return the zcode plan JWT as data.token;
// the provider OAuth token nests under data.zai / data.bigmodel. Z.AI
// additionally uses a separate business-login call for the JWT; BigModel
// returns it inline.
// {"code":0,"msg":"","data":{"token":"<jwt>","zai":{"access_token","refresh_token"},"bigmodel":{...},"user":{"user_id"},"expires_in":3600}}.
type TokenEnvelopeData struct {
	// Token is the zcode plan JWT (the inference credential).
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	ZAI       struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"zai"`
	BigModel struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"bigmodel"`
}

type TokenEnvelope struct {
	Code *int              `json:"code"`
	Msg  string            `json:"msg"`
	Data TokenEnvelopeData `json:"data"`
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

// osVersion returns a best-effort OS version string the way os.version() does
// in the Electron client (on Windows: the build number, e.g. "10.0.26200").
func osVersion() string {
	switch runtime.GOOS {
	case "windows":
		if v := osVersionWindows(); v != "unknown" {
			return v
		}
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	default:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if utsname, err := exec.CommandContext(ctx, "uname", "-r").Output(); err == nil {
			return strings.TrimSpace(string(utsname))
		}
	}
	return "unknown"
}

// osCategory maps GOOS to the client's X-Os-Category values
// (darwin→macos, win32→windows, else linux).
func osCategory() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// zcodeHeaders rebuilds the source-header fingerprint the desktop client's
// NodeApiClient stamps on every request to the zcode endpoint origin
// (buildZCodeSourceHeadersFromContext in the 3.11.2 asar) plus the per-request
// x-request-id UUID (withRequestIdHeader).
func zcodeHeaders() http.Header {
	header := http.Header{}
	header.Set("User-Agent", "ZCode/"+AppVersion)
	header.Set("Http-Referer", EndpointOriginProduction)
	header.Set("X-Zcode-App-Version", AppVersion)
	header.Set("X-Title", "Z Code@electron")
	header.Set("X-Platform", runtime.GOOS+"-"+runtime.GOARCH)
	header.Set("X-Release-Channel", "production")
	header.Set("X-Client-Language", clientLanguage())
	header.Set("X-Client-Timezone", clientTimezone())
	header.Set("X-Os-Category", osCategory())
	header.Set("X-Os-Version", osVersion())
	header.Set("X-Device-Mid", deviceMid())
	header.Set("X-Request-ID", uuid.NewString())
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return header
}

// clientLanguage mirrors the client's Intl locale lookup (fallback "unknown").
func clientLanguage() string {
	loc := os.Getenv("LANG")
	if loc == "" {
		loc = os.Getenv("LC_ALL")
	}
	if i := strings.IndexAny(loc, ".@"); i > 0 {
		loc = loc[:i]
	}
	if loc == "" {
		return "unknown"
	}
	return loc
}

// clientTimezone mirrors the client's Intl timeZone lookup (fallback "unknown").
func clientTimezone() string {
	return time.Local.String()
}

// deviceMid reads the telemetry device id the client copies into X-Device-Mid;
// the file only exists on real desktop installs, so empty means omit.
func deviceMid() string {
	if home, err := os.UserHomeDir(); err == nil {
		for _, rel := range []string{".zcode/telemetry-state.json", "AppData/Roaming/ZCode/telemetry-state.json"} {
			if data, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel))); err == nil {
				var state struct {
					DeviceMid string `json:"deviceMid"`
				}
				if json.Unmarshal(data, &state) == nil && state.DeviceMid != "" {
					return state.DeviceMid
				}
			}
		}
	}
	return "unknown"
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
// token (grant == "refresh_token") for the OAuth access token pair. The
// provider selects the envelope branch (Provider for z.ai, BigModelProvider for
// bigmodel.cn).
func exchangeToken(ctx context.Context, client *httpclient.HttpClient, tokenURL, provider, code, refreshToken, redirectURI, state string) (*TokenEnvelope, error) {
	if client == nil {
		return nil, errors.New("http client is nil")
	}

	reqBody := map[string]string{
		"provider": provider,
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

	envelope, err := decodeJSONResponse[TokenEnvelope](resp.Body)
	if err != nil {
		return nil, err
	}

	// The envelope is successful when code is absent, 0, or 200.
	if envelope.Code != nil && *envelope.Code != 0 && *envelope.Code != http.StatusOK {
		return nil, fmt.Errorf("token request failed: code=%d msg=%s", *envelope.Code, envelope.Msg)
	}

	if providerAccessToken(envelope, provider) == "" {
		return nil, fmt.Errorf("token response missing data.%s.access_token", provider)
	}

	return envelope, nil
}

// providerAccessToken reads the provider-nested access token from the envelope.
func providerAccessToken(envelope *TokenEnvelope, provider string) string {
	if provider == BigModelProvider {
		return envelope.Data.BigModel.AccessToken
	}
	return envelope.Data.ZAI.AccessToken
}

// providerRefreshToken reads the provider-nested refresh token from the envelope.
func providerRefreshToken(envelope *TokenEnvelope, provider string) string {
	if provider == BigModelProvider {
		return envelope.Data.BigModel.RefreshToken
	}
	return envelope.Data.ZAI.RefreshToken
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
// plus the business JWT (the inference credential). The provider selects which
// envelope branch holds the OAuth access/refresh tokens.
func buildCreds(envelope *TokenEnvelope, provider, jwt string, previous *oauth.OAuthCredentials) *oauth.OAuthCredentials {
	clientID := ClientID
	if provider == BigModelProvider {
		clientID = BigModelAppID
	}

	creds := &oauth.OAuthCredentials{
		ClientID:     clientID,
		AccessToken:  providerAccessToken(envelope, provider),
		RefreshToken: providerRefreshToken(envelope, provider),
		IDToken:      "",
		ExpiresAt:    time.Now().Add(expiresIn(envelope)),
		TokenType:    "",
		Scopes:       nil,
		KimiCode:     nil,
		ZCode:        &oauth.ZCodeMetadata{BusinessJWT: jwt, Provider: provider},
	}

	// Refresh responses do not always return a new refresh token.
	if creds.RefreshToken == "" && previous != nil {
		creds.RefreshToken = previous.RefreshToken
	}

	// The provisioned coding-plan key outlives token refreshes; carry it over.
	if creds.ZCode.APIKeyID == "" && previous != nil && previous.ZCode != nil {
		creds.ZCode.APIKeyID = previous.ZCode.APIKeyID
		creds.ZCode.APIKeySecret = previous.ZCode.APIKeySecret
	}

	return creds
}

func expiresIn(envelope *TokenEnvelope) time.Duration {
	if envelope.Data.ExpiresIn > 0 {
		return time.Duration(envelope.Data.ExpiresIn) * time.Second
	}
	return 1 * time.Hour
}

// Exchange performs the full login flow: authorization code → OAuth access
// token → business JWT. The credentials are cached in the provider. The
// BigModel provider returns the zcode JWT inline (data.token) and skips the
// business-login step the Z.AI provider needs.
func (p *TokenProvider) Exchange(ctx context.Context, params ExchangeParams) (*oauth.OAuthCredentials, error) {
	if params.Code == "" {
		return nil, errors.New("code is empty")
	}

	provider := params.Provider
	if provider == "" {
		provider = Provider
	}

	redirectURI := params.RedirectURI
	if redirectURI == "" {
		redirectURI = RedirectURI
	}

	envelope, err := exchangeToken(ctx, p.httpClient, p.tokenURL, provider, params.Code, "", redirectURI, params.State)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	jwt, err := p.resolveJWT(ctx, provider, envelope)
	if err != nil {
		return nil, err
	}

	creds := buildCreds(envelope, provider, jwt, nil)

	// The BigModel coding-plan endpoints authenticate with a two-part API key,
	// not the JWT — provision it right after the exchange so the channel is
	// usable for inference immediately.
	if provider == BigModelProvider {
		apiKeyID, apiKeySecret, err := ResolveCodingPlanKey(ctx, p.httpClient, creds.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("provision coding-plan key: %w", err)
		}
		creds.ZCode.APIKeyID = apiKeyID
		creds.ZCode.APIKeySecret = apiKeySecret
	}

	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// CompleteCliFlow turns a ready cli/poll payload into persisted credentials:
// the JWT comes inline (data.token) for bigmodel, so the token exchange and
// business-login steps are skipped entirely.
func (p *TokenProvider) CompleteCliFlow(ctx context.Context, envelope *TokenEnvelope) (*oauth.OAuthCredentials, error) {
	provider := CliFlowProvider

	jwt := strings.TrimSpace(envelope.Data.Token)
	if jwt == "" {
		return nil, errors.New("bigmodel token response missing data.token")
	}

	creds := buildCreds(envelope, provider, jwt, nil)

	// The BigModel coding-plan endpoints authenticate with a two-part API key,
	// not the JWT — provision it so the channel is usable immediately.
	apiKeyID, apiKeySecret, err := ResolveCodingPlanKey(ctx, p.httpClient, creds.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("provision coding-plan key: %w", err)
	}
	creds.ZCode.APIKeyID = apiKeyID
	creds.ZCode.APIKeySecret = apiKeySecret

	p.mu.Lock()
	p.creds = creds
	p.mu.Unlock()

	return creds, nil
}

// resolveJWT obtains the business JWT after a token exchange. BigModel returns
// it inline as data.token; Z.AI requires the business-login exchange.
func (p *TokenProvider) resolveJWT(ctx context.Context, provider string, envelope *TokenEnvelope) (string, error) {
	if provider == BigModelProvider {
		jwt := strings.TrimSpace(envelope.Data.Token)
		if jwt == "" {
			return "", errors.New("bigmodel token response missing data.token")
		}
		return jwt, nil
	}

	return exchangeBusinessJWT(ctx, p.httpClient, p.businessURL, providerAccessToken(envelope, provider))
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

	// The provider is persisted on the credential so refresh re-runs the same
	// login flow that produced it (z.ai business-login vs bigmodel inline token).
	provider := Provider
	if creds.ZCode != nil && creds.ZCode.Provider != "" {
		provider = creds.ZCode.Provider
	}

	envelope, err := exchangeToken(ctx, p.httpClient, p.tokenURL, provider, "", creds.RefreshToken, "", "")
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	jwt, err := p.resolveJWT(ctx, provider, envelope)
	if err != nil {
		return nil, err
	}

	return buildCreds(envelope, provider, jwt, creds), nil
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
