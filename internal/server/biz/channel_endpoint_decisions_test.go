package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/llm"
)

func TestValidateEndpoints_AllowsManualDecisionsEndpoint(t *testing.T) {
	// Given a manually configured Decisions endpoint
	endpoints := []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIDecisions.String()}}

	// When endpoint configuration is validated
	err := ValidateEndpoints(endpoints)

	// Then the opt-in format is accepted without becoming a default
	require.NoError(t, err)
}
