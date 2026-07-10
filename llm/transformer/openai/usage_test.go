package openai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/llm"
)

func TestUsage_ToLLMUsage(t *testing.T) {
	tests := []struct {
		name     string
		usage    *Usage
		expected *llm.Usage
	}{
		{
			name:     "nil usage returns nil",
			usage:    nil,
			expected: nil,
		},
		{
			name: "basic usage without cached tokens",
			usage: &Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			expected: &llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		{
			name: "usage with cached tokens and no existing details",
			usage: &Usage{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
				CachedTokens:     5,
			},
			expected: &llm.Usage{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 5,
				},
			},
		},
		{
			name: "usage with cached tokens and existing details - cached tokens not overwritten",
			usage: &Usage{
				PromptTokens:     20,
				CompletionTokens: 30,
				TotalTokens:      50,
				PromptTokensDetails: PromptTokensDetails{
					CachedTokens: 2,
				},
				CachedTokens: 8,
			},
			expected: &llm.Usage{
				PromptTokens:     20,
				CompletionTokens: 30,
				TotalTokens:      50,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 2,
				},
			},
		},
		{
			name: "usage with zero cached tokens",
			usage: &Usage{
				PromptTokens:     12,
				CompletionTokens: 18,
				TotalTokens:      30,
				CachedTokens:     0,
			},
			expected: &llm.Usage{
				PromptTokens:     12,
				CompletionTokens: 18,
				TotalTokens:      30,
			},
		},
		{
			name: "usage with prompt tokens details",
			usage: &Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: PromptTokensDetails{
					AudioTokens:  10,
					CachedTokens: 20,
				},
			},
			expected: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:  10,
					CachedTokens: 20,
				},
			},
		},
		{
			name: "usage with completion tokens details",
			usage: &Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				CompletionTokensDetails: CompletionTokensDetails{
					AudioTokens:              5,
					ReasoningTokens:          10,
					AcceptedPredictionTokens: 3,
					RejectedPredictionTokens: 2,
				},
			},
			expected: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					AudioTokens:              5,
					ReasoningTokens:          10,
					AcceptedPredictionTokens: 3,
					RejectedPredictionTokens: 2,
				},
			},
		},
		{
			name: "usage with all details",
			usage: &Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
				PromptTokensDetails: PromptTokensDetails{
					AudioTokens:  20,
					CachedTokens: 30,
				},
				CompletionTokensDetails: CompletionTokensDetails{
					AudioTokens:              10,
					ReasoningTokens:          20,
					AcceptedPredictionTokens: 5,
					RejectedPredictionTokens: 5,
				},
			},
			expected: &llm.Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:  20,
					CachedTokens: 30,
				},
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					AudioTokens:              10,
					ReasoningTokens:          20,
					AcceptedPredictionTokens: 5,
					RejectedPredictionTokens: 5,
				},
			},
		},
		{
			name: "usage with cached tokens and zero cached tokens in details",
			usage: &Usage{
				PromptTokens:     50,
				CompletionTokens: 30,
				TotalTokens:      80,
				PromptTokensDetails: PromptTokensDetails{
					CachedTokens: 0,
				},
				CachedTokens: 15,
			},
			expected: &llm.Usage{
				PromptTokens:     50,
				CompletionTokens: 30,
				TotalTokens:      80,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 15,
				},
			},
		},
		{
			name: "usage with write cached tokens",
			usage: &Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: PromptTokensDetails{
					AudioTokens:       10,
					CachedTokens:      20,
					WriteCachedTokens: 5,
				},
			},
			expected: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:       10,
					CachedTokens:      20,
					WriteCachedTokens: 5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.usage.ToLLMUsage()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestUsageFromLLM(t *testing.T) {
	tests := []struct {
		name     string
		usage    *llm.Usage
		expected *Usage
	}{
		{
			name:     "nil usage returns nil",
			usage:    nil,
			expected: nil,
		},
		{
			name: "basic usage without details",
			usage: &llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			expected: &Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		{
			name: "usage with prompt tokens details",
			usage: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:  10,
					CachedTokens: 20,
				},
			},
			expected: &Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: PromptTokensDetails{
					AudioTokens:  10,
					CachedTokens: 20,
				},
			},
		},
		{
			name: "usage with completion tokens details",
			usage: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					AudioTokens:              5,
					ReasoningTokens:          10,
					AcceptedPredictionTokens: 3,
					RejectedPredictionTokens: 2,
				},
			},
			expected: &Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				CompletionTokensDetails: CompletionTokensDetails{
					AudioTokens:              5,
					ReasoningTokens:          10,
					AcceptedPredictionTokens: 3,
					RejectedPredictionTokens: 2,
				},
			},
		},
		{
			name: "usage with all details",
			usage: &llm.Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:  20,
					CachedTokens: 30,
				},
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					AudioTokens:              10,
					ReasoningTokens:          20,
					AcceptedPredictionTokens: 5,
					RejectedPredictionTokens: 5,
				},
			},
			expected: &Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
				PromptTokensDetails: PromptTokensDetails{
					AudioTokens:  20,
					CachedTokens: 30,
				},
				CompletionTokensDetails: CompletionTokensDetails{
					AudioTokens:              10,
					ReasoningTokens:          20,
					AcceptedPredictionTokens: 5,
					RejectedPredictionTokens: 5,
				},
			},
		},
		{
			name: "usage with zero cached tokens in details",
			usage: &llm.Usage{
				PromptTokens:     50,
				CompletionTokens: 30,
				TotalTokens:      80,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 0,
				},
			},
			expected: &Usage{
				PromptTokens:     50,
				CompletionTokens: 30,
				TotalTokens:      80,
				PromptTokensDetails: PromptTokensDetails{
					CachedTokens: 0,
				},
			},
		},
		{
			name: "usage with nil prompt tokens details",
			usage: &llm.Usage{
				PromptTokens:     75,
				CompletionTokens: 25,
				TotalTokens:      100,
			},
			expected: &Usage{
				PromptTokens:     75,
				CompletionTokens: 25,
				TotalTokens:      100,
			},
		},
		{
			name: "usage with write cached tokens",
			usage: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:       10,
					CachedTokens:      20,
					WriteCachedTokens: 5,
				},
			},
			expected: &Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: PromptTokensDetails{
					AudioTokens:      10,
					CachedTokens:     20,
					CacheWriteTokens: 5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UsageFromLLM(tt.usage)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestUsage_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		usage *llm.Usage
	}{
		{
			name: "round trip with basic usage",
			usage: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
		{
			name: "round trip with all details",
			usage: &llm.Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
				PromptTokensDetails: &llm.PromptTokensDetails{
					AudioTokens:  20,
					CachedTokens: 30,
				},
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					AudioTokens:              10,
					ReasoningTokens:          20,
					AcceptedPredictionTokens: 5,
					RejectedPredictionTokens: 5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiUsage := UsageFromLLM(tt.usage)
			result := openaiUsage.ToLLMUsage()
			require.Equal(t, tt.usage, result)
		})
	}
}

// TestUsage_CacheTokenParsing verifies that cache-read and cache-write token
// counts are parsed from the official OpenAI detail fields and, when absent,
// from the alias field names emitted by various OpenAI-compatible providers and
// middlewares. The official nested field always takes precedence over aliases.
func TestUsage_CacheTokenParsing(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		read    int64
		write   int64
	}{
		{
			name:    "official nested cache_write_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cache_write_tokens":7}}`,
			write:   7,
		},
		{
			name:    "alias cache_creation_input_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_creation_input_tokens":9}`,
			write:   9,
		},
		{
			name:    "alias cache_write_input_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_write_input_tokens":11}`,
			write:   11,
		},
		{
			name:    "alias cache_creation_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_creation_tokens":13}`,
			write:   13,
		},
		{
			name:    "alias cache_write_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_write_tokens":15}`,
			write:   15,
		},
		{
			name:    "official write beats aliases",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cache_write_tokens":21},"cache_creation_input_tokens":99,"cache_write_tokens":88}`,
			write:   21,
		},
		{
			name:    "read alias cache_read_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_read_tokens":5}`,
			read:    5,
		},
		{
			name:    "read alias cache_read_input_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_read_input_tokens":6}`,
			read:    6,
		},
		{
			name:    "read top-level cached_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cached_tokens":8}`,
			read:    8,
		},
		{
			name:    "official read beats aliases",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cached_tokens":3},"cached_tokens":80,"cache_read_tokens":70}`,
			read:    3,
		},
		{
			name:    "combined read and write from aliases",
			payload: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cache_creation_input_tokens":12,"cache_read_tokens":4}`,
			read:    4,
			write:   12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u Usage
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &u))

			llmUsage := u.ToLLMUsage()
			require.NotNil(t, llmUsage.PromptTokensDetails)
			require.Equal(t, tt.read, llmUsage.PromptTokensDetails.CachedTokens, "cache-read mismatch")
			require.Equal(t, tt.write, llmUsage.PromptTokensDetails.WriteCachedTokens, "cache-write mismatch")
		})
	}
}

// TestUsage_CacheWriteRoundTrip ensures cache-write survives llm.Usage →
// UsageFromLLM → ToLLMUsage, which is what protocol conversion relies on.
func TestUsage_CacheWriteRoundTrip(t *testing.T) {
	src := &llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens:      20,
			WriteCachedTokens: 7,
		},
	}

	result := UsageFromLLM(src).ToLLMUsage()
	require.Equal(t, src, result)
}
