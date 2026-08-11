package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type SignUpHandlersParams struct {
	fx.In

	SignUpService *biz.SignUpService
	SystemService *biz.SystemService
}

func NewSignUpHandlers(params SignUpHandlersParams) *SignUpHandlers {
	return &SignUpHandlers{
		SignUpService: params.SignUpService,
		SystemService: params.SystemService,
	}
}

type SignUpHandlers struct {
	SignUpService *biz.SignUpService
	SystemService *biz.SystemService
}

type SendVerificationCodeRequest struct {
	Email          string `json:"email" binding:"required,email"`
	TurnstileToken string `json:"turnstile_token"`
}

type SignUpRequest struct {
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required,min=8"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
}

func (h *SignUpHandlers) SendVerificationCode(c *gin.Context) {
	var req SendVerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.SignUpService.SendVerificationCode(c.Request.Context(), req.Email, req.TurnstileToken); err != nil {
		writeSignUpError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Verification code sent"})
}

func (h *SignUpHandlers) SignUp(c *gin.Context) {
	var req SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	ctx := contexts.WithBaseURL(c.Request.Context(), baseURL)

	_, _, err := h.SignUpService.SignUp(ctx, biz.SignUpInput{
		Email:            req.Email,
		Password:         req.Password,
		VerificationCode: req.VerificationCode,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
	})
	if err != nil {
		writeSignUpError(c, err)
		return
	}

	// Tell the frontend whether the new account will require admin approval after email verification.
	pending := false
	if rs, rsErr := h.SystemService.RegistrationSettings(c.Request.Context()); rsErr == nil {
		pending = rs.ApprovalRequired
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Registration successful. Please check your email to verify your account.",
		"pending": pending,
	})
}

func writeSignUpError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, biz.ErrTurnstileInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Turnstile verification failed. Please try again"})
	case errors.Is(err, biz.ErrTurnstileUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Turnstile verification is temporarily unavailable"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func (h *SignUpHandlers) AllowSignUp(c *gin.Context) {
	allowed := h.SignUpService.AllowSignUp(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"allowed": allowed})
}
