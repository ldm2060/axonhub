package biz

import (
	"context"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/log"
)

func (s *DataStorageService) reloadFileSystemsPeriodically(ctx context.Context) {
	err := authz.RunWithSystemBypassVoid(ctx, "refresh data storage filesystems", func(ctx context.Context) error {
		return s.refreshFileSystems(ctx)
	})
	if err != nil {
		log.Error(ctx, "failed to refresh data storage filesystems", log.Cause(err))
	}
}
