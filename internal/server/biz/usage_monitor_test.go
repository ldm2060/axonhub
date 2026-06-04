package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/usagemonitorchannel"
	"github.com/ldm2060/axonhub/internal/server/biz/usage_monitor"
)

func TestEnrichDisplayFieldsFromTemplate_UpgradesLegacyCopilotPercentRemainingValueRef(t *testing.T) {
	ch := &ent.UsageMonitorChannel{
		Source:       usagemonitorchannel.SourceTemplate,
		ProviderType: usagemonitorchannel.ProviderTypeGithubCopilot,
	}
	displayFields := []usage_monitor.DisplayField{
		{Key: "chat_pct", Label: "Chat Usage", ValueRef: "chat_pct", Format: "percentage"},
	}

	enrichDisplayFieldsFromTemplate(ch, displayFields)

	assert.Equal(t, "used_percent_from_remaining(chat_pct)", displayFields[0].ValueRef)
}
