package backup

import (
	"context"

	"github.com/ldm2060/axonhub/internal/authz"
)

func (svc *BackupService) runBackupPeriodically(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "run-auto-backup")
	svc.triggerAutoBackup(ctx)
}
