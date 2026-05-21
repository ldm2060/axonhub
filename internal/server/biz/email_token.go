package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/emailtoken"
)

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

// CreateToken generates a UUID token, stores it with a 24-hour expiry, and returns the token string.
func (s *EmailTokenService) CreateToken(ctx context.Context, userID int, tokenType emailtoken.Type) (string, error) {
	token := uuid.New().String()
	_, err := s.entFromContext(ctx).EmailToken.Create().
		SetToken(token).
		SetType(tokenType).
		SetExpiresAt(time.Now().Add(24*time.Hour)).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		return "", fmt.Errorf("create email token: %w", err)
	}
	return token, nil
}

// ValidateToken checks that the token exists, matches the given type, has not expired, and has not been consumed.
// Returns the associated userID on success.
func (s *EmailTokenService) ValidateToken(ctx context.Context, token string, tokenType emailtoken.Type) (int, error) {
	t, err := s.entFromContext(ctx).EmailToken.Query().
		Where(
			emailtoken.Token(token),
			emailtoken.TypeEQ(tokenType),
			emailtoken.ExpiresAtGT(time.Now()),
			emailtoken.ConsumedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		return 0, fmt.Errorf("validate email token: %w", err)
	}
	return t.UserID, nil
}

// ConsumeToken marks a token as consumed by setting consumed_at to the current time.
func (s *EmailTokenService) ConsumeToken(ctx context.Context, token string) error {
	_, err := s.entFromContext(ctx).EmailToken.Update().
		Where(emailtoken.Token(token)).
		SetConsumedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("consume email token: %w", err)
	}
	return nil
}

// CleanupExpired deletes all email tokens that have passed their expiry time.
func (s *EmailTokenService) CleanupExpired(ctx context.Context) error {
	_, err := s.entFromContext(ctx).EmailToken.Delete().
		Where(emailtoken.ExpiresAtLT(time.Now())).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("cleanup expired email tokens: %w", err)
	}
	return nil
}
