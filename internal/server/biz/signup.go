package biz

import (
	"context"
	"fmt"
	"regexp"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/scopes"
)

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
}

// SignUpInput is the input for user self-registration.
type SignUpInput struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// SignUpService handles user self-registration.
type SignUpService struct {
	*AbstractService

	userService      *UserService
	authService      *AuthService
	systemService    *SystemService
	emailTokenService *EmailTokenService
	emailService     *EmailService
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
	rs, err := s.systemService.RegistrationSettings(ctx)
	if err != nil {
		return false
	}
	return rs.AllowSignUp
}

// SignUp registers a new user and sends a verification email.
func (s *SignUpService) SignUp(ctx context.Context, input SignUpInput) (*ent.User, string, error) {
	ctx = authz.WithSystemBypass(ctx, "signup")

	if !s.AllowSignUp(ctx) {
		return nil, "", fmt.Errorf("sign-up is not allowed")
	}

	// Validate email against allow/deny patterns.
	rs, err := s.systemService.RegistrationSettings(ctx)
	if err == nil {
		if err := validateEmailPatterns(input.Email, rs.EmailAllowPatterns, rs.EmailDenyPatterns); err != nil {
			return nil, "", err
		}
	}

	client := s.entFromContext(ctx)

	exists, err := client.User.Query().
		Where(user.EmailEQ(input.Email)).Exist(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return nil, "", fmt.Errorf("email already registered")
	}

	status := user.StatusPending

	newUser, err := s.userService.CreateUser(ctx, ent.CreateUserInput{
		Email:     input.Email,
		Password:  input.Password,
		FirstName: &input.FirstName,
		LastName:  &input.LastName,
		Status:    &status,
		Scopes:    DefaultUserScopes,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Create email verification token
	token, err := s.emailTokenService.CreateToken(ctx, newUser.ID, emailtoken.TypeVerifyEmail)
	if err != nil {
		log.Error(ctx, "Failed to create email verification token", log.Cause(err), log.Int("user_id", newUser.ID))
		return nil, "", fmt.Errorf("failed to create verification token: %w", err)
	}

	// Build verification URL using the token
	verifyURL := s.emailService.BuildURL(fmt.Sprintf("/admin/auth/verify-email?token=%s", token))

	// Send verification email
	userName := newUser.FirstName
	if userName == "" {
		userName = newUser.Email
	}
	if err := s.emailService.SendVerificationEmail(ctx, newUser.Email, userName, verifyURL); err != nil {
		log.Error(ctx, "Failed to send verification email", log.Cause(err), log.Int("user_id", newUser.ID))
		return nil, "", fmt.Errorf("failed to send verification email: %w", err)
	}

	return newUser, "", nil
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
