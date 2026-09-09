package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic/zcode"
)

type ZCodeHandlersParams struct {
	fx.In

	CacheConfig xcache.Config
	HttpClient  *httpclient.HttpClient
}

type ZCodeHandlers struct {
	// sessionCache stores the cli-flow session (flow id + poll token) keyed by
	// the state from the authorize URL.
	sessionCache xcache.Cache[zcodeCliFlowSession]
	httpClient   *httpclient.HttpClient
}

// zcodeCliFlowSession is what start persists so exchange can poll the flow
// the user completed in the browser.
type zcodeCliFlowSession struct {
	FlowID     string `json:"flow_id"`
	PollToken  string `json:"poll_token"`
	Interval   int64  `json:"interval_sec"`
	Expiration int64  `json:"expires_at"`
}

func NewZCodeHandlers(params ZCodeHandlersParams) *ZCodeHandlers {
	return &ZCodeHandlers{
		sessionCache: xcache.NewFromConfig[zcodeCliFlowSession](params.CacheConfig),
		httpClient:   params.HttpClient,
	}
}

type StartZCodeOAuthRequest struct{}

type StartZCodeOAuthResponse struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
}

func zcodeOAuthCacheKey(sessionID string) string {
	return fmt.Sprintf("zcode:oauth:%s", sessionID)
}

// zcodeAuthorizeURL rewrites the cli/init authorize URL the way the desktop
// client does: replace the redirect param with the desktop OAuth bridge on the
// runtime endpoint origin, and reuse the server-issued state as session id.
func zcodeAuthorizeURL(authorizeURL string) (state, rewritten string, err error) {
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid authorize_url: %w", err)
	}

	state = u.Query().Get("state")
	if state == "" {
		return "", "", errors.New("authorize_url missing state")
	}

	q := u.Query()
	q.Set("redirect", zcode.DesktopRedirectURI(zcode.EndpointOrigin(os.Getenv)))
	u.RawQuery = q.Encode()

	return state, u.String(), nil
}

// StartOAuth initializes a server-side cli flow and returns the authorize URL.
// POST /admin/zcode/oauth/start.
func (h *ZCodeHandlers) StartOAuth(c *gin.Context) {
	ctx := c.Request.Context()

	var req StartZCodeOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}

	// cli/init only accepts provider "bigmodel" ("zai" → 3004 invalid_flow),
	// matching the desktop client's bigmodel login.
	session, err := zcode.InitCliFlow(ctx, h.httpClient, zcode.EndpointOrigin(os.Getenv))
	if err != nil {
		JSONError(c, http.StatusBadGateway, fmt.Errorf("init oauth flow failed: %w", err))
		return
	}

	state, authURL, err := zcodeAuthorizeURL(session.AuthorizeURL)
	if err != nil {
		JSONError(c, http.StatusBadGateway, err)
		return
	}

	// The flow window is bounded upstream (the desktop client uses 300s); keep
	// the session alive slightly longer than that.
	expiration := zcode.CliFlowMaxWindow + time.Minute
	if session.ExpiresAt > 0 {
		if until := time.Until(time.Unix(session.ExpiresAt, 0)); until > time.Minute && until < 30*time.Minute {
			expiration = until + time.Minute
		}
	}

	if err := h.sessionCache.Set(ctx, zcodeOAuthCacheKey(state), zcodeCliFlowSession{
		FlowID:     session.FlowID,
		PollToken:  session.PollToken,
		Interval:   int64(session.Interval / time.Second),
		Expiration: session.ExpiresAt,
	}, xcache.WithExpiration(expiration)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to save oauth session: %w", err))
		return
	}

	c.JSON(http.StatusOK, StartZCodeOAuthResponse{
		SessionID: state,
		AuthURL:   authURL,
	})
}

type ExchangeZCodeOAuthRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	// CallbackURL is the optional zcode://oauth/callback?...authCode=...&state=...
	// link the browser fires after login. When present, exchange runs the direct
	// code-exchange path instead of polling the cli flow.
	CallbackURL string                  `json:"callback_url,omitempty"`
	Proxy       *httpclient.ProxyConfig `json:"proxy,omitempty"`
}

type ExchangeZCodeOAuthResponse struct {
	Credentials string `json:"credentials"`
}

// pollCliFlowOnce polls the cli flow once, translating the tri-state result.
func pollCliFlowOnce(ctx context.Context, h *ZCodeHandlers, httpClient *httpclient.HttpClient, session zcodeCliFlowSession) (status string, envelope *zcode.TokenEnvelope, err error) {
	status, data, err := zcode.PollCliFlow(ctx, httpClient, zcode.EndpointOrigin(os.Getenv), session.PollToken, session.FlowID)
	if err != nil {
		return "", nil, err
	}
	if status == "ready" {
		return "ready", data, nil
	}
	return status, nil, nil
}

// Exchange resolves the OAuth credentials for a started session. Two paths:
// when the request carries the pasted callback URL (the zcode://oauth/callback
// link the browser fires after login, with authCode + state query params) the
// code is exchanged directly; otherwise the cli flow is polled until the
// upstream reports the login ready.
// POST /admin/zcode/oauth/exchange.
func (h *ZCodeHandlers) Exchange(c *gin.Context) {
	ctx := c.Request.Context()

	var req ExchangeZCodeOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
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

	if req.CallbackURL != "" {
		creds, err := h.exchangeWithCallback(ctx, provider, req)
		if err != nil {
			JSONError(c, http.StatusBadGateway, err)
			return
		}

		output, err := creds.ToJSON()
		if err != nil {
			JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to encode credentials: %w", err))
			return
		}

		c.JSON(http.StatusOK, ExchangeZCodeOAuthResponse{Credentials: output})
		return
	}

	cacheKey := zcodeOAuthCacheKey(req.SessionID)

	session, err := h.sessionCache.Get(ctx, cacheKey)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired oauth session"))
		return
	}

	deadline := zcode.CliFlowMaxWindow
	if session.Expiration > 0 {
		if until := time.Until(time.Unix(session.Expiration, 0)); until > 0 && until < deadline {
			deadline = until
		}
	}

	pollCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	interval := time.Duration(session.Interval) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}

	for {
		status, envelope, err := pollCliFlowOnce(pollCtx, h, httpClient, session)
		if err != nil {
			JSONError(c, http.StatusBadGateway, fmt.Errorf("oauth polling failed: %w", err))
			return
		}

		if status == "ready" {
			creds, err := provider.CompleteCliFlow(pollCtx, envelope)
			if err != nil {
				JSONError(c, http.StatusBadGateway, fmt.Errorf("failed to build credentials: %w", err))
				return
			}

			if err := h.sessionCache.Delete(ctx, cacheKey); err != nil {
				log.Warn(ctx, "failed to delete used oauth session from cache", log.String("session_id", req.SessionID), log.Cause(err))
			}

			output, err := creds.ToJSON()
			if err != nil {
				JSONError(c, http.StatusInternalServerError, fmt.Errorf("failed to encode credentials: %w", err))
				return
			}

			c.JSON(http.StatusOK, ExchangeZCodeOAuthResponse{Credentials: output})
			return
		}

		select {
		case <-pollCtx.Done():
			JSONError(c, http.StatusRequestTimeout, errors.New("oauth login not completed in time; please retry the exchange"))
			return
		case <-time.After(interval):
		}
	}
}

// exchangeWithCallback runs the direct code exchange for a pasted callback
// URL. The redirect_uri must match the authorize-time value — the bare
// zcode://oauth/callback is rejected with code 2007 — so it is rebuilt the
// same way zcodeAuthorizeURL rewrote it.
func (h *ZCodeHandlers) exchangeWithCallback(ctx context.Context, provider *zcode.TokenProvider, req ExchangeZCodeOAuthRequest) (*oauth.OAuthCredentials, error) {
	parsed, err := url.Parse(strings.TrimSpace(req.CallbackURL))
	if err != nil {
		return nil, fmt.Errorf("invalid callback url: %w", err)
	}

	code := parsed.Query().Get("authCode")
	if code == "" {
		code = parsed.Query().Get("code")
	}
	if code == "" {
		return nil, errors.New("callback url missing authCode")
	}

	state := parsed.Query().Get("state")
	if state == "" {
		return nil, errors.New("callback url missing state")
	}

	creds, err := provider.Exchange(ctx, zcode.ExchangeParams{
		Code:        code,
		RedirectURI: zcode.DesktopRedirectURI(zcode.EndpointOrigin(os.Getenv)),
		State:       state,
		Provider:    zcode.BigModelProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}

	return creds, nil
}
