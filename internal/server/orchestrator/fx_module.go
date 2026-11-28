package orchestrator

import (
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/server/biz"
	rt "github.com/ldm2060/axonhub/internal/server/realtime"
)

var Module = fx.Module("orchestrator",
	fx.Provide(NewDefaultSelector),
	fx.Provide(NewRealtimeService),
	fx.Provide(func() rt.Dialer { return rt.SecureDialer{} }),
	fx.Provide(NewCandidateSelectorDiagnostics),
	fx.Provide(NewChannelLimiterManager),
	fx.Provide(func(svc *biz.ProviderQuotaService) ProviderQuotaStatusProvider { return svc }),
	fx.Provide(func(svc *biz.ChannelPoolingService) PoolingGate { return svc }),
)
