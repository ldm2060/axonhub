package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
	"github.com/ldm2060/axonhub/llm/transformer/kimicode"
)

const kimiCodeOAuthCacheKeyPrefix = "kimi-code:oauth"

type KimiCodeHandlersParams struct {
	fx.In

	CacheConfig   xcache.Config
	HttpClient    *httpclient.HttpClient
	SystemService *biz.SystemService
	Clock         Clock `optional:"true"`
}

type KimiCodeHandlers struct {
	deviceCodeCache xcache.Cache[kimiCodeDeviceFlowState]
	httpClient      *httpclient.HttpClient
	systemService   *biz.SystemService
	clock           Clock
}

type kimiCodeDeviceFlowState struct {
	DeviceCode  string                  `json:"device_code"`
	ExpiresIn   int                     `json:"expires_in"`
	Interval    int                     `json:"interval"`
	CreatedAt   int64                   `json:"created_at"`
	Identity    kimicode.Identity       `json:"identity"`
	Proxy       *httpclient.ProxyConfig `json:"proxy,omitempty"`
	Credentials *oauth.OAuthCredentials `json:"credentials,omitempty"`
}

type StartKimiCodeOAuthRequest struct {
	Proxy *httpclient.ProxyConfig `json:"proxy,omitempty"`
}
type StartKimiCodeOAuthResponse struct {
	SessionID               string `json:"session_id"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}
type PollKimiCodeOAuthRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}
type PollKimiCodeOAuthResponse struct {
	Status      string           `json:"status"`
	Message     string           `json:"message,omitempty"`
	Credentials string           `json:"credentials,omitempty"`
	Models      []kimicode.Model `json:"models,omitempty"`
}

func NewKimiCodeHandlers(params KimiCodeHandlersParams) *KimiCodeHandlers {
	clock := params.Clock
	if clock == nil {
		clock = realClock{}
	}
	return &KimiCodeHandlers{deviceCodeCache: xcache.NewFromConfig[kimiCodeDeviceFlowState](params.CacheConfig), httpClient: params.HttpClient, systemService: params.SystemService, clock: clock}
}

func kimiCodeOAuthCacheKey(sessionID string) string {
	return kimiCodeOAuthCacheKeyPrefix + ":" + sessionID
}

func generateKimiCodeSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func (h *KimiCodeHandlers) StartOAuth(c *gin.Context) {
	var input StartKimiCodeOAuthRequest
	if err := c.ShouldBindJSON(&input); err != nil && err.Error() != "EOF" {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}
	identity, err := h.systemService.KimiCodeIdentity(c.Request.Context())
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("resolve Kimi Code identity: %w", err))
		return
	}
	sessionID, err := generateKimiCodeSessionID()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("generate OAuth session: %w", err))
		return
	}
	client := h.clientForProxy(input.Proxy)
	device, err := kimicode.RequestDeviceAuthorization(c.Request.Context(), client, "", identity)
	if err != nil {
		JSONError(c, http.StatusBadGateway, err)
		return
	}
	expiresIn := device.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int((15 * time.Minute).Seconds())
	}
	state := kimiCodeDeviceFlowState{DeviceCode: device.DeviceCode, ExpiresIn: expiresIn, Interval: device.Interval, CreatedAt: h.clock.Now().Unix(), Identity: identity, Proxy: input.Proxy}
	if err := h.deviceCodeCache.Set(c.Request.Context(), kimiCodeOAuthCacheKey(sessionID), state, xcache.WithExpiration(time.Duration(expiresIn)*time.Second)); err != nil {
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("save device flow session: %w", err))
		return
	}
	c.JSON(http.StatusOK, StartKimiCodeOAuthResponse{SessionID: sessionID, UserCode: device.UserCode, VerificationURI: device.VerificationURI, VerificationURIComplete: device.VerificationURIComplete, ExpiresIn: expiresIn, Interval: device.Interval})
}

func (h *KimiCodeHandlers) PollOAuth(c *gin.Context) {
	var input PollKimiCodeOAuthRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}
	key := kimiCodeOAuthCacheKey(input.SessionID)
	state, err := h.deviceCodeCache.Get(c.Request.Context(), key)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid or expired session"))
		return
	}
	if h.clock.Now().Unix() >= state.CreatedAt+int64(state.ExpiresIn) {
		_ = h.deviceCodeCache.Delete(c.Request.Context(), key)
		JSONError(c, http.StatusBadRequest, errors.New("device code expired"))
		return
	}
	client := h.clientForProxy(state.Proxy)
	creds := state.Credentials
	if creds == nil {
		creds, err = kimicode.PollDeviceToken(c.Request.Context(), client, "", state.DeviceCode, state.Identity)
		if err != nil {
			var oauthErr *kimicode.OAuthError
			if errors.As(err, &oauthErr) {
				switch oauthErr.Code {
				case "authorization_pending":
					c.JSON(http.StatusOK, PollKimiCodeOAuthResponse{Status: "pending", Message: oauthErr.Description})
					return
				case "slow_down":
					c.JSON(http.StatusOK, PollKimiCodeOAuthResponse{Status: "slow_down", Message: oauthErr.Description})
					return
				case "expired_token", "access_denied":
					_ = h.deviceCodeCache.Delete(c.Request.Context(), key)
					JSONError(c, http.StatusBadRequest, oauthErr)
					return
				}
			}
			JSONError(c, http.StatusBadGateway, err)
			return
		}
		state.Credentials = creds
	}
	models, err := kimicode.FetchModels(c.Request.Context(), client, kimicode.DefaultBaseURL, creds.AccessToken, state.Identity)
	if err != nil {
		// The token can be valid while catalog discovery has a transient failure.
		// Retain only the short-lived session and retry model discovery on the next poll.
		if setErr := h.deviceCodeCache.Set(c.Request.Context(), key, state, xcache.WithExpiration(time.Until(time.Unix(state.CreatedAt+int64(state.ExpiresIn), 0)))); setErr != nil {
			JSONError(c, http.StatusInternalServerError, setErr)
			return
		}
		JSONError(c, http.StatusBadGateway, fmt.Errorf("fetch Kimi Code models: %w", err))
		return
	}
	creds.KimiCode = kimicode.NewMetadata(models)
	canonical, err := creds.ToJSON()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.deviceCodeCache.Delete(c.Request.Context(), key); err != nil { /* cache expiry is safe fallback */
	}
	c.JSON(http.StatusOK, PollKimiCodeOAuthResponse{Status: "complete", Credentials: canonical, Models: models})
}

func (h *KimiCodeHandlers) clientForProxy(proxy *httpclient.ProxyConfig) *httpclient.HttpClient {
	if proxy != nil && proxy.Type == httpclient.ProxyTypeURL && strings.TrimSpace(proxy.URL) != "" {
		return h.httpClient.WithProxy(proxy)
	}
	return h.httpClient
}
