package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

// EmailTokenAPIParams holds the dependencies for EmailTokenAPI.
type EmailTokenAPIParams struct {
	fx.In

	EmailTokenService *biz.EmailTokenService
	EmailService      *biz.EmailService
	UserService       *biz.UserService
	SystemService     *biz.SystemService
}

// EmailTokenAPI provides REST handlers for email verification and password reset flows.
type EmailTokenAPI struct {
	emailTokenService *biz.EmailTokenService
	emailService      *biz.EmailService
	userService       *biz.UserService
	systemService     *biz.SystemService
	rateLimiter       map[string]time.Time
	rateMu            sync.Mutex
}

// NewEmailTokenAPI creates a new EmailTokenAPI.
func NewEmailTokenAPI(params EmailTokenAPIParams) *EmailTokenAPI {
	return &EmailTokenAPI{
		emailTokenService: params.EmailTokenService,
		emailService:      params.EmailService,
		userService:       params.UserService,
		systemService:     params.SystemService,
		rateLimiter:       make(map[string]time.Time),
	}
}

// requestBaseURL derives the base URL from the request when public_url is not configured.
func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

// checkRateLimit enforces a 1 request per minute rate limit per key.
// Returns true if the request is allowed, false if rate limited.
func (h *EmailTokenAPI) checkRateLimit(key string) bool {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()

	now := time.Now()
	if last, ok := h.rateLimiter[key]; ok && now.Sub(last) < time.Minute {
		return false
	}
	h.rateLimiter[key] = now

	// Periodic cleanup: remove entries older than 5 minutes.
	if len(h.rateLimiter) > 1000 {
		cutoff := now.Add(-5 * time.Minute)
		for k, v := range h.rateLimiter {
			if v.Before(cutoff) {
				delete(h.rateLimiter, k)
			}
		}
	}

	return true
}

// VerifyEmail handles GET /auth/verify-email?token=...
// Validates the token, marks email as verified, and redirects to the frontend.
//
// Like ResetPassword, the token is consumed only after the work it authorizes
// has succeeded, so a transient failure leaves the link usable for a retry.
func (h *EmailTokenAPI) VerifyEmail(c *gin.Context) {
	ctx := c.Request.Context()
	token := c.Query("token")
	if token == "" {
		log.Warn(ctx, "Email verification requested without token")
		c.Redirect(http.StatusFound, "/verify-email?verified=0")
		return
	}

	userID, err := h.emailTokenService.ValidateToken(ctx, token, emailtoken.TypeVerifyEmail)
	if err != nil {
		log.Error(ctx, "Email verification token validation failed",
			zap.Error(err),
			zap.String("token_prefix", token[:min(8, len(token))]))
		c.Redirect(http.StatusFound, "/verify-email?verified=0")
		return
	}

	// Mark email as verified
	if err := h.userService.MarkEmailVerified(ctx, userID); err != nil {
		log.Error(ctx, "Failed to mark email as verified",
			zap.Error(err),
			zap.Int("user_id", userID))
		c.Redirect(http.StatusFound, "/verify-email?verified=0")
		return
	}

	// Check if approval is required
	rs, err := h.systemService.RegistrationSettings(ctx)
	if err != nil {
		log.Error(ctx, "Failed to check registration settings during email verification",
			zap.Error(err),
			zap.Int("user_id", userID))
		h.consumeVerificationToken(ctx, token, userID)
		c.Redirect(http.StatusFound, "/verify-email?verified=1&pending=1")
		return
	}

	if rs.ApprovalRequired {
		log.Info(ctx, "Email verified, awaiting admin approval",
			zap.Int("user_id", userID))
		h.consumeVerificationToken(ctx, token, userID)
		c.Redirect(http.StatusFound, "/verify-email?verified=1&pending=1")
		return
	}

	// Auto-activate the user
	if err := h.userService.ActivateUser(ctx, userID); err != nil {
		log.Error(ctx, "Failed to activate user after email verification",
			zap.Error(err),
			zap.Int("user_id", userID))
		h.consumeVerificationToken(ctx, token, userID)
		c.Redirect(http.StatusFound, "/verify-email?verified=1&pending=1")
		return
	}

	h.consumeVerificationToken(ctx, token, userID)

	log.Info(ctx, "Email verified and user activated",
		zap.Int("user_id", userID))
	c.Redirect(http.StatusFound, "/verify-email?verified=1")
}

// consumeVerificationToken burns a verification token that has already done its
// job. A failure here is logged rather than surfaced: the email is verified, so
// failing the redirect would tell the user their link is broken when it is not.
func (h *EmailTokenAPI) consumeVerificationToken(ctx context.Context, token string, userID int) {
	if err := h.emailTokenService.ConsumeToken(ctx, token); err != nil {
		log.Error(ctx, "Failed to consume email verification token",
			zap.Error(err),
			zap.Int("user_id", userID))
	}
}

// ResendVerificationRequest is the request body for resending a verification email.
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResendVerification handles POST /auth/resend-verification
// Rate-limited per email. Resends verification email to pending users with unverified emails.
func (h *EmailTokenAPI) ResendVerification(c *gin.Context) {
	ctx := c.Request.Context()
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	// Rate limit per email
	if !h.checkRateLimit("resend:" + req.Email) {
		JSONError(c, http.StatusTooManyRequests, errors.New("Please wait before requesting another email"))
		return
	}

	// Find pending user with unverified email
	u, err := h.userService.GetByEmail(ctx, req.Email)
	if err != nil {
		// Don't reveal if email exists — return generic success
		c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists and is unverified, a new verification email has been sent."})
		return
	}

	if u.Status != user.StatusPending || u.EmailVerifiedAt != nil {
		// Don't reveal if email exists or is already verified
		c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists and is unverified, a new verification email has been sent."})
		return
	}

	// Create new verification token
	token, err := h.emailTokenService.CreateToken(ctx, u.ID, emailtoken.TypeVerifyEmail)
	if err != nil {
		// Return generic success to avoid leaking info
		c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists and is unverified, a new verification email has been sent."})
		return
	}

	// Send verification email
	verifyURL := h.emailService.BuildURLWithBase(c.Request.Context(), fmt.Sprintf("/auth/verify-email?token=%s", token), requestBaseURL(c))
	userName := u.FirstName
	if userName == "" {
		userName = u.Email
	}
	_ = h.emailService.SendVerificationEmail(ctx, u.Email, userName, verifyURL)

	c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists and is unverified, a new verification email has been sent."})
}

// ForgotPasswordRequest is the request body for forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword handles POST /auth/forgot-password
// Rate-limited per email. Creates a reset token and sends a reset email.
func (h *EmailTokenAPI) ForgotPassword(c *gin.Context) {
	ctx := c.Request.Context()
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	// Rate limit per email
	if !h.checkRateLimit("forgot:" + req.Email) {
		JSONError(c, http.StatusTooManyRequests, errors.New("Please wait before requesting another reset email"))
		return
	}

	// Find activated user
	u, err := h.userService.GetByEmail(ctx, req.Email)
	if err != nil {
		// Don't reveal if email exists
		c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
		return
	}

	if u.Status != user.StatusActivated {
		c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
		return
	}

	// Create reset token
	token, err := h.emailTokenService.CreateToken(ctx, u.ID, emailtoken.TypeResetPassword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
		return
	}

	// Send reset email
	resetURL := h.emailService.BuildURLWithBase(c.Request.Context(), fmt.Sprintf("/reset-password?token=%s", token), requestBaseURL(c))
	userName := u.FirstName
	if userName == "" {
		userName = u.Email
	}
	_ = h.emailService.SendPasswordResetEmail(ctx, u.Email, userName, resetURL)

	c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
}

// ResetPasswordRequest is the request body for reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// ResetPassword handles POST /auth/reset-password
// Validates the token, consumes it, and sets the new password.
//
// The password is changed before the token is consumed: consuming first would
// burn the one-shot token even when the reset fails, and the user would then be
// stuck retrying a link that reports "Invalid or expired reset token" forever.
func (h *EmailTokenAPI) ResetPassword(c *gin.Context) {
	ctx := c.Request.Context()
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	// Validate token
	userID, err := h.emailTokenService.ValidateToken(ctx, req.Token, emailtoken.TypeResetPassword)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid or expired reset token"))
		return
	}

	// Reset password
	if err := h.userService.ResetPassword(ctx, userID, req.Password); err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("Failed to reset password"))
		return
	}

	// Consume token. The password is already changed at this point, so a failure
	// here must not fail the request: report success and let the token expire.
	if err := h.emailTokenService.ConsumeToken(ctx, req.Token); err != nil {
		log.Error(ctx, "Failed to consume reset token after a successful password reset",
			zap.Error(err),
			zap.Int("user_id", userID))
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully."})
}
