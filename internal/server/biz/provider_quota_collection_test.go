package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

func setupProviderQuotaCollectionService(t *testing.T) (*ProviderQuotaService, *SystemService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := NewSystemService(SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})
	service := &ProviderQuotaService{
		AbstractService: &AbstractService{db: client},
		SystemService:   systemService,
	}

	return service, systemService, client
}

func TestProviderQuotaService_GetQuotaStatus_CollectionDisabledForProvider(t *testing.T) {
	service, systemService, client := setupProviderQuotaCollectionService(t)
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
	service, systemService, client := setupProviderQuotaCollectionService(t)
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
	service, systemService, client := setupProviderQuotaCollectionService(t)
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
