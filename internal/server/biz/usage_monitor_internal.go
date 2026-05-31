package biz

import (
	"context"

	"github.com/ldm2060/axonhub/internal/authz"
)

func (svc *UsageMonitorService) runPollAllScheduled(ctx context.Context) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	ctx = authz.WithSystemBypass(ctx, "usage_monitor")
	svc.runPollAll(ctx)
}
