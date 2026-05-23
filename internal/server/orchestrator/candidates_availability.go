package orchestrator

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm"
)

type AvailabilitySelector struct {
	wrapped CandidateSelector
}

func WithAvailabilitySelector(wrapped CandidateSelector) *AvailabilitySelector {
	return &AvailabilitySelector{wrapped: wrapped}
}

func (s *AvailabilitySelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		return objects.IsChannelAvailable(c.Channel.Policies, now)
	}), nil
}
