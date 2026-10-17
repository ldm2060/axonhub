package orchestrator

import (
	"context"

	"github.com/ldm2060/axonhub/llm/pipeline"
	"github.com/ldm2060/axonhub/llm/transformer/shared"
)

type responsesHealthMiddleware struct{ pipeline.Middleware }

func withoutResponsesRecoveryPenalty(m pipeline.Middleware) pipeline.Middleware {
	return &responsesHealthMiddleware{Middleware: m}
}

func (m *responsesHealthMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	if shared.IsResponsesRecoveryError(err) {
		if tracker, ok := m.Middleware.(*modelCircuitBreakerTracker); ok {
			tracker.releaseProbeLease()
		}
		return
	}
	m.Middleware.OnOutboundRawError(ctx, err)
}
