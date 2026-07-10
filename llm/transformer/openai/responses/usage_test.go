package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
)

// TestUsage_ToUsage_CacheParsing verifies that cache-read and cache-write token
// counts are parsed from the official Responses detail fields and, when absent,
// from alias field names. The official nested field always takes precedence.
func TestUsage_ToUsage_CacheParsing(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		read    int64
		write   int64
	}{
		{
			name:    "official nested cache_write_tokens",
			payload: `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"input_tokens_details":{"cache_write_tokens":7}}`,
			write:   7,
		},
		{
			name:    "alias cache_creation_input_tokens",
			payload: `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"cache_creation_input_tokens":9}`,
			write:   9,
		},
		{
			name:    "alias cache_write_tokens",
			payload: `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"cache_write_tokens":15}`,
			write:   15,
		},
		{
			name:    "official write beats aliases",
			payload: `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"input_tokens_details":{"cache_write_tokens":21},"cache_creation_input_tokens":99,"cache_write_tokens":88}`,
			write:   21,
		},
		{
			name:    "read alias cache_read_tokens",
			payload: `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"cache_read_tokens":5}`,
			read:    5,
		},
		{
			name:    "official read cached_tokens beats alias",
			payload: `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"input_tokens_details":{"cached_tokens":3},"cache_read_tokens":70}`,
			read:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u Usage
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &u))

			llmUsage := u.ToUsage()
			require.NotNil(t, llmUsage.PromptTokensDetails)
			require.Equal(t, tt.read, llmUsage.PromptTokensDetails.CachedTokens, "cache-read mismatch")
			require.Equal(t, tt.write, llmUsage.PromptTokensDetails.WriteCachedTokens, "cache-write mismatch")
		})
	}
}

// TestConvertLLMUsageToResponsesUsage_CacheWrite verifies cache-write is carried
// through unified → Responses conversion (the protocol-conversion loss fix).
func TestConvertLLMUsageToResponsesUsage_CacheWrite(t *testing.T) {
	src := &llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens:      20,
			WriteCachedTokens: 7,
		},
	}

	got := ConvertLLMUsageToResponsesUsage(src)
	require.NotNil(t, got)
	require.Equal(t, int64(20), got.InputTokenDetails.CachedTokens)
	require.Equal(t, int64(7), got.InputTokenDetails.CacheWriteTokens)
}

// TestUsage_CacheWriteRoundTrip verifies cache-write survives a full
// Responses → unified → Responses round trip.
func TestUsage_CacheWriteRoundTrip(t *testing.T) {
	src := &Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}
	src.InputTokenDetails.CachedTokens = 20
	src.InputTokenDetails.CacheWriteTokens = 7

	back := ConvertLLMUsageToResponsesUsage(src.ToUsage())
	require.Equal(t, int64(20), back.InputTokenDetails.CachedTokens)
	require.Equal(t, int64(7), back.InputTokenDetails.CacheWriteTokens)
}

// TestUsage_MarshalOmitsZeroAliases ensures unset alias fields are omitted from
// the wire format so conversion output stays clean.
func TestUsage_MarshalOmitsZeroAliases(t *testing.T) {
	u := ConvertLLMUsageToResponsesUsage(&llm.Usage{
		PromptTokens: 100,
		TotalTokens:  100,
	})

	data, err := json.Marshal(u)
	require.NoError(t, err)

	// cache alias fields must not appear when unset
	require.NotContains(t, string(data), "cache_creation_input_tokens")
	require.NotContains(t, string(data), "cache_read_tokens")
}
