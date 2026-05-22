package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

func setupTestSignUpService(t *testing.T) (*SignUpService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}

	systemService := &SystemService{
		AbstractService: &AbstractService{db: client},
		Cache:           xcache.NewFromConfig[ent.System](cacheConfig),
	}

	userService := &UserService{
		AbstractService: &AbstractService{db: client},
		UserCache:       xcache.NewFromConfig[ent.User](cacheConfig),
	}

	emailTokenService := &EmailTokenService{
		AbstractService: &AbstractService{db: client},
	}

	svc := &SignUpService{
		AbstractService:    &AbstractService{db: client},
		userService:        userService,
		authService:        &AuthService{},
		systemService:      systemService,
		emailTokenService:  emailTokenService,
		emailService:       &EmailService{db: client, systemService: systemService},
	}

	return svc, client
}

// TestSignUp_CheckExistingUser_NoAuthContext verifies that the email-existence
// check inside SignUp works without an authenticated user in context.
// This is a regression test for: "ent: check existence: no user in context: ent/privacy: deny rule".
func TestSignUp_CheckExistingUser_NoAuthContext(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	// Use a bypass context for setup only — pre-create a user and enable signup.
	setupCtx := context.Background()
	setupCtx = ent.NewContext(setupCtx, client)
	setupCtx = authz.WithTestBypass(setupCtx)

	_, err := client.User.Create().
		SetEmail("existing@example.com").
		SetPassword("hashedpassword").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	// Enable signup by writing registration settings directly (bypasses email config gate).
	rs := &RegistrationSettings{AllowSignUp: true, DefaultUserScopes: DefaultUserScopes}
	rsBytes, err := json.Marshal(rs)
	require.NoError(t, err)
	err = svc.systemService.setSystemValue(setupCtx, SystemKeyRegistrationSettings, string(rsBytes))
	require.NoError(t, err)

	// Call SignUp on a PLAIN context — no user, no bypass, no principal.
	// This must NOT fail with "ent/privacy: deny" on the existence check.
	// It should either succeed (for new emails) or return "email already registered".
	plainCtx := context.Background()
	plainCtx = ent.NewContext(plainCtx, client)

	// Duplicate email should return "email already registered", not a privacy deny.
	_, _, err = svc.SignUp(plainCtx, SignUpInput{
		Email:    "existing@example.com",
		Password: "password123",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "email already registered")
}
