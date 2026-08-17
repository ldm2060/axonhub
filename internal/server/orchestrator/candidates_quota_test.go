package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/providerquotastatus"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/internal/server/biz/provider_quota"
	"github.com/ldm2060/axonhub/llm"
)

func TestProviderQuotaSelector_ExhaustedOnlyMode(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusWarning, Ready: true},
			3: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "warning", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 2, got[0].Channel.ID)
	require.Equal(t, 3, got[1].Channel.ID)
}

func TestProviderQuotaSelector_DePrioritizeMode(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusWarning, Ready: true},
			3: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeDePrioritize},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "warning", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 3)

	ids := make([]int, len(got))
	for i, c := range got {
		ids[i] = c.Channel.ID
	}
	require.ElementsMatch(t, []int{1, 2, 3}, ids)
}

func TestProviderQuotaSelector_EnforcementDisabled(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_AllExhausted(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "c1", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "c2", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestProviderQuotaSelector_NoQuotaData(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "no-data", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_NilProvider(t *testing.T) {
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, nil, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_WrappedError(t *testing.T) {
	provider := &mockQuotaStatusProvider{}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{err: errors.New("inner error")}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	_, err := selector.Select(context.Background(), &llm.Request{})

	require.Error(t, err)
	require.Equal(t, "inner error", err.Error())
}

func TestProviderQuotaSelector_EmptyCandidates(t *testing.T) {
	provider := &mockQuotaStatusProvider{}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{candidates: []*ChannelModelsCandidate{}}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestProviderQuotaSelector_UnknownStatusKept(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusUnknown, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "unknown", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_MixedCandidates(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusWarning, Ready: true},
			3: {Status: providerquotastatus.StatusAvailable, Ready: true},
			4: {Status: providerquotastatus.StatusUnknown, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeDePrioritize},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "warning", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 4, Name: "unknown", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 5, Name: "no-data", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 5)

	ids := make([]int, len(got))
	for i, c := range got {
		ids[i] = c.Channel.ID
	}
	require.ElementsMatch(t, []int{1, 2, 3, 4, 5}, ids)
}

func TestProviderQuotaSelector_PerLimit_ImageExhausted_KeptForToken(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {
				Status: providerquotastatus.StatusWarning,
				Ready:  true,
				Limits: []provider_quota.QuotaLimitStatus{
					{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1.0, Ready: false},
					{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
				},
			},
		},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1", QuotaBindingReady: true}}},
		},
	}

	systemService := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	selector := WithProviderQuotaSelector(inner, provider, systemService)

	tokenReq := &llm.Request{Model: "gpt-4"}
	result, err := selector.Select(context.Background(), tokenReq)
	require.NoError(t, err)
	require.Len(t, result, 1, "channel should be kept for token request when only image limit is exhausted")

	imageReq := &llm.Request{Model: "dall-e-3", Image: &llm.ImageRequest{}}
	result, err = selector.Select(context.Background(), imageReq)
	require.NoError(t, err)
	require.Len(t, result, 0, "channel should be filtered for image request when image limit is exhausted")
}

func TestProviderQuotaSelector_FiltersExhaustedBeforeLoadBalancer(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusExhausted, Ready: false},
			3: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted-1", QuotaBindingReady: true}}, Priority: 0},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "exhausted-2", QuotaBindingReady: true}}, Priority: 0},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available", QuotaBindingReady: true}}, Priority: 1},
		},
	}

	quotaSelector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := quotaSelector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 3, got[0].Channel.ID, "available channel should be preserved after quota filtering")
}

func TestProviderQuotaSelector_ChannelExhaustedOverridesPerLimitAvailable(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {
				Status: providerquotastatus.StatusExhausted,
				Ready:  false,
				Limits: []provider_quota.QuotaLimitStatus{
					{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.3, Ready: true},
				},
			},
		},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1", QuotaBindingReady: true}}},
		},
	}

	systemService := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	selector := WithProviderQuotaSelector(inner, provider, systemService)

	tokenReq := &llm.Request{Model: "gpt-4"}
	result, err := selector.Select(context.Background(), tokenReq)
	require.NoError(t, err)
	require.Empty(t, result, "channel with Exhausted channel-level status must be filtered even if per-limit token status is available")
}

func TestProviderQuotaSelector_WithQuotaBinding(t *testing.T) {
	t.Run("filters out quota_binding_ready=false channels", func(t *testing.T) {
		provider := &mockQuotaStatusProvider{
			statuses: map[int]*biz.QuotaChannelStatus{
				1: {Status: providerquotastatus.StatusAvailable, Ready: true},
				2: {Status: providerquotastatus.StatusAvailable, Ready: true},
			},
		}
		settings := &mockQuotaEnforcementSettingsProvider{
			settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
		}

		inner := &mockSelector{
			candidates: []*ChannelModelsCandidate{
				{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ready", QuotaBindingReady: true}}},
				{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "not-ready", QuotaBindingReady: false}}},
			},
		}

		selector := WithProviderQuotaSelector(inner, provider, settings)
		got, err := selector.Select(context.Background(), &llm.Request{})

		require.NoError(t, err)
		require.Len(t, got, 1, "should only return quota_binding_ready=true channel")
		require.Equal(t, 1, got[0].Channel.ID)
		require.Equal(t, "ready", got[0].Channel.Name)
	})

	t.Run("keeps both channels when both have quota_binding_ready=true", func(t *testing.T) {
		provider := &mockQuotaStatusProvider{
			statuses: map[int]*biz.QuotaChannelStatus{
				1: {Status: providerquotastatus.StatusAvailable, Ready: true},
				2: {Status: providerquotastatus.StatusAvailable, Ready: true},
			},
		}
		settings := &mockQuotaEnforcementSettingsProvider{
			settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
		}

		inner := &mockSelector{
			candidates: []*ChannelModelsCandidate{
				{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ready1", QuotaBindingReady: true}}},
				{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ready2", QuotaBindingReady: true}}},
			},
		}

		selector := WithProviderQuotaSelector(inner, provider, settings)
		got, err := selector.Select(context.Background(), &llm.Request{})

		require.NoError(t, err)
		require.Len(t, got, 2, "should return both quota_binding_ready=true channels")
	})

	t.Run("enforcement disabled - still filters by quota_binding_ready", func(t *testing.T) {
		provider := &mockQuotaStatusProvider{
			statuses: map[int]*biz.QuotaChannelStatus{
				1: {Status: providerquotastatus.StatusAvailable, Ready: true},
				2: {Status: providerquotastatus.StatusAvailable, Ready: true},
			},
		}
		settings := &mockQuotaEnforcementSettingsProvider{
			settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: biz.QuotaEnforcementModeExhaustedOnly},
		}

		inner := &mockSelector{
			candidates: []*ChannelModelsCandidate{
				{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ready", QuotaBindingReady: true}}},
				{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "not-ready", QuotaBindingReady: false}}},
			},
		}

		selector := WithProviderQuotaSelector(inner, provider, settings)
		got, err := selector.Select(context.Background(), &llm.Request{})

		require.NoError(t, err)
		require.Len(t, got, 1, "should filter by quota_binding_ready even when enforcement is disabled")
		require.Equal(t, 1, got[0].Channel.ID)
	})
}

func TestProviderQuotaSelector_AllowedChannelIDs_ExemptFromFiltering(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusExhausted, Ready: false},
			3: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{
			Enabled:           true,
			Mode:              biz.QuotaEnforcementModeExhaustedOnly,
			AllowedChannelIDs: []int{1},
		},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exempt-exhausted", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "filtered-exhausted", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 2, "exempt channel should be kept, non-exempt exhausted filtered, available kept")
	require.Equal(t, 1, got[0].Channel.ID, "exempt channel must be preserved")
	require.Equal(t, 3, got[1].Channel.ID, "available channel must be preserved")
}

func TestProviderQuotaSelector_AllowedChannelIDs_EmptyList(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{
			Enabled:           true,
			Mode:              biz.QuotaEnforcementModeExhaustedOnly,
			AllowedChannelIDs: []int{},
		},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Empty(t, got, "empty allowed list should not exempt any channel")
}

func TestProviderQuotaSelector_AllowedChannelIDs_NilList(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{
			Enabled: true,
			Mode:    biz.QuotaEnforcementModeExhaustedOnly,
		},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Empty(t, got, "nil allowed list should not exempt any channel")
}

func TestProviderQuotaSelector_AllowedChannelIDs_MultipleExempt(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusExhausted, Ready: false},
			3: {Status: providerquotastatus.StatusExhausted, Ready: false},
			4: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{
			Enabled:           true,
			Mode:              biz.QuotaEnforcementModeExhaustedOnly,
			AllowedChannelIDs: []int{1, 3},
		},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exempt-1", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "filtered", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "exempt-2", QuotaBindingReady: true}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 4, Name: "available", QuotaBindingReady: true}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 3, "two exempt + one available should remain")

	ids := make([]int, len(got))
	for i, c := range got {
		ids[i] = c.Channel.ID
	}
	require.ElementsMatch(t, []int{1, 3, 4}, ids)
}
