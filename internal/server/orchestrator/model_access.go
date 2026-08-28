package orchestrator

import (
	"context"
	"fmt"
	"slices"

	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
	"github.com/ldm2060/axonhub/llm/pipeline"
)

func checkApiKeyModelAccess(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("check-api-key-model-access", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if llmRequest.Model == "" {
			return nil, fmt.Errorf("%w: request model is empty", biz.ErrInvalidModel)
		}

		if inbound.state.APIKey == nil {
			return llmRequest, nil
		}

		profile := inbound.state.APIKey.GetActiveProfile()
		if profile == nil {
			return llmRequest, nil
		}

		// Deny-list wins even when the allow-list is empty (ModelIDs == 0 means
		// allow-all, but an excluded model must never be reachable).
		if len(profile.ExcludedModelIDs) > 0 {
			if slices.Contains(profile.ExcludedModelIDs, llmRequest.Model) {
				log.Warn(ctx, "model access denied by API key profile exclude list",
					log.String("api_key_name", inbound.state.APIKey.Name),
					log.String("active_profile", inbound.state.APIKey.Profiles.ActiveProfile),
					log.String("model", llmRequest.Model),
					log.Any("excluded_models", profile.ExcludedModelIDs))

				return nil, fmt.Errorf("%w: %s", biz.ErrInvalidModel, llmRequest.Model)
			}
		}

		if len(profile.ModelIDs) == 0 {
			return llmRequest, nil
		}

		allowed := slices.Contains(profile.ModelIDs, llmRequest.Model)

		if !allowed {
			log.Warn(ctx, "model access denied by API key profile",
				log.String("api_key_name", inbound.state.APIKey.Name),
				log.String("active_profile", inbound.state.APIKey.Profiles.ActiveProfile),
				log.String("model", llmRequest.Model),
				log.Any("allowed_models", profile.ModelIDs))

			return nil, fmt.Errorf("%w: %s", biz.ErrInvalidModel, llmRequest.Model)
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "model access allowed by API key profile",
				log.String("api_key_name", inbound.state.APIKey.Name),
				log.String("active_profile", inbound.state.APIKey.Profiles.ActiveProfile),
				log.String("model", llmRequest.Model))
		}

		return llmRequest, nil
	})
}
