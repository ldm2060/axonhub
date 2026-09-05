package orchestrator

import (
	"context"

	entchannel "github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/pipeline"
	"github.com/ldm2060/axonhub/llm/transformer/anthropic/claudecode"
)

type billingSystemMessageMiddleware struct {
	pipeline.DummyMiddleware

	state *PersistenceState
}

var _ pipeline.OutboundLlmRequestMiddleware = (*billingSystemMessageMiddleware)(nil)

// newBillingSystemMessageMiddleware preserves Claude Code billing metadata only
// for the official OAuth channel selected for the current outbound attempt.
func newBillingSystemMessageMiddleware(state *PersistenceState) pipeline.OutboundLlmRequestMiddleware {
	return &billingSystemMessageMiddleware{
		DummyMiddleware: pipeline.DummyMiddleware{},
		state:           state,
	}
}

func (m *billingSystemMessageMiddleware) Name() string {
	return "claudecode-billing-system-message"
}

func (m *billingSystemMessageMiddleware) OnOutboundLlmRequest(
	_ context.Context,
	request *llm.Request,
	_ llm.APIFormat,
) (*llm.Request, error) {
	if m.state != nil && m.state.CurrentCandidate != nil {
		channel := m.state.CurrentCandidate.Channel
		if channel != nil && channel.Type == entchannel.TypeClaudecode && channel.Credentials.IsOAuth() {
			return request, nil
		}
	}

	return claudecode.RemoveBillingSystemMessages(request), nil
}
