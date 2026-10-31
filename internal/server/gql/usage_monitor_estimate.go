package gql

import (
	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
)

// estimateConfigsFromInput converts the GraphQL estimate config inputs into
// the biz representation stored on the monitor row. Lives outside the
// resolvers file because gqlgen regenerates that file and strips helper
// functions.
func estimateConfigsFromInput(data []*EstimateConfigInput) []usage_monitor.EstimateConfig {
	cfgs := make([]usage_monitor.EstimateConfig, 0, len(data))
	for _, entry := range data {
		if entry == nil {
			continue
		}
		cfgs = append(cfgs, usage_monitor.EstimateConfig{
			FieldKey:      entry.FieldKey,
			Remaining:     lo.FromPtr(entry.Remaining),
			ResetFieldKey: lo.FromPtr(entry.ResetFieldKey),
			Window:        entry.Window,
		})
	}
	return cfgs
}
