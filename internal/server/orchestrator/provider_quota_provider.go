package orchestrator

import (
	"context"

	"github.com/ldm2060/axonhub/internal/server/biz"
)

// ProviderQuotaStatusProvider provides quota status information for channels.
type ProviderQuotaStatusProvider interface {
	GetQuotaStatus(ctx context.Context, channelID int) *biz.QuotaChannelStatus

	// HasActiveBindings reports whether the channel currently has effective
	// quota_monitor_bindings rows (the binding path). Channels with effective
	// bindings are enforced solely via QuotaBindingReady; the independent
	// quotaStatus exhaustion filter is the fallback for channels without bindings.
	HasActiveBindings(ctx context.Context, channelID int) bool
}
