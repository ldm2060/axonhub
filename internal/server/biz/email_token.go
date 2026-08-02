package biz

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/pkg/xtime"
)

var errEmailCodeUnchanged = errors.New("email code unchanged")

// EmailTokenServiceParams holds the dependencies for EmailTokenService.
type EmailTokenServiceParams struct {
	fx.In

	Ent *ent.Client
}

// EmailTokenService handles creation, validation, and consumption of email tokens
// used for email verification and password reset flows.
type EmailTokenService struct {
	*AbstractService
}

// NewEmailTokenService creates a new EmailTokenService.
func NewEmailTokenService(params EmailTokenServiceParams) *EmailTokenService {
	return &EmailTokenService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func generateEmailCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate email code: %w", err)
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

// CreateToken generates a UUID token, stores it with a 24-hour expiry, and returns the token string.
func (s *EmailTokenService) CreateToken(ctx context.Context, userID int, tokenType emailtoken.Type) (string, error) {
	token := uuid.New().String()

	err := authz.RunWithSystemBypassVoid(ctx, "email-token-create", func(bypassCtx context.Context) error {
		_, err := s.entFromContext(bypassCtx).EmailToken.Create().
			SetToken(token).
			SetType(tokenType).
			SetExpiresAt(xtime.UTCNow().Add(24 * time.Hour)).
			SetUserID(userID).
			Save(bypassCtx)

		return err
	})
	if err != nil {
		return "", fmt.Errorf("create email token: %w", err)
	}

	return token, nil
}

// ValidateToken checks that the token exists, matches the given type, has not expired, and has not been consumed.
// Returns the associated userID on success.
func (s *EmailTokenService) ValidateToken(ctx context.Context, token string, tokenType emailtoken.Type) (int, error) {
	t, err := authz.RunWithSystemBypass(ctx, "email-token-validate", func(bypassCtx context.Context) (*ent.EmailToken, error) {
		return s.entFromContext(bypassCtx).EmailToken.Query().
			Where(
				emailtoken.Token(token),
				emailtoken.TypeEQ(tokenType),
				emailtoken.ExpiresAtGT(xtime.UTCNow()),
				emailtoken.ConsumedAtIsNil(),
				emailtoken.UserIDNotNil(),
			).
			Only(bypassCtx)
	})
	if err != nil {
		return 0, fmt.Errorf("validate email token: %w", err)
	}

	return *t.UserID, nil
}

func (s *EmailTokenService) CreateEmailCode(ctx context.Context, email string, tokenType emailtoken.Type, ttl time.Duration) (string, error) {
	normalizedEmail := normalizeEmail(email)

	var lastErr error
	for range 5 {
		code, err := generateEmailCode()
		if err != nil {
			return "", err
		}
		err = s.saveEmailCode(ctx, normalizedEmail, tokenType, code, ttl)
		if err == nil {
			return code, nil
		}
		if !ent.IsConstraintError(err) && !errors.Is(err, errEmailCodeUnchanged) {
			return "", err
		}
		lastErr = err
	}

	return "", fmt.Errorf("create email code: %w", lastErr)
}

func (s *EmailTokenService) saveEmailCode(ctx context.Context, normalizedEmail string, tokenType emailtoken.Type, code string, ttl time.Duration) error {
	expiresAt := xtime.UTCNow().Add(ttl)

	return authz.RunWithSystemBypassVoid(ctx, "email-code-save", func(bypassCtx context.Context) error {
		client := s.entFromContext(bypassCtx)

		updated, err := client.EmailToken.Update().
			Where(
				emailtoken.TypeEQ(tokenType),
				emailtoken.EmailEQ(normalizedEmail),
				emailtoken.TokenNEQ(code),
			).
			SetToken(code).
			SetExpiresAt(expiresAt).
			ClearConsumedAt().
			Save(bypassCtx)
		if err != nil {
			return fmt.Errorf("update email code: %w", err)
		}

		if updated > 0 {
			return nil
		}

		exists, err := client.EmailToken.Query().
			Where(
				emailtoken.TypeEQ(tokenType),
				emailtoken.EmailEQ(normalizedEmail),
				emailtoken.Token(code),
			).
			Exist(bypassCtx)
		if err != nil {
			return fmt.Errorf("check existing email code: %w", err)
		}

		if exists {
			return errEmailCodeUnchanged
		}

		_, err = client.EmailToken.Create().
			SetToken(code).
			SetType(tokenType).
			SetEmail(normalizedEmail).
			SetExpiresAt(expiresAt).
			Save(bypassCtx)
		if err != nil {
			return fmt.Errorf("create email code row: %w", err)
		}

		return nil
	})
}

func (s *EmailTokenService) ValidateEmailCode(ctx context.Context, email, code string, tokenType emailtoken.Type) (*ent.EmailToken, error) {
	t, err := authz.RunWithSystemBypass(ctx, "email-code-validate", func(bypassCtx context.Context) (*ent.EmailToken, error) {
		return s.entFromContext(bypassCtx).EmailToken.Query().
			Where(
				emailtoken.Token(code),
				emailtoken.TypeEQ(tokenType),
				emailtoken.EmailEQ(normalizeEmail(email)),
				emailtoken.ExpiresAtGT(xtime.UTCNow()),
				emailtoken.ConsumedAtIsNil(),
			).
			Only(bypassCtx)
	})
	if err != nil {
		return nil, fmt.Errorf("validate email code: %w", err)
	}

	return t, nil
}

func (s *EmailTokenService) ConsumeEmailCode(ctx context.Context, email, code string, tokenType emailtoken.Type) error {
	updated, err := authz.RunWithSystemBypass(ctx, "email-code-consume", func(bypassCtx context.Context) (int, error) {
		return s.entFromContext(bypassCtx).EmailToken.Update().
			Where(
				emailtoken.Token(code),
				emailtoken.TypeEQ(tokenType),
				emailtoken.EmailEQ(normalizeEmail(email)),
				emailtoken.ExpiresAtGT(xtime.UTCNow()),
				emailtoken.ConsumedAtIsNil(),
			).
			SetConsumedAt(xtime.UTCNow()).
			Save(bypassCtx)
	})
	if err != nil {
		return fmt.Errorf("consume email code: %w", err)
	}

	if updated != 1 {
		return fmt.Errorf("consume email code: token not found")
	}

	return nil
}

// ConsumeToken marks a token as consumed by setting consumed_at to the current time.
func (s *EmailTokenService) ConsumeToken(ctx context.Context, token string) error {
	err := authz.RunWithSystemBypassVoid(ctx, "email-token-consume", func(bypassCtx context.Context) error {
		_, err := s.entFromContext(bypassCtx).EmailToken.Update().
			Where(emailtoken.Token(token)).
			SetConsumedAt(xtime.UTCNow()).
			Save(bypassCtx)

		return err
	})
	if err != nil {
		return fmt.Errorf("consume email token: %w", err)
	}

	return nil
}

// CleanupExpired deletes all email tokens that have passed their expiry time.
func (s *EmailTokenService) CleanupExpired(ctx context.Context) error {
	err := authz.RunWithSystemBypassVoid(ctx, "email-token-cleanup", func(bypassCtx context.Context) error {
		_, err := s.entFromContext(bypassCtx).EmailToken.Delete().
			Where(emailtoken.ExpiresAtLT(xtime.UTCNow())).
			Exec(bypassCtx)

		return err
	})
	if err != nil {
		return fmt.Errorf("cleanup expired email tokens: %w", err)
	}

	return nil
}
