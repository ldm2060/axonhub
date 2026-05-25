package biz

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/scopes"
)

const (
	verificationCodeTTL            = 5 * time.Minute
	invalidVerificationCodeMessage = "验证码无效或已过期，请重新获取"
)

var errInvalidVerificationCode = errors.New(invalidVerificationCodeMessage)

// DefaultUserScopes are assigned to self-registered users.
var DefaultUserScopes = []string{
	string(scopes.ScopeManageOwnChannels),
	string(scopes.ScopeManageOwnModels),
	string(scopes.ScopeReadChannels),
	string(scopes.ScopeReadAPIKeys),
	string(scopes.ScopeWriteAPIKeys),
	string(scopes.ScopeReadRequests),
	string(scopes.ScopeWriteRequests),
	string(scopes.ScopeReadPrompts),
	string(scopes.ScopeWritePrompts),
	string(scopes.ScopeReadRoles),
	string(scopes.ScopeWriteRoles),
}

// SignUpInput is the input for user self-registration.
type SignUpInput struct {
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required,min=8"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
}

type verificationEmailSender interface {
	SendVerificationEmail(ctx context.Context, userEmail, userName, actionURL string) error
}

// SignUpService handles user self-registration.
type SignUpService struct {
	*AbstractService

	userService       *UserService
	authService       *AuthService
	systemService     *SystemService
	emailTokenService *EmailTokenService
	emailService      verificationEmailSender
}

// SignUpServiceParams holds the dependencies for SignUpService.
type SignUpServiceParams struct {
	fx.In

	Ent               *ent.Client
	UserService       *UserService
	AuthService       *AuthService
	SystemService     *SystemService
	EmailTokenService *EmailTokenService
	EmailService      *EmailService
}

// NewSignUpService creates a new SignUpService.
func NewSignUpService(params SignUpServiceParams) *SignUpService {
	return &SignUpService{
		AbstractService:   &AbstractService{db: params.Ent},
		userService:       params.UserService,
		authService:       params.AuthService,
		systemService:     params.SystemService,
		emailTokenService: params.EmailTokenService,
		emailService:      params.EmailService,
	}
}

// AllowSignUp checks if self-registration is enabled.
// Reads from the consolidated RegistrationSettings (with migration from the legacy key).
func (s *SignUpService) AllowSignUp(ctx context.Context) bool {
	rs, err := authz.RunWithSystemBypass(ctx, "signup-allowed", func(bypassCtx context.Context) (*RegistrationSettings, error) {
		return s.systemService.RegistrationSettings(bypassCtx)
	})
	if err != nil {
		return false
	}
	return rs.AllowSignUp
}

func (s *SignUpService) signUpDefaultScopes(rs *RegistrationSettings) []string {
	if rs != nil && rs.DefaultUserScopes != nil {
		return rs.DefaultUserScopes
	}

	return DefaultUserScopes
}

func (s *SignUpService) validateRegistrationEmail(ctx context.Context, email string) (*RegistrationSettings, error) {
	rs, err := s.systemService.RegistrationSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get registration settings: %w", err)
	}
	if !rs.AllowSignUp {
		return nil, fmt.Errorf("sign-up is not allowed")
	}
	if err := validateEmailPatterns(email, rs.EmailAllowPatterns, rs.EmailDenyPatterns); err != nil {
		return nil, err
	}

	return rs, nil
}

// SendVerificationCode creates and emails a registration verification code.
func (s *SignUpService) SendVerificationCode(ctx context.Context, email string) error {
	ctx = authz.WithSystemBypass(ctx, "signup")
	normalizedEmail := normalizeEmail(email)

	if _, err := s.validateRegistrationEmail(ctx, normalizedEmail); err != nil {
		return err
	}

	client := s.entFromContext(ctx)
	exists, err := client.User.Query().
		Where(user.EmailEqualFold(normalizedEmail)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return fmt.Errorf("email already registered")
	}

	code, err := s.emailTokenService.CreateEmailCode(ctx, normalizedEmail, emailtoken.TypeVerifyEmail, verificationCodeTTL)
	if err != nil {
		return fmt.Errorf("failed to create verification code: %w", err)
	}

	if err := s.emailService.SendVerificationEmail(ctx, normalizedEmail, normalizedEmail, code); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}

// SignUp registers a new user after verifying the email code.
func (s *SignUpService) SignUp(ctx context.Context, input SignUpInput) (*ent.User, string, error) {
	ctx = authz.WithSystemBypass(ctx, "signup")
	normalizedEmail := normalizeEmail(input.Email)

	rs, err := s.validateRegistrationEmail(ctx, normalizedEmail)
	if err != nil {
		return nil, "", err
	}

	client := s.entFromContext(ctx)
	exists, err := client.User.Query().
		Where(user.EmailEqualFold(normalizedEmail)).
		Exist(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return nil, "", fmt.Errorf("email already registered")
	}

	if _, err := s.emailTokenService.ValidateEmailCode(ctx, normalizedEmail, input.VerificationCode, emailtoken.TypeVerifyEmail); err != nil {
		return nil, "", errInvalidVerificationCode
	}

	defaultScopes := s.signUpDefaultScopes(rs)
	targetStatus := user.StatusPending
	if !rs.ApprovalRequired {
		targetStatus = user.StatusActivated
	}

	var createdUser *ent.User
	err = s.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.emailTokenService.ConsumeEmailCode(txCtx, normalizedEmail, input.VerificationCode, emailtoken.TypeVerifyEmail); err != nil {
			return errInvalidVerificationCode
		}

		createdUser, err = s.userService.CreateUser(txCtx, ent.CreateUserInput{
			Email:     input.Email,
			Password:  input.Password,
			FirstName: &input.FirstName,
			LastName:  &input.LastName,
			Scopes:    defaultScopes,
		})
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		createdUser, err = s.entFromContext(txCtx).User.UpdateOneID(createdUser.ID).
			SetStatus(targetStatus).
			SetScopes(defaultScopes).
			SetEmailVerifiedAt(time.Now()).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to finalize user sign-up: %w", err)
		}

		s.userService.invalidateUserCache(txCtx, createdUser.ID)

		return nil
	})
	if err != nil {
		return nil, "", err
	}

	return createdUser, "", nil
}

// validateEmailPatterns checks the email against allow and deny regex lists.
// If allow patterns are non-empty, the email must match at least one.
// If deny patterns are non-empty, the email must not match any.
func validateEmailPatterns(email string, allowPatterns, denyPatterns []string) error {
	if len(allowPatterns) > 0 {
		matched := false
		for _, p := range allowPatterns {
			re, err := regexp.Compile(p)
			if err != nil {
				continue
			}
			if re.MatchString(email) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("this email address is not in the allowed list")
		}
	}

	for _, p := range denyPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		if re.MatchString(email) {
			return fmt.Errorf("this email address is not allowed")
		}
	}

	return nil
}
