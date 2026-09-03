package responses

import (
	"github.com/ldm2060/axonhub/llm"
)

type Usage struct {
	InputTokens       int64 `json:"input_tokens"`
	InputTokenDetails struct {
		// CacheWriteTokens is the number of input tokens written to the prompt cache.
		CacheWriteTokens int64 `json:"cache_write_tokens"`
		// CachedTokens is the number of input tokens retrieved from the prompt cache.
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens       int64 `json:"output_tokens"`
	OutputTokenDetails struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int64 `json:"total_tokens"`

	// Top-level cache fields emitted by some providers/middlewares as fallbacks
	// when the official nested detail fields are absent. Unset values are omitted.
	CachedTokens             int64 `json:"cached_tokens,omitempty"`
	CacheReadTokens          int64 `json:"cache_read_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	CacheCreationTokens      int64 `json:"cache_creation_tokens,omitempty"`
	CacheWriteTokens         int64 `json:"cache_write_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheWriteInputTokens    int64 `json:"cache_write_input_tokens,omitempty"`

	// Cost is the request cost calculated by AxonHub from channel model prices.
	// Omitted when no matching price is configured.
	Cost *float64 `json:"cost,omitempty"`
}

func firstNonZero(values ...int64) int64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}

	return 0
}

func (u *Usage) ToUsage() *llm.Usage {
	if u == nil {
		return nil
	}

	// Resolve cache-read/write with official-field-first precedence, falling back
	// to provider/middleware alias field names. input_tokens already includes
	// cached tokens, so these are recorded in details only (never added to the
	// prompt total) — matching how ComputeUsageCost subtracts them.
	cachedRead := firstNonZero(
		u.InputTokenDetails.CachedTokens, // official nested
		u.CachedTokens,                   // top-level aliases
		u.CacheReadTokens,
		u.CacheReadInputTokens,
	)
	writeCached := firstNonZero(
		u.InputTokenDetails.CacheWriteTokens, // official nested
		u.CacheCreationInputTokens,           // top-level aliases
		u.CacheWriteInputTokens,
		u.CacheCreationTokens,
		u.CacheWriteTokens,
	)

	return &llm.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens:      cachedRead,
			WriteCachedTokens: writeCached,
		},
		CompletionTokensDetails: &llm.CompletionTokensDetails{
			ReasoningTokens: u.OutputTokenDetails.ReasoningTokens,
		},
	}
}

// ConvertLLMUsageToResponsesUsage converts llm.Usage to Responses API Usage.
func ConvertLLMUsageToResponsesUsage(usage *llm.Usage) *Usage {
	if usage == nil {
		return nil
	}

	result := &Usage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
		Cost:         usage.Cost,
	}

	if usage.PromptTokensDetails != nil {
		result.InputTokenDetails.CachedTokens = usage.PromptTokensDetails.CachedTokens
		result.InputTokenDetails.CacheWriteTokens = usage.PromptTokensDetails.WriteCachedTokens
	}

	if usage.CompletionTokensDetails != nil {
		result.OutputTokenDetails.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
	}

	return result
}
