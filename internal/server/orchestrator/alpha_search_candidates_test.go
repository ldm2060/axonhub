package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
)

func TestPopulateAPIFormatFiltersAlphaSearchUnsupportedChannels(t *testing.T) {
	unsupported := &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{Type: channel.TypeOpenai}}}
	supported := &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{Type: channel.TypeCodex}}}

	candidates := populateAPIFormat([]*ChannelModelsCandidate{unsupported, supported}, &llm.Request{
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
	})

	require.Equal(t, []*ChannelModelsCandidate{supported}, candidates)
	require.Equal(t, llm.APIFormatOpenAIAlphaSearch.String(), supported.APIFormat)
}
