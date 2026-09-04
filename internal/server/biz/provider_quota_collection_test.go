package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
)

func setupProviderQuotaCollectionService(t *testing.T) (*ProviderQuotaService, *SystemService, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := NewSystemService(SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	service := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		SystemService:   systemService,
	}

	return service, systemService, ctx, client
}

func createProviderQuotaCollectionChannel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	name string,
	channelType channel.Type,
) *ent.Channel {
	t.Helper()

	result, err := client.Channel.Create().
		SetName(name).
		SetType(channelType).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		Save(ctx)
	require.NoError(t, err)
	return result
}

func TestProviderQuotaService_GetQuotaStatus_CollectionDisabledForProvider(t *testing.T) {
	service, systemService, _, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	service.quotaCache.Store(1, &QuotaChannelStatus{
		ProviderType: "minimax",
		Status:       "unknown",
		Ready:        false,
	})
	service.quotaCache.Store(2, &QuotaChannelStatus{
		ProviderType: "codex",
		Status:       "available",
		Ready:        true,
	})
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: false},
	}))

	require.Nil(t, service.GetQuotaStatus(ctx, 1))
	require.NotNil(t, service.GetQuotaStatus(ctx, 2))
}

func TestProviderQuotaService_GetQuotaStatus_CollectionDisabledGlobally(t *testing.T) {
	service, systemService, _, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	service.quotaCache.Store(1, &QuotaChannelStatus{
		ProviderType: "minimax",
		Status:       "available",
		Ready:        true,
	})
	service.quotaCache.Store(2, &QuotaChannelStatus{
		ProviderType: "codex",
		Status:       "available",
		Ready:        true,
	})

	disabled := false
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, &disabled, nil))

	require.Nil(t, service.GetQuotaStatus(ctx, 1))
	require.Nil(t, service.GetQuotaStatus(ctx, 2))
}

func TestProviderQuotaService_GetQuotaStatus_NoProviderTypeBypassesGate(t *testing.T) {
	service, systemService, _, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	// Channels without a provider type (e.g. legacy / builtin-monitored rows)
	// are not gated by the per-provider collection settings.
	service.quotaCache.Store(1, &QuotaChannelStatus{
		ProviderType: "",
		Status:       "available",
		Ready:        true,
	})

	disabled := false
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, &disabled, nil))

	require.NotNil(t, service.GetQuotaStatus(ctx, 1))
}

// Reset capability tests adapted to our architecture: resetCheckerForProvider
// instantiates a real CodexQuotaChecker for "codex" and returns unsupported for
// every other provider type (no svc.checkers map injection).

func TestProviderQuotaService_ListResets_ReturnsUnsupportedForNonCodexProvider(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	// minimax is not wired into resetCheckerForProvider, so ListResets reports
	// the capability as unsupported without erroring.
	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)

	resets, err := service.ListResets(ctx, channelEntity.ID)

	require.NoError(t, err)
	require.False(t, resets.Supported)
	require.Empty(t, resets.Resets)
}

func TestProviderQuotaService_ResetChannelQuotaNow_ReturnsUnsupportedForNonCodexProvider(t *testing.T) {
	service, _, ctx, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "MiniMax", channel.TypeMinimax)

	err := service.ResetChannelQuotaNow(ctx, channelEntity.ID)

	require.Error(t, err)
	require.ErrorIs(t, err, provider_quota.ErrResetUnsupported)
}

func TestProviderQuotaService_ListResets_CollectionDisabledForCodex(t *testing.T) {
	service, systemService, _, client := setupProviderQuotaCollectionService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	channelEntity := createProviderQuotaCollectionChannel(t, ctx, client, "Codex", channel.TypeCodex)
	require.NoError(t, systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "codex", Enabled: false},
	}))

	_, err := service.ListResets(ctx, channelEntity.ID)

	require.ErrorContains(t, err, "provider quota collection is disabled for codex")
}
