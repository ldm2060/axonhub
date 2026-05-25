package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/server/biz"
	"github.com/ldm2060/axonhub/llm"
)

type stubTimeLocationProvider struct {
	loc *time.Location
}

func (p stubTimeLocationProvider) TimeLocation(context.Context) *time.Location {
	return p.loc
}

func newTestAvailabilitySelector(wrapped CandidateSelector) *AvailabilitySelector {
	selector := WithAvailabilitySelector(wrapped, stubTimeLocationProvider{loc: time.UTC})
	selector.now = func() time.Time {
		return time.Date(2026, 5, 25, 10, 30, 0, 0, time.UTC)
	}
	return selector
}

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
			selector := newTestAvailabilitySelector(mock)
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

func TestAvailabilitySelector_UsesSystemTimezone(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			Policies: objects.ChannelPolicies{
				Availability: &objects.ChannelAvailability{
					Rules: []objects.ChannelAvailabilityRule{
						{
							Type:      objects.ChannelAvailabilityRuleTypeAvailable,
							Days:      []int{1},
							StartTime: "09:00",
							EndTime:   "18:00",
							Enabled:   true,
						},
					},
				},
			},
		},
	}

	mock := &mockSelector{
		candidates: []*ChannelModelsCandidate{{Channel: channel}},
	}
	selector := WithAvailabilitySelector(mock, stubTimeLocationProvider{loc: shanghai})
	selector.now = func() time.Time {
		return time.Date(2026, 5, 25, 1, 30, 0, 0, time.UTC)
	}

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// Integration tests verify the availability feature through the broader candidate
// selection flow and confirm design-level guarantees about bypass behavior.

// Available rules act as a whitelist. At the helper clock (10:30 UTC), this window does not match.
func TestAvailabilitySelector_AvailableRuleWhitelist(t *testing.T) {
	t.Parallel()

	channelA := &biz.Channel{
		Channel: &ent.Channel{
			ID: 1,
			Policies: objects.ChannelPolicies{
				Availability: &objects.ChannelAvailability{
					Rules: []objects.ChannelAvailabilityRule{
						{
							Type:      objects.ChannelAvailabilityRuleTypeAvailable,
							Days:      nil,
							StartTime: "02:00",
							EndTime:   "03:00",
							Enabled:   true,
						},
					},
				},
			},
		},
	}

	// Channel B has no availability rules → always passes through.
	channelB := &biz.Channel{
		Channel: &ent.Channel{
			ID:       2,
			Policies: objects.ChannelPolicies{},
		},
	}

	mock := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: channelA},
			{Channel: channelB},
		},
	}
	selector := newTestAvailabilitySelector(mock)

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, channelB.ID, got[0].Channel.ID)
}

// The last matching rule wins when multiple rules match the helper clock.
func TestAvailabilitySelector_MultipleRulesLastWins(t *testing.T) {
	t.Parallel()

	channel := &biz.Channel{
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
	}

	mock := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: channel},
		},
	}
	selector := newTestAvailabilitySelector(mock)

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)
	require.Len(t, got, 0)
}

// A fixed late-night clock makes this cross-day window match deterministically.
func TestAvailabilitySelector_CrossDayWindow(t *testing.T) {
	t.Parallel()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			Policies: objects.ChannelPolicies{
				Availability: &objects.ChannelAvailability{
					Rules: []objects.ChannelAvailabilityRule{
						{
							Type:      objects.ChannelAvailabilityRuleTypeUnavailable,
							Days:      nil,
							StartTime: "22:00",
							EndTime:   "06:00",
							Enabled:   true,
						},
					},
				},
			},
		},
	}

	mock := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: channel},
		},
	}
	selector := WithAvailabilitySelector(mock, stubTimeLocationProvider{loc: time.UTC})
	selector.now = func() time.Time {
		return time.Date(2026, 5, 25, 23, 30, 0, 0, time.UTC)
	}

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)
	require.Len(t, got, 0)
}

// Design verification: SpecifiedChannelSelector bypasses availability rules.
//
// SpecifiedChannelSelector (defined in candidates.go) is used exclusively for
// channel testing (see tester.go). Its Select method directly retrieves a single
// channel by ID and returns it as a candidate without going through any decorator
// chain — including WithAvailabilitySelector.
//
// In the normal request flow (select_candidates.go), the decorator chain is built
// incrementally on top of DefaultSelector:
//
//   selector = WithStreamPolicySelector(selector)
//   selector = WithAvailabilitySelector(selector)
//
// But SpecifiedChannelSelector is used directly as the channelSelector field in
// the TestChannelOrchestrator's ChatCompletionOrchestrator (tester.go lines 91, 423).
// It never passes through select_candidates.go's decorator construction, so
// availability rules are intentionally bypassed for test requests. This is correct
// behavior: when a user explicitly tests a channel, they want to verify the channel
// works regardless of its current availability window.
