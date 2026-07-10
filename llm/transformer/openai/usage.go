package openai

import "github.com/ldm2060/axonhub/llm"

// PromptTokensDetails Breakdown of tokens used in the prompt.
type PromptTokensDetails struct {
	AudioTokens  int64 `json:"audio_tokens"`
	CachedTokens int64 `json:"cached_tokens"`

	// CacheWriteTokens is the official OpenAI prompt cache-write token count
	// (prompt_tokens_details.cache_write_tokens), billed independently.
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`

	// WriteCachedTokens is a legacy internal alias for cache-write tokens kept
	// for backward compatibility with older axonhub peers. New output uses
	// CacheWriteTokens; this field is tolerated as input only.
	WriteCachedTokens int64 `json:"write_cached_tokens,omitempty"`
}

// CompletionTokensDetails Breakdown of tokens used in a completion.
type CompletionTokensDetails struct {
	AudioTokens              int64 `json:"audio_tokens"`
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
}

// Usage represents the usage response from OpenAI compatible format.
// Difference provider may have different format, so we use this to convert to unified format.
//
// In addition to the official OpenAI fields, the top-level cache* fields below capture
// cache-read/write token counts emitted at the usage-object root by various
// OpenAI-compatible providers and middlewares. They are used as fallbacks when the
// official nested detail fields are absent. Unset values are omitted on marshal.
type Usage struct {
	PromptTokens            int64                   `json:"prompt_tokens"`
	CompletionTokens        int64                   `json:"completion_tokens"`
	TotalTokens             int64                   `json:"total_tokens"`
	PromptTokensDetails     PromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details"`

	// Top-level cache fields emitted by some providers/middlewares.
	CachedTokens             int64 `json:"cached_tokens,omitempty"`
	CacheReadTokens          int64 `json:"cache_read_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	CacheCreationTokens      int64 `json:"cache_creation_tokens,omitempty"`
	CacheWriteTokens         int64 `json:"cache_write_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheWriteInputTokens    int64 `json:"cache_write_input_tokens,omitempty"`
}

func firstNonZero(values ...int64) int64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}

	return 0
}

func (u *Usage) ToLLMUsage() *llm.Usage {
	if u == nil {
		return nil
	}

	usage := &llm.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}

	// Resolve cache-read and cache-write counts with official-field-first precedence,
	// falling back to provider/middleware alias field names. OpenAI's prompt_tokens
	// already includes cached tokens, so these are recorded in details only (never
	// added to PromptTokens) — matching how ComputeUsageCost subtracts them.
	cachedRead := firstNonZero(
		u.PromptTokensDetails.CachedTokens, // official nested
		u.CachedTokens,                     // top-level (Moonshot, etc.)
		u.CacheReadTokens,
		u.CacheReadInputTokens,
	)
	writeCached := firstNonZero(
		u.PromptTokensDetails.CacheWriteTokens,  // official nested
		u.PromptTokensDetails.WriteCachedTokens, // legacy internal alias
		u.CacheCreationInputTokens,              // top-level aliases
		u.CacheWriteInputTokens,
		u.CacheCreationTokens,
		u.CacheWriteTokens,
	)

	if u.PromptTokensDetails != (PromptTokensDetails{}) || cachedRead != 0 || writeCached != 0 {
		usage.PromptTokensDetails = &llm.PromptTokensDetails{
			AudioTokens:       u.PromptTokensDetails.AudioTokens,
			CachedTokens:      cachedRead,
			WriteCachedTokens: writeCached,
		}
	}

	if u.CompletionTokensDetails != (CompletionTokensDetails{}) {
		usage.CompletionTokensDetails = &llm.CompletionTokensDetails{
			AudioTokens:              u.CompletionTokensDetails.AudioTokens,
			ReasoningTokens:          u.CompletionTokensDetails.ReasoningTokens,
			AcceptedPredictionTokens: u.CompletionTokensDetails.AcceptedPredictionTokens,
			RejectedPredictionTokens: u.CompletionTokensDetails.RejectedPredictionTokens,
		}
	}

	return usage
}

// UsageFromLLM creates OpenAI Usage from unified llm.Usage.
func UsageFromLLM(u *llm.Usage) *Usage {
	if u == nil {
		return nil
	}

	usage := &Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}

	if u.PromptTokensDetails != nil {
		usage.PromptTokensDetails = PromptTokensDetails{
			AudioTokens:      u.PromptTokensDetails.AudioTokens,
			CachedTokens:     u.PromptTokensDetails.CachedTokens,
			CacheWriteTokens: u.PromptTokensDetails.WriteCachedTokens,
		}
	}

	if u.CompletionTokensDetails != nil {
		usage.CompletionTokensDetails = CompletionTokensDetails{
			AudioTokens:              u.CompletionTokensDetails.AudioTokens,
			ReasoningTokens:          u.CompletionTokensDetails.ReasoningTokens,
			AcceptedPredictionTokens: u.CompletionTokensDetails.AcceptedPredictionTokens,
			RejectedPredictionTokens: u.CompletionTokensDetails.RejectedPredictionTokens,
		}
	}

	return usage
}
