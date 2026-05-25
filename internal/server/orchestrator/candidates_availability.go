package orchestrator

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm"
)

type timeLocationProvider interface {
	TimeLocation(context.Context) *time.Location
}

type AvailabilitySelector struct {
	wrapped      CandidateSelector
	timeLocation timeLocationProvider
	now          func() time.Time
}

func WithAvailabilitySelector(wrapped CandidateSelector, timeLocation timeLocationProvider) *AvailabilitySelector {
	return &AvailabilitySelector{
		wrapped:      wrapped,
		timeLocation: timeLocation,
		now:          time.Now,
	}
}

func (s *AvailabilitySelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	loc := time.UTC
	if s.timeLocation != nil {
		loc = s.timeLocation.TimeLocation(ctx)
	}

	now := s.now().In(loc)
	return lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		return objects.IsChannelAvailable(c.Channel.Policies, now)
	}), nil
}
