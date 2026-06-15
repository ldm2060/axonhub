package datamigrate

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/log"
)

type V0_1_33 struct{}

func NewV0_1_33() DataMigrator {
	return &V0_1_33{}
}

func (v *V0_1_33) Version() string {
	return "v0.1.33"
}

func (v *V0_1_33) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(context.Background(), "database-migrate")

	// Set quota_binding_ready=true for all existing channels
	// This field was added with a default value of true,
	// but existing rows need to be updated explicitly
	channelUpdated, err := client.Channel.Update().
		SetQuotaBindingReady(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate channels quota_binding_ready: %w", err)
	}
	if channelUpdated > 0 {
		log.Info(ctx, "set existing channels quota_binding_ready to true", log.Int("count", channelUpdated))
	}

	return nil
}
