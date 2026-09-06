package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/providerquotastatus"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
)

func TestProviderQuotaSelector_FiltersExhaustedZenMuxNativeVideoChannel(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {ProviderType: "zenmux", Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {ProviderType: "zenmux", Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{
				Channel: &biz.Channel{Channel: &ent.Channel{
					ID:                1,
					Name:              "zenmux-native-video",
					Type:              channel.TypeZenmux,
					QuotaBindingReady: true,
					Endpoints:         []objects.ChannelEndpoint{{APIFormat: llm.APIFormatZenmuxVideo.String()}},
				}},
				APIFormat: llm.APIFormatZenmuxVideo.String(),
			},
			{
				Channel: &biz.Channel{Channel: &ent.Channel{
					ID:                2,
					Name:              "zenmux-openai-video",
					Type:              channel.TypeZenmux,
					QuotaBindingReady: true,
				}},
				APIFormat: llm.APIFormatOpenAIVideo.String(),
			},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	result, err := selector.Select(context.Background(), &llm.Request{
		Model:       "video-model",
		RequestType: llm.RequestTypeVideo,
		Video:       &llm.VideoRequest{Model: "video-model"},
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, 2, result[0].Channel.ID, "exhausted ZenMux quota must filter the native video candidate")
}
