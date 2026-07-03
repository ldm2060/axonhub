package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
)

// ClientRestrictionSelector filters candidates based on client restriction policies.
type ClientRestrictionSelector struct {
	wrapped       CandidateSelector
	retryProvider RetryPolicyProvider
	checker       *biz.ClientRestrictionChecker
}

// WithClientRestrictionSelector creates a selector that filters candidates by client restriction.
func WithClientRestrictionSelector(wrapped CandidateSelector, retryProvider RetryPolicyProvider) *ClientRestrictionSelector {
	return &ClientRestrictionSelector{
		wrapped:       wrapped,
		retryProvider: retryProvider,
		checker:       biz.NewClientRestrictionChecker(),
	}
}

func (s *ClientRestrictionSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	// Get User-Agent and Referer from raw request headers
	userAgent := ""
	referer := ""
	if req.RawRequest != nil && req.RawRequest.Headers != nil {
		userAgent = req.RawRequest.Headers.Get("User-Agent")
		referer = req.RawRequest.Headers.Get("Referer")
	}

	// Get global client restriction setting
	retryPolicy := s.retryProvider.RetryPolicyOrDefault(ctx)

	// Filter candidates by client restriction
	filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		// Convert ent.ClientRestriction to biz.ClientRestrictionLevel
		var channelRestriction *biz.ClientRestrictionLevel
		if c.Channel.ClientRestriction != nil {
			level := biz.ClientRestrictionLevel(*c.Channel.ClientRestriction)
			channelRestriction = &level
		}

		allowed := s.checker.CheckClientRestriction(
			userAgent,
			referer,
			c.Channel.Type.String(),
			channelRestriction,
			retryPolicy.ClientRestriction,
		)
		return allowed
	})

	return filtered, nil
}
