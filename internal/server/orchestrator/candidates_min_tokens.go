package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/llm"
)

// MinInputTokensSelector filters out channels whose MinInputTokens setting
// exceeds the estimated prompt token count of the request.
type MinInputTokensSelector struct {
	wrapped CandidateSelector
}

func WithMinInputTokensSelector(wrapped CandidateSelector) *MinInputTokensSelector {
	return &MinInputTokensSelector{wrapped: wrapped}
}

func (s *MinInputTokensSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	promptTokens := estimatePromptTokens(req)

	filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		if c.Channel == nil || c.Channel.Settings == nil || c.Channel.Settings.MinInputTokens == nil {
			return true
		}
		return promptTokens >= int64(*c.Channel.Settings.MinInputTokens)
	})

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "MinInputTokensSelector: filtered candidates",
			log.Int64("estimated_prompt_tokens", promptTokens),
			log.Int("before", len(candidates)),
			log.Int("after", len(filtered)),
		)
	}

	return filtered, nil
}
