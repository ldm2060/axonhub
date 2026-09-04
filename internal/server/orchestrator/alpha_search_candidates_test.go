//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package orchestrator

import (
	"context"
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

	candidates := populateAPIFormat(context.Background(), []*ChannelModelsCandidate{unsupported, supported}, &llm.Request{
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
	})

	require.Equal(t, []*ChannelModelsCandidate{supported}, candidates)
	require.Equal(t, llm.APIFormatOpenAIAlphaSearch.String(), supported.APIFormat)
}
