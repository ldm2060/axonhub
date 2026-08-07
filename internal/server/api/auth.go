package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type AuthHandlersParams struct {
	fx.In

	AuthService       *biz.AuthService
	TurnstileVerifier biz.TurnstileVerifier
}

func NewAuthHandlers(params AuthHandlersParams) *AuthHandlers {
	return &AuthHandlers{
		AuthService:       params.AuthService,
		TurnstileVerifier: params.TurnstileVerifier,
	}
}

type AuthHandlers struct {
	AuthService       *biz.AuthService
	TurnstileVerifier biz.TurnstileVerifier
}

// SignInRequest 登录请求.
type SignInRequest struct {
	Email          string `json:"email"             binding:"required,email"`
	Password       string `json:"password"          binding:"required"`
	TurnstileToken string `json:"turnstile_token"`
}

// SignInResponse 登录响应.
type SignInResponse struct {
	User  *objects.UserInfo `json:"user"`
	Token string            `json:"token"`
}

// Config returns public authentication configuration without secret material.
func (h *AuthHandlers) Config(c *gin.Context) {
	config, err := h.TurnstileVerifier.PublicConfig(c.Request.Context())
	if err != nil {
		c.Header("Cache-Control", "no-store")
		JSONError(c, http.StatusServiceUnavailable, errors.New("Authentication configuration is temporarily unavailable"))
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"turnstile": config})
}

// SignIn handles user authentication.
func (h *AuthHandlers) SignIn(c *gin.Context) {
	var (
		ctx = c.Request.Context()
		req SignInRequest
	)

	err := c.ShouldBindJSON(&req)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	// Authenticate user
	user, err := h.AuthService.AuthenticateUser(ctx, req.Email, req.Password, req.TurnstileToken)
	if err != nil {
		switch {
		case errors.Is(err, biz.ErrTurnstileInvalid):
			JSONError(c, http.StatusBadRequest, errors.New("Turnstile verification failed. Please try again"))
			return
		case errors.Is(err, biz.ErrTurnstileUnavailable):
			JSONError(c, http.StatusServiceUnavailable, errors.New("Turnstile verification is temporarily unavailable"))
			return
		case errors.Is(err, biz.ErrInvalidPassword):
			JSONError(c, http.StatusUnauthorized, errors.New("Invalid email or password"))
			return
		default:
			JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))
			return
		}
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateJWTToken(ctx, user)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))
		return
	}

	response := SignInResponse{
		User:  biz.ConvertUserToUserInfo(ctx, user),
		Token: token,
	}

	c.JSON(http.StatusOK, response)
}
