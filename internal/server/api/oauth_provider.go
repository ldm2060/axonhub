package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type OAuthProviderHandlers struct {
	service *biz.OAuthProviderService
}

type OAuthProviderHandlerParams struct {
	fx.In

	OAuthProviderService *biz.OAuthProviderService
}

func NewOAuthProviderHandlers(params OAuthProviderHandlerParams) *OAuthProviderHandlers {
	return &OAuthProviderHandlers{
		service: params.OAuthProviderService,
	}
}

// RegisterRoutes mounts the OIDC provider endpoints. It is called with the
// server root because the discovery document lives at /.well-known.
func (h *OAuthProviderHandlers) RegisterRoutes(r gin.IRouter) {
	r.GET("/.well-known/openid-configuration", h.Metadata)

	group := r.Group("/oauth2")
	group.GET("/authorize", h.Authorize)
	group.POST("/token", h.Token)
	group.GET("/userinfo", h.UserInfo)
	group.GET("/jwks.json", h.JWKS)
}

func (h *OAuthProviderHandlers) Metadata(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, h.service.Metadata(c.Request.Context()))
}

func (h *OAuthProviderHandlers) JWKS(c *gin.Context) {
	jwks, err := h.service.JWKS(c.Request.Context())
	if err != nil {
		log.Error(c.Request.Context(), "failed to build oauth jwks", log.Cause(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": biz.OAuthErrorServerError})
		return
	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, jwks)
}

// Authorize validates an authorization request and hands the browser over to
// the consent page. Per RFC 6749, client and redirect_uri errors are rendered
// directly, while parameter errors are redirected back to the client.
func (h *OAuthProviderHandlers) Authorize(c *gin.Context) {
	ctx := c.Request.Context()
	query := c.Request.URL.Query()

	if h.service.PublicURL() == "" {
		c.String(http.StatusInternalServerError, "server.public_url must be configured to use AxonHub as an OIDC provider")
		return
	}

	client, err := h.service.ValidateAuthorizeClient(ctx, query.Get("client_id"), query.Get("redirect_uri"))
	if err != nil {
		c.Header("Cache-Control", "no-store")
		c.String(http.StatusBadRequest, "invalid authorization request: %s", err.Error())
		return
	}

	requestID, err := h.service.CreateAuthorizationRequest(ctx, client, biz.AuthorizationRequestParams{
		ClientID:            query.Get("client_id"),
		RedirectURI:         query.Get("redirect_uri"),
		ResponseType:        query.Get("response_type"),
		Scope:               query.Get("scope"),
		State:               query.Get("state"),
		Nonce:               query.Get("nonce"),
		CodeChallenge:       query.Get("code_challenge"),
		CodeChallengeMethod: query.Get("code_challenge_method"),
	})
	if err != nil {
		if oauthErr, ok := errors.AsType[*biz.OAuthError](err); ok {
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusFound, biz.BuildOAuthErrorRedirect(query.Get("redirect_uri"), query.Get("state"), oauthErr))
			return
		}

		log.Error(ctx, "failed to create oauth authorization request", log.Cause(err))
		c.String(http.StatusInternalServerError, "failed to create authorization request")
		return
	}

	consentURL := h.service.PublicURL() + "/oauth/consent?request_id=" + url.QueryEscape(requestID)
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, consentURL)
}

func (h *OAuthProviderHandlers) Token(c *gin.Context) {
	ctx := c.Request.Context()

	clientID, clientSecret := extractClientCredentials(c)

	grantType := c.PostForm("grant_type")
	if grantType != "authorization_code" {
		writeOAuthError(c, &biz.OAuthError{
			Code:        "unsupported_grant_type",
			Description: "only the authorization_code grant is supported",
		})
		return
	}

	response, err := h.service.ExchangeAuthorizationCode(ctx, biz.TokenExchangeParams{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         c.PostForm("code"),
		RedirectURI:  c.PostForm("redirect_uri"),
		CodeVerifier: c.PostForm("code_verifier"),
	})
	if err != nil {
		writeOAuthError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"access_token": response.AccessToken,
		"id_token":     response.IDToken,
		"token_type":   response.TokenType,
		"expires_in":   response.ExpiresIn,
		"scope":        response.Scope,
	})
}

func (h *OAuthProviderHandlers) UserInfo(c *gin.Context) {
	token := ""
	if bearer, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
		token = strings.TrimSpace(bearer)
	}

	if token == "" {
		c.Header("WWW-Authenticate", `Bearer error="invalid_request"`)
		writeOAuthError(c, &biz.OAuthError{Code: biz.OAuthErrorInvalidRequest, Description: "bearer token is required"})
		return
	}

	claims, err := h.service.UserInfo(c.Request.Context(), token)
	if err != nil {
		if oauthErr, ok := errors.AsType[*biz.OAuthError](err); ok {
			c.Header("WWW-Authenticate", `Bearer error="`+oauthErr.Code+`"`)
		}

		writeOAuthError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, claims)
}

// extractClientCredentials reads client authentication from HTTP Basic or the
// request body (client_secret_basic / client_secret_post).
func extractClientCredentials(c *gin.Context) (string, string) {
	if user, pass, ok := c.Request.BasicAuth(); ok {
		return user, pass
	}

	return c.PostForm("client_id"), c.PostForm("client_secret")
}

func writeOAuthError(c *gin.Context, err error) {
	c.Header("Cache-Control", "no-store")

	if oauthErr, ok := errors.AsType[*biz.OAuthError](err); ok {
		status := http.StatusBadRequest
		if oauthErr.Code == biz.OAuthErrorInvalidClient {
			status = http.StatusUnauthorized
		}

		c.JSON(status, gin.H{
			"error":             oauthErr.Code,
			"error_description": oauthErr.Description,
		})
		return
	}

	log.Error(c.Request.Context(), "unexpected oauth provider error", log.Cause(err))
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":             biz.OAuthErrorServerError,
		"error_description": "internal error",
	})
}
