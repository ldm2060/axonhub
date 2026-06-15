package datamigrate

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/log"
)

type V0_1_34 struct{}

func NewV0_1_34() DataMigrator {
	return &V0_1_34{}
}

func (v *V0_1_34) Version() string {
	return "v0.1.34"
}

func (v *V0_1_34) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	// 1. Set quota_binding_ready=true for all existing channels
	// This field was added with a default value of true, but existing rows need explicit update
	channelUpdated, err := client.Channel.Update().
		SetQuotaBindingReady(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate channels quota_binding_ready: %w", err)
	}
	if channelUpdated > 0 {
		log.Info(ctx, "set existing channels quota_binding_ready to true", log.Int("count", channelUpdated))
	}

	// 2. Set auto_disable_enabled=false for all existing usage monitor channels
	// This field was added with a default value of false, but existing rows need explicit update
	monitorUpdated, err := client.UsageMonitorChannel.Update().
		SetAutoDisableEnabled(false).
		SetAutoDisableThreshold(1.0).
		SetAutoEnableThreshold(0.95).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate usage_monitor_channels auto_disable fields: %w", err)
	}
	if monitorUpdated > 0 {
		log.Info(ctx, "set existing usage_monitor_channels auto_disable defaults", log.Int("count", monitorUpdated))
	}

	return nil
}
