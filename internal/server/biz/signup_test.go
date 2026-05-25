package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

type recordingVerificationEmailSender struct {
	calls []verificationEmailCall
}

type verificationEmailCall struct {
	userEmail string
	userName  string
	actionURL string
}

func (s *recordingVerificationEmailSender) SendVerificationEmail(_ context.Context, userEmail, userName, actionURL string) error {
	s.calls = append(s.calls, verificationEmailCall{
		userEmail: userEmail,
		userName:  userName,
		actionURL: actionURL,
	})

	return nil
}

func setupTestSignUpService(t *testing.T) (*SignUpService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")

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
		AbstractService:   &AbstractService{db: client},
		userService:       userService,
		authService:       &AuthService{},
		systemService:     systemService,
		emailTokenService: emailTokenService,
		emailService:      &EmailService{db: client, systemService: systemService},
	}

	return svc, client
}


func setRegistrationSettings(t *testing.T, svc *SignUpService, ctx context.Context, settings *RegistrationSettings) {
	t.Helper()

	rsBytes, err := json.Marshal(settings)
	require.NoError(t, err)

	err = svc.systemService.setSystemValue(ctx, SystemKeyRegistrationSettings, string(rsBytes))
	require.NoError(t, err)
}

func enableTestSignUp(t *testing.T, svc *SignUpService, client *ent.Client, approvalRequired bool) context.Context {
	t.Helper()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	setRegistrationSettings(t, svc, ctx, &RegistrationSettings{
		AllowSignUp:        true,
		ApprovalRequired:   approvalRequired,
		DefaultUserScopes:  append([]string(nil), DefaultUserScopes...),
		EmailAllowPatterns: []string{`^[^@]+@example\.com$`},
	})

	return ctx
}

func TestAllowSignUp_NoAuthContextReadsRegistrationSettings(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	enableTestSignUp(t, svc, client, false)

	plainCtx := ent.NewContext(context.Background(), client)

	require.True(t, svc.AllowSignUp(plainCtx))
}

// TestSignUp_CheckExistingUser_NoAuthContext verifies that the email-existence
// check inside SignUp works without an authenticated user in context.
// This is a regression test for: "ent: check existence: no user in context: ent/privacy: deny rule".
func TestSignUp_CheckExistingUser_NoAuthContext(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)

	_, err := client.User.Create().
		SetEmail("existing@example.com").
		SetPassword("hashedpassword").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	plainCtx := ent.NewContext(context.Background(), client)

	_, _, err = svc.SignUp(plainCtx, SignUpInput{
		Email:    "existing@example.com",
		Password: "password123",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "email already registered")
}

func TestSignUp_SendVerificationCode_CreatesEmailToken(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)
	sender := &recordingVerificationEmailSender{}
	svc.emailService = sender

	plainCtx := ent.NewContext(context.Background(), client)

	err := svc.SendVerificationCode(plainCtx, "new@example.com")
	require.NoError(t, err)
	require.Len(t, sender.calls, 1)
	require.Equal(t, "new@example.com", sender.calls[0].userEmail)
	require.Equal(t, "new@example.com", sender.calls[0].userName)
	require.Len(t, sender.calls[0].actionURL, 6)

	token, err := client.EmailToken.Query().
		Where(
			emailtoken.EmailEQ("new@example.com"),
			emailtoken.TypeEQ(emailtoken.TypeVerifyEmail),
		).
		Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, sender.calls[0].actionURL, token.Token)
	require.Nil(t, token.ConsumedAt)
}

func TestSignUp_WithVerificationCode_RejectsInvalidCode(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)
	plainCtx := ent.NewContext(context.Background(), client)

	createdUser, _, err := svc.SignUp(plainCtx, SignUpInput{
		Email:            "missing@example.com",
		Password:         "password123",
		VerificationCode: "000000",
	})
	require.Nil(t, createdUser)
	require.EqualError(t, err, invalidVerificationCodeMessage)

	count, err := client.User.Query().Where(user.EmailEQ("missing@example.com")).Count(setupCtx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestSignUp_WithVerificationCode_RejectsMissingCode(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)
	plainCtx := ent.NewContext(context.Background(), client)

	createdUser, _, err := svc.SignUp(plainCtx, SignUpInput{
		Email:            "nocode@example.com",
		Password:         "password123",
		VerificationCode: "",
	})
	require.Nil(t, createdUser)
	require.EqualError(t, err, "验证码无效或已过期，请重新获取")

	count, err := client.User.Query().Where(user.EmailEQ("nocode@example.com")).Count(setupCtx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestSignUp_WithVerificationCode_ActivatesUserWhenApprovalNotRequired(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)
	customScopes := []string{"scope:activated:test"}
	setRegistrationSettings(t, svc, setupCtx, &RegistrationSettings{
		AllowSignUp:        true,
		ApprovalRequired:   false,
		DefaultUserScopes:  customScopes,
		EmailAllowPatterns: []string{`^[^@]+@example\.com$`},
	})

	sender := &recordingVerificationEmailSender{}
	svc.emailService = sender

	code, err := svc.emailTokenService.CreateEmailCode(setupCtx, "activated@example.com", emailtoken.TypeVerifyEmail, verificationCodeTTL)
	require.NoError(t, err)

	plainCtx := ent.NewContext(context.Background(), client)
	createdUser, _, err := svc.SignUp(plainCtx, SignUpInput{
		Email:            "activated@example.com",
		Password:         "password123",
		FirstName:        "Active",
		VerificationCode: code,
	})
	require.NoError(t, err)
	require.Empty(t, sender.calls)
	require.Equal(t, user.StatusActivated, createdUser.Status)
	require.ElementsMatch(t, customScopes, createdUser.Scopes)
	require.NotNil(t, createdUser.EmailVerifiedAt)

	savedUser, err := client.User.Query().Where(user.EmailEQ("activated@example.com")).Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, user.StatusActivated, savedUser.Status)
	require.ElementsMatch(t, customScopes, savedUser.Scopes)
	require.NotNil(t, savedUser.EmailVerifiedAt)

	token, err := client.EmailToken.Query().
		Where(
			emailtoken.EmailEQ("activated@example.com"),
			emailtoken.TypeEQ(emailtoken.TypeVerifyEmail),
		).
		Only(setupCtx)
	require.NoError(t, err)
	require.NotNil(t, token.ConsumedAt)
}

func TestSignUp_WithVerificationCode_PendingWhenApprovalRequired(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, true)
	customScopes := []string{"scope:pending:test"}
	setRegistrationSettings(t, svc, setupCtx, &RegistrationSettings{
		AllowSignUp:        true,
		ApprovalRequired:   true,
		DefaultUserScopes:  customScopes,
		EmailAllowPatterns: []string{`^[^@]+@example\.com$`},
	})

	sender := &recordingVerificationEmailSender{}
	svc.emailService = sender

	code, err := svc.emailTokenService.CreateEmailCode(setupCtx, "pending@example.com", emailtoken.TypeVerifyEmail, verificationCodeTTL)
	require.NoError(t, err)

	plainCtx := ent.NewContext(context.Background(), client)
	createdUser, _, err := svc.SignUp(plainCtx, SignUpInput{
		Email:            "pending@example.com",
		Password:         "password123",
		LastName:         "Review",
		VerificationCode: code,
	})
	require.NoError(t, err)
	require.Empty(t, sender.calls)
	require.Equal(t, user.StatusPending, createdUser.Status)
	require.ElementsMatch(t, customScopes, createdUser.Scopes)
	require.NotNil(t, createdUser.EmailVerifiedAt)

	savedUser, err := client.User.Query().Where(user.EmailEQ("pending@example.com")).Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, user.StatusPending, savedUser.Status)
	require.ElementsMatch(t, customScopes, savedUser.Scopes)
	require.NotNil(t, savedUser.EmailVerifiedAt)

	token, err := client.EmailToken.Query().
		Where(
			emailtoken.EmailEQ("pending@example.com"),
			emailtoken.TypeEQ(emailtoken.TypeVerifyEmail),
		).
		Only(setupCtx)
	require.NoError(t, err)
	require.NotNil(t, token.ConsumedAt)
}
func TestSignUp_WithVerificationCode_RejectsCaseVariantDuplicateEmail(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)

	_, err := client.User.Create().
		SetEmail("User@Example.com").
		SetPassword("hashedpassword").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	code, err := svc.emailTokenService.CreateEmailCode(setupCtx, "user@example.com", emailtoken.TypeVerifyEmail, verificationCodeTTL)
	require.NoError(t, err)

	plainCtx := ent.NewContext(context.Background(), client)
	createdUser, _, err := svc.SignUp(plainCtx, SignUpInput{
		Email:            "user@example.com",
		Password:         "password123",
		VerificationCode: code,
	})
	require.Nil(t, createdUser)
	require.Error(t, err)
	require.Contains(t, err.Error(), "email already registered")

	count, err := client.User.Query().Count(setupCtx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestSignUp_WithVerificationCode_NormalizesStoredEmail(t *testing.T) {
	svc, client := setupTestSignUpService(t)
	defer client.Close()

	setupCtx := enableTestSignUp(t, svc, client, false)
	code, err := svc.emailTokenService.CreateEmailCode(setupCtx, "user@example.com", emailtoken.TypeVerifyEmail, verificationCodeTTL)
	require.NoError(t, err)

	plainCtx := ent.NewContext(context.Background(), client)
	createdUser, _, err := svc.SignUp(plainCtx, SignUpInput{
		Email:            "User@Example.COM",
		Password:         "password123",
		VerificationCode: code,
	})
	require.NoError(t, err)
	require.NotNil(t, createdUser)
	require.Equal(t, "user@example.com", createdUser.Email)

	storedUser, err := client.User.Query().Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", storedUser.Email)
}
