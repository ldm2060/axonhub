package datamigrate

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channelusagemonitorbinding"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/objects"
)

type V0_1_35 struct{}

func NewV0_1_35() DataMigrator {
	return &V0_1_35{}
}

func (v *V0_1_35) Version() string {
	return "v0.1.35"
}

func (v *V0_1_35) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	// Find all UsageMonitorChannels that have a channel_id set (linked to a Channel)
	// and are not soft-deleted. These represent the old UsageMonitorChannel->Channel
	// relationship that needs to be migrated to ChannelUsageMonitorBinding rows.
	monitors, err := client.UsageMonitorChannel.Query().
		Where(
			usagemonitorchannel.ChannelIDNotNil(),
			usagemonitorchannel.DeletedAtEQ(0),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("migrate v0.1.35: query usage_monitor_channels: %w", err)
	}

	if len(monitors) == 0 {
		return nil
	}

	migrated := 0
	for _, mon := range monitors {
		channelID := *mon.ChannelID

		// Check if a binding already exists for this (channel_id, usage_monitor_channel_id) pair
		// to ensure idempotency.
		exists, err := client.ChannelUsageMonitorBinding.Query().
			Where(
				channelusagemonitorbinding.ChannelIDEQ(channelID),
				channelusagemonitorbinding.UsageMonitorChannelIDEQ(mon.ID),
				channelusagemonitorbinding.DeletedAtEQ(0),
			).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("migrate v0.1.35: check existing binding for monitor %d: %w", mon.ID, err)
		}
		if exists {
			continue
		}

		// Build the binding based on the monitor's auto-disable configuration.
		creator := client.ChannelUsageMonitorBinding.Create().
			SetChannelID(channelID).
			SetUsageMonitorChannelID(mon.ID)

		if mon.AutoDisableEnabled {
			// Monitor had auto-disable enabled: create an enabled binding with a
			// condition that mirrors the old threshold behavior.
			threshold := mon.AutoDisableThreshold
			if threshold <= 0 {
				threshold = 1.0
			}
			creator = creator.
				SetEnabled(true).
				SetConditions([]objects.QuotaMonitorBindingCondition{
					{
						Field:    "maxUsageRatio",
						Operator: objects.QuotaMonitorOperatorGTE,
						Value:    fmt.Sprintf("%.4f", threshold),
					},
				})
		} else {
			// Monitor had auto-disable off: create a disabled binding with empty
			// conditions. The old relationship is preserved but ineffective.
			creator = creator.
				SetEnabled(false).
				SetTriggerStatuses([]string{}).
				SetConditions([]objects.QuotaMonitorBindingCondition{})
		}

		if _, err := creator.Save(ctx); err != nil {
			return fmt.Errorf("migrate v0.1.35: create binding for monitor %d -> channel %d: %w", mon.ID, channelID, err)
		}
		migrated++
	}

	// Reset quotaBindingReady to true for all existing channels so old channels
	// are not accidentally excluded from routing after the migration.
	channelUpdated, err := client.Channel.Update().
		SetQuotaBindingReady(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate v0.1.35: reset channels quota_binding_ready: %w", err)
	}

	log.Info(ctx, "migrated usage monitor channel bindings",
		log.Int("monitors_found", len(monitors)),
		log.Int("bindings_created", migrated),
		log.Int("channels_reset_quota_binding_ready", channelUpdated),
	)

	return nil
}
