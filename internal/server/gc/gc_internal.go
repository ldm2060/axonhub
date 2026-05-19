package gc

import (
	"context"

	"github.com/ldm2060/axonhub/internal/authz"
)

func (w *Worker) runAutomaticCleanup(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "gc-cleanup")
	w.runCleanup(ctx, false, nil)
}
