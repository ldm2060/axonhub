package orchestrator

import (
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/server/biz"
)

var Module = fx.Module("orchestrator",
	fx.Provide(NewDefaultSelector),
	fx.Provide(NewCandidateSelectorDiagnostics),
	fx.Provide(NewChannelLimiterManager),
	fx.Provide(func(svc *biz.ProviderQuotaService) ProviderQuotaStatusProvider { return svc }),
)
