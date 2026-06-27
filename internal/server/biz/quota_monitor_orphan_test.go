package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
)

func saveTestBinding(t *testing.T, svc *UsageMonitorService, ctx context.Context, chID, monID int) {
	t.Helper()
	require.NoError(t, svc.SaveChannelQuotaMonitorBindings(ctx, chID, SaveChannelQuotaMonitorBindingsInput{
		Strategy: "any",
		Bindings: []SaveChannelQuotaMonitorBindingInput{
			{UsageMonitorChannelID: monID, Enabled: true, TriggerStatuses: []string{"exhausted"}},
		},
	}))
}

func activeBindingCount(t *testing.T, svc *UsageMonitorService, ctx context.Context) int {
	t.Helper()
	n, err := svc.db.ChannelUsageMonitorBinding.Query().
		Where(channelusagemonitorbinding.DeletedAtEQ(0)).
		Count(ctx)
	require.NoError(t, err)
	return n
}

// TestSoftDeleteBindingsForMonitor verifies the helper used when a usage
// monitor is deleted: it must soft-delete the bindings that reference that
// monitor, so no orphan bindings remain.
func TestSoftDeleteBindingsForMonitor(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")
	any := channel.QuotaMultiMonitorStrategyAny
	ch := createTestChannelForBinding(t, svc, ctx, "C", &any)
	mon := createTestMonitorForBinding(t, svc, ctx, "M", usagemonitorchannel.QuotaStatusAvailable, nil)
	saveTestBinding(t, svc, ctx, ch.ID, mon.ID)
	require.Equal(t, 1, activeBindingCount(t, svc, ctx))

	require.NoError(t, svc.softDeleteBindingsForMonitor(ctx, mon.ID))

	assert.Equal(t, 0, activeBindingCount(t, svc, ctx), "soft-deleting a monitor's bindings must remove them")
}

// TestSoftDeleteBindingsForChannel verifies the helper used when a channel is
// hard-deleted: it must soft-delete that channel's quota monitor bindings.
func TestSoftDeleteBindingsForChannel(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")
	any := channel.QuotaMultiMonitorStrategyAny
	ch := createTestChannelForBinding(t, svc, ctx, "C", &any)
	mon := createTestMonitorForBinding(t, svc, ctx, "M", usagemonitorchannel.QuotaStatusAvailable, nil)
	saveTestBinding(t, svc, ctx, ch.ID, mon.ID)
	require.Equal(t, 1, activeBindingCount(t, svc, ctx))

	require.NoError(t, svc.SoftDeleteBindingsForChannel(ctx, ch.ID))

	assert.Equal(t, 0, activeBindingCount(t, svc, ctx), "soft-deleting a channel's bindings must remove them")
}

// TestListBindingSummaries_DefaultsStrategyWhenChannelMissing verifies the
// defense-in-depth: even if an orphan binding (missing channel) exists, the
// summary's strategy must default to a valid value instead of empty, so the
// frontend zod schema (z.enum(['any','all'])) never rejects it.
func TestListBindingSummaries_DefaultsStrategyWhenChannelMissing(t *testing.T) {
	svc, ctx := setupTestBindingService(t, "any")
	any := channel.QuotaMultiMonitorStrategyAny
	ch := createTestChannelForBinding(t, svc, ctx, "C", &any)
	mon := createTestMonitorForBinding(t, svc, ctx, "M", usagemonitorchannel.QuotaStatusAvailable, nil)
	saveTestBinding(t, svc, ctx, ch.ID, mon.ID)

	// Simulate the pre-fix bug: channel hard-deleted while its binding remains.
	require.NoError(t, svc.db.Channel.DeleteOneID(ch.ID).Exec(ctx))

	summaries, err := svc.ListUsageMonitorBindingSummaries(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "any", summaries[0].Strategy, "strategy must default to 'any' even when the channel edge is missing")
}
