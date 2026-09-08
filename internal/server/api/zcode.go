package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic/zcode"
)

type ZCodeHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
}

type ZCodeHandlers struct {
	// stateCache only records that a session exists; ZCode has no PKCE, so
	// there is no code verifier to store.
	stateCache xcache.Cache[struct{}]
	httpClient *httpclient.HttpClient
}

func NewZCodeHandlers(params ZCodeHandlersParams) *ZCodeHandlers {
	return &ZCodeHandlers{
		stateCache: xcache.NewFromConfig[struct{}](params.CacheConfig),
		httpClient: params.HttpClient,
	}
}

type StartZCodeOAuthRequest struct{}

type StartZCodeOAuthResponse struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
}

func generateZCodeState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

func zcodeOAuthCacheKey(sessionID string) string {
	return fmt.Sprintf("zcode:oauth:%s", sessionID)
}

// StartOAuth creates an OAuth session and returns the BigModel authorize URL.
// POST /admin/zcode/oauth/start.
func (h *ZCodeHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	var req StartZCodeOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	state, err := generateZCodeState()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate oauth state: %w", err))
		return
	}

	// The authorization code lives only a few minutes, so the session does
	// not need to outlive it by much.
	if err := h.stateCache.Set(ctx, zcodeOAuthCacheKey(state), struct{}{}, xcache.WithExpiration(10*time.Minute)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to save oauth state: %w", err))
		return
	}

	// BigModel login uses custom query params (appId/redirect/state), unlike the
	// standard OAuth2 shape — see the ZCode client's BigModel provider adapter.
	params := url.Values{}
	params.Set("redirect", zcode.RedirectURI)
	params.Set("appId", zcode.BigModelAppID)
	params.Set("state", state)

	c.JSON(http.StatusOK, StartZCodeOAuthResponse{
		SessionID: state,
		AuthURL:   fmt.Sprintf("%s?%s", zcode.BigModelAuthorizeURL, params.Encode()),
	})
}

type ExchangeZCodeOAuthRequest struct {
	SessionID   string                  `json:"session_id" binding:"required"`
	CallbackURL string                  `json:"callback_url" binding:"required"`
	Proxy       *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

type ExchangeZCodeOAuthResponse struct {
	Credentials string `json:"credentials"`
}

func parseZCodeCallbackURL(callbackURL string) (code, state string, err error) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid callback_url: %w", err)
	}

	q := u.Query()

	code = q.Get("code")
	if code == "" {
		return "", "", errors.New("code parameter not found in callback_url")
	}

	state = q.Get("state")
	if state == "" {
		return "", "", errors.New("state parameter not found in callback_url")
	}

	return code, state, nil
}

// zcodeCallbackRedirectURI rebuilds the redirect_uri that must accompany the
// token exchange from the pasted callback URL, mirroring the reference CLI's
// manual mode (zcode_auth.py cmd_code). The token endpoint validates
// redirect_uri against the authorize-time value — replaying the pasted URL's
// scheme://host/path keeps them consistent whatever registration z.ai uses.
func zcodeCallbackRedirectURI(callbackURL string) string {
	u, err := url.Parse(callbackURL)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Path == "" {
		return zcode.RedirectURI
	}

	return u.Scheme + "://" + u.Host + u.Path
}

// Exchange exchanges the callback URL for OAuth credentials JSON.
// POST /admin/zcode/oauth/exchange.
func (h *ZCodeHandlers) Exchange(c *gin.Context) {
	ctx := c.Request.Context()

	var req ExchangeZCodeOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	cacheKey := zcodeOAuthCacheKey(req.SessionID)

	if _, err := h.stateCache.Get(ctx, cacheKey); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}

	if err := h.stateCache.Delete(ctx, cacheKey); err != nil {
		log.Warn(ctx, "failed to delete used oauth state from cache", log.String("session_id", req.SessionID), log.Cause(err))
	}

	code, callbackState, err := parseZCodeCallbackURL(req.CallbackURL)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	if callbackState != req.SessionID {
		JSONError(c, http.StatusBadRequest, errors.New("oauth state mismatch"))
		return
	}

	httpClient := h.httpClient
	if req.Proxy != nil && req.Proxy.Type == httpclient.ProxyTypeURL && req.Proxy.URL != "" {
		httpClient = h.httpClient.WithProxy(req.Proxy)
	}

	provider := zcode.NewTokenProvider(zcode.TokenProviderParams{
		Credentials: nil,
		HTTPClient:  httpClient,
		OnRefreshed: nil,
	})

	creds, err := provider.Exchange(ctx, zcode.ExchangeParams{
		Code:        code,
		RedirectURI: zcodeCallbackRedirectURI(req.CallbackURL),
		State:       callbackState,
		Provider:    zcode.BigModelProvider,
	})
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("token exchange failed: %w", err))
		return
	}

	output, err := creds.ToJSON()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to encode credentials: %w", err))
		return
	}

	c.JSON(http.StatusOK, ExchangeZCodeOAuthResponse{Credentials: output})
}
