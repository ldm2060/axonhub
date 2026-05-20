package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/user"
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

	userService   *UserService
	authService   *AuthService
	systemService *SystemService
}

// SignUpServiceParams holds the dependencies for SignUpService.
type SignUpServiceParams struct {
	fx.In

	Ent            *ent.Client
	UserService    *UserService
	AuthService    *AuthService
	SystemService  *SystemService
}

// NewSignUpService creates a new SignUpService.
func NewSignUpService(params SignUpServiceParams) *SignUpService {
	return &SignUpService{
		AbstractService: &AbstractService{db: params.Ent},
		userService:     params.UserService,
		authService:     params.AuthService,
		systemService:   params.SystemService,
	}
}

// AllowSignUp checks if self-registration is enabled.
func (s *SignUpService) AllowSignUp(ctx context.Context) bool {
	value, err := s.systemService.getSystemValue(ctx, SystemKeyAllowSignUp)
	if err != nil {
		return false
	}
	return value == "true"
}

// SignUp registers a new user.
func (s *SignUpService) SignUp(ctx context.Context, input SignUpInput) (*ent.User, string, error) {
	if !s.AllowSignUp(ctx) {
		return nil, "", fmt.Errorf("sign-up is not allowed")
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

	approvalValue, _ := s.systemService.getSystemValue(ctx, SystemKeySignUpApprovalRequired)
	approvalRequired := approvalValue == "true"

	status := user.StatusActivated
	if approvalRequired {
		status = user.StatusDeactivated
	}

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

	var token string
	if !approvalRequired {
		token, err = s.authService.GenerateJWTToken(ctx, newUser)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate token: %w", err)
		}
	}

	return newUser, token, nil
}
