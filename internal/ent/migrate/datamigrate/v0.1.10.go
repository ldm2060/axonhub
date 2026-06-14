package datamigrate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
)

// V0_1_10 implements DataMigrator for version 0.1.10 migration.
// It converts old fields data to the new variables + display_fields format.
type V0_1_10 struct{}

// NewV0_1_10 creates a new V0_1_10 data migrator.
func NewV0_1_10() DataMigrator {
	return &V0_1_10{}
}

// Version returns the version of this migrator.
func (v *V0_1_10) Version() string {
	return "v0.1.10"
}

// Migrate converts old fields data to variables + display_fields for all UsageMonitorChannels.
func (v *V0_1_10) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	channels, err := client.UsageMonitorChannel.Query().All(ctx)
	if err != nil {
		return err
	}

	migrated := 0
	for _, ch := range channels {
		// Skip if already migrated
		if len(ch.Variables) > 0 {
			continue
		}

		// Skip channels with no fields to convert
		if len(ch.Fields) == 0 {
			continue
		}

		// Convert old fields to new format
		var fcs []usage_monitor.FieldConfig
		fieldsJSON, err := json.Marshal(ch.Fields)
		if err != nil {
			log.Warn(ctx, "failed to marshal fields for channel",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			continue
		}
		if err := json.Unmarshal(fieldsJSON, &fcs); err != nil {
			log.Warn(ctx, "failed to unmarshal fields for channel",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			continue
		}

		vars := usage_monitor.VariablesFromFieldConfigs(fcs)
		dfs := usage_monitor.DisplayFieldsFromFieldConfigs(fcs)

		// Convert to []map[string]any for Ent
		varsMaps, err := structSliceToMapSlice(vars)
		if err != nil {
			log.Warn(ctx, "failed to convert variables to maps",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			continue
		}
		dfsMaps, err := structSliceToMapSlice(dfs)
		if err != nil {
			log.Warn(ctx, "failed to convert display_fields to maps",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			continue
		}

		_, err = client.UsageMonitorChannel.UpdateOneID(ch.ID).
			SetVariables(varsMaps).
			SetDisplayFields(dfsMaps).
			Save(ctx)
		if err != nil {
			log.Warn(ctx, "failed to update channel with variables/display_fields",
				log.Int("channel_id", ch.ID),
				log.Cause(err))
			continue
		}

		migrated++
	}

	if migrated > 0 {
		log.Info(ctx, "migrated usage monitor channels from fields to variables/display_fields",
			log.Int("migrated", migrated),
			log.Int("total", len(channels)))
	}

	return nil
}

// structSliceToMapSlice converts a slice of structs to a slice of maps using JSON marshaling.
func structSliceToMapSlice[T any](in []T) ([]map[string]any, error) {
	out := make([]map[string]any, len(in))
	for i, v := range in {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal: %w", err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal: %w", err)
		}
		out[i] = m
	}
	return out, nil
}
