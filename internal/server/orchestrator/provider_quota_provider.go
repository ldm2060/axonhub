package orchestrator

import (
	"context"

	"github.com/ldm2060/axonhub/internal/server/biz"
)

// ProviderQuotaStatusProvider provides quota status information for channels.
type ProviderQuotaStatusProvider interface {
	GetQuotaStatus(ctx context.Context, channelID int) *biz.QuotaChannelStatus
}
