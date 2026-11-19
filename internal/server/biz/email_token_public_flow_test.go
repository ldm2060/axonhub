// 职责: 覆盖邮箱令牌（验证/重置）在无凭证的公开请求上下文里的读写行为。
// 关联文件:
//   internal/server/biz/email_token.go
//   internal/server/biz/user.go
//   internal/server/api/email_token.go
// 注意事项:
//   这些入口都由未认证的公开路由调用，请求上下文里没有任何 authz principal，
//   因此 Biz 侧必须显式 RunWithSystemBypass，否则 Ent 隐私层会直接拒绝写入。

package biz

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

// publicRequestContext returns a bare context, exactly like the one a Gin
// handler gets from c.Request.Context() on an unauthenticated route.
func publicRequestContext(t *testing.T) (*ent.Client, context.Context) {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "email_token_public.db")) + "?_fk=1&_pragma=journal_mode(WAL)"
	client := enttest.NewEntClient(t, "sqlite3", dsn)
	ctx := ent.NewContext(context.Background(), client)
	return client, ctx
}

func seedActivatedUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()
	u, err := authz.RunWithSystemBypass(ctx, "test-seed-user", func(bypassCtx context.Context) (*ent.User, error) {
		return client.User.Create().
			SetEmail(email).
			SetFirstName("Repro").
			SetPassword("$2a$10$notarealhashnotarealhashnotarealhashnotarealhashno").
			SetStatus(user.StatusActivated).
			Save(bypassCtx)
	})
	require.NoError(t, err)
	return u
}

func newTestUserService(t *testing.T, client *ent.Client) *UserService {
	t.Helper()
	cacheConfig := xcache.Config{Mode: "memory"}
	systemService := &SystemService{
		AbstractService: &AbstractService{db: client},
		CacheConfig:     cacheConfig,
		Cache:           xcache.NewFromConfig[ent.System](cacheConfig),
	}
	return &UserService{
		AbstractService: &AbstractService{db: client},
		UserCache:       xcache.NewFromConfig[ent.User](cacheConfig),
		systemService:   systemService,
	}
}

func TestResetPasswordTokenRoundTripOnPublicContext(t *testing.T) {
	client, ctx := publicRequestContext(t)
	defer client.Close()

	emailTokenService := &EmailTokenService{AbstractService: &AbstractService{db: client}}
	userService := newTestUserService(t, client)

	u := seedActivatedUser(t, ctx, client, "reset@example.com")

	token, err := emailTokenService.CreateToken(ctx, u.ID, emailtoken.TypeResetPassword)
	require.NoError(t, err)

	userID, err := emailTokenService.ValidateToken(ctx, token, emailtoken.TypeResetPassword)
	require.NoError(t, err)
	require.Equal(t, u.ID, userID)

	// The regression: ResetPassword used to run without a privacy bypass, so the
	// User mutation was denied and the token had already been consumed.
	require.NoError(t, userService.ResetPassword(ctx, userID, "NewPassword123"))

	persisted, err := authz.RunWithSystemBypass(ctx, "test-read-user", func(bypassCtx context.Context) (*ent.User, error) {
		return client.User.Get(bypassCtx, userID)
	})
	require.NoError(t, err)
	require.NotEqual(t, "$2a$10$notarealhashnotarealhashnotarealhashnotarealhashno", persisted.Password)

	// The handler consumes the token after the password has been changed; from
	// then on the token is single-use.
	require.NoError(t, emailTokenService.ConsumeToken(ctx, token))
	_, err = emailTokenService.ValidateToken(ctx, token, emailtoken.TypeResetPassword)
	require.Error(t, err)
}

func TestMarkEmailVerifiedAndActivateUserOnPublicContext(t *testing.T) {
	client, ctx := publicRequestContext(t)
	defer client.Close()

	userService := newTestUserService(t, client)

	u, err := authz.RunWithSystemBypass(ctx, "test-seed-pending-user", func(bypassCtx context.Context) (*ent.User, error) {
		return client.User.Create().
			SetEmail("pending@example.com").
			SetFirstName("Pending").
			SetPassword("$2a$10$notarealhashnotarealhashnotarealhashnotarealhashno").
			SetStatus(user.StatusPending).
			Save(bypassCtx)
	})
	require.NoError(t, err)

	require.NoError(t, userService.MarkEmailVerified(ctx, u.ID))
	require.NoError(t, userService.ActivateUser(ctx, u.ID))

	persisted, err := authz.RunWithSystemBypass(ctx, "test-read-user", func(bypassCtx context.Context) (*ent.User, error) {
		return client.User.Get(bypassCtx, u.ID)
	})
	require.NoError(t, err)
	require.NotNil(t, persisted.EmailVerifiedAt)
	require.Equal(t, user.StatusActivated, persisted.Status)
	require.NotEmpty(t, persisted.Scopes)
}

func TestResetPasswordDoesNotBurnTokenWhenResetFails(t *testing.T) {
	client, ctx := publicRequestContext(t)
	defer client.Close()

	emailTokenService := &EmailTokenService{AbstractService: &AbstractService{db: client}}
	userService := newTestUserService(t, client)

	u := seedActivatedUser(t, ctx, client, "stuck@example.com")

	token, err := emailTokenService.CreateToken(ctx, u.ID, emailtoken.TypeResetPassword)
	require.NoError(t, err)

	// A password bcrypt refuses to hash stands in for any failure inside the
	// reset step: nothing is written and the handler must not have burned the
	// token yet.
	require.Error(t, userService.ResetPassword(ctx, u.ID, strings.Repeat("x", 73)))

	// The token must survive so the user can retry instead of being told the
	// link is invalid forever.
	userID, err := emailTokenService.ValidateToken(ctx, token, emailtoken.TypeResetPassword)
	require.NoError(t, err)
	require.Equal(t, u.ID, userID)

	require.NoError(t, userService.ResetPassword(ctx, userID, "NewPassword123"))
	require.NoError(t, emailTokenService.ConsumeToken(ctx, token))
	_, err = emailTokenService.ValidateToken(ctx, token, emailtoken.TypeResetPassword)
	require.Error(t, err)
}

func TestEmailVerifiedAtIsUTC(t *testing.T) {
	client, ctx := publicRequestContext(t)
	defer client.Close()

	userService := newTestUserService(t, client)
	u := seedActivatedUser(t, ctx, client, "utc@example.com")

	require.NoError(t, userService.MarkEmailVerified(ctx, u.ID))

	persisted, err := authz.RunWithSystemBypass(ctx, "test-read-user", func(bypassCtx context.Context) (*ent.User, error) {
		return client.User.Get(bypassCtx, u.ID)
	})
	require.NoError(t, err)
	require.NotNil(t, persisted.EmailVerifiedAt)
	require.Equal(t, time.UTC, persisted.EmailVerifiedAt.Location())
}
