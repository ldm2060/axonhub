package gql

import (
	"github.com/samber/lo"

	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
)

// estimateConfigFromInput converts the GraphQL estimate config input into the
// biz representation stored on the monitor row. Lives outside the resolvers
// file because gqlgen regenerates that file and strips helper functions.
func estimateConfigFromInput(data *EstimateConfigInput) *usage_monitor.EstimateConfig {
	return &usage_monitor.EstimateConfig{
		FieldKey:      data.FieldKey,
		Remaining:     lo.FromPtr(data.Remaining),
		ResetFieldKey: lo.FromPtr(data.ResetFieldKey),
		Window:        data.Window,
	}
}
