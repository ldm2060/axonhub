package orchestrator

import (
	"context"

	"github.com/ldm2060/axonhub/llm"
)

type PromptProtecter interface {
	Protect(ctx context.Context, req *llm.Request) (*llm.Request, error)
}
