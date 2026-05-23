package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
)

func TestAvailabilitySelector_Select(t *testing.T) {
	tests := []struct {
		name       string
		candidates []*ChannelModelsCandidate
		mockErr    error
		wantCount  int
		wantErr    bool
	}{
		{
			name: "no availability rules - passes through",
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "nil availability - passes through",
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Availability: nil,
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "unavailable rule covering current time - filtered out",
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Availability: &objects.ChannelAvailability{
									Rules: []objects.ChannelAvailabilityRule{
										{
											Type:      objects.ChannelAvailabilityRuleTypeUnavailable,
											Days:      nil, // every day
											StartTime: "00:00",
											EndTime:   "23:59",
											Enabled:   true,
										},
									},
								},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "mixed channels - one available, one unavailable",
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Availability: &objects.ChannelAvailability{
									Rules: []objects.ChannelAvailabilityRule{
										{
											Type:      objects.ChannelAvailabilityRuleTypeUnavailable,
											Days:      nil,
											StartTime: "00:00",
											EndTime:   "23:59",
											Enabled:   true,
										},
									},
								},
							},
						},
					},
				},
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "available rule covering current time - passes through",
			candidates: []*ChannelModelsCandidate{
				{
					Channel: &biz.Channel{
						Channel: &ent.Channel{
							Policies: objects.ChannelPolicies{
								Availability: &objects.ChannelAvailability{
									Rules: []objects.ChannelAvailabilityRule{
										{
											Type:      objects.ChannelAvailabilityRuleTypeAvailable,
											Days:      nil,
											StartTime: "00:00",
											EndTime:   "23:59",
											Enabled:   true,
										},
									},
								},
							},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name:       "no candidates",
			candidates: []*ChannelModelsCandidate{},
			wantCount:  0,
		},
		{
			name:    "wrapped error",
			mockErr: errors.New("wrapped error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSelector{candidates: tt.candidates, err: tt.mockErr}
			selector := WithAvailabilitySelector(mock)
			req := &llm.Request{}

			got, err := selector.Select(context.Background(), req)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, got, tt.wantCount)
		})
	}
}
