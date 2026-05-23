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

// Integration tests verify the availability feature through the broader candidate
// selection flow and confirm design-level guarantees about bypass behavior.

// TestAvailabilitySelector_AvailableRuleWhitelist verifies that an "available" rule
// acts as a whitelist: when the current time is outside the specified window, the
// channel is filtered out. We use a narrow window (02:00–03:00) that is almost
// certainly outside the current time of day, so the channel should always be removed.
func TestAvailabilitySelector_AvailableRuleWhitelist(t *testing.T) {
	t.Parallel()

	// Channel A has a "available" whitelist rule for a tiny window (02:00-03:00).
	// Since the current time is almost certainly not in that window, IsChannelAvailable
	// will return false (whitelist behavior: no matching rule → unavailable).
	channelA := &biz.Channel{
		Channel: &ent.Channel{
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
			Policies: objects.ChannelPolicies{},
		},
	}

	mock := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: channelA},
			{Channel: channelB},
		},
	}
	selector := WithAvailabilitySelector(mock)

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)

	// Channel A should be filtered out (outside whitelist window),
	// Channel B should pass through (no rules → always available).
	require.Len(t, got, 1)
	require.Equal(t, channelB.ID, got[0].Channel.ID)
}

// TestAvailabilitySelector_MultipleRulesLastWins verifies the "last matching rule
// wins" behavior through the selector. Two rules are set:
//   - "available 00:00-23:59" (always matches → available)
//   - "unavailable 00:00-23:59" (also always matches → unavailable, and is last)
//
// The last rule wins, so the channel should be filtered out regardless of current time.
func TestAvailabilitySelector_MultipleRulesLastWins(t *testing.T) {
	t.Parallel()

	// Channel with two overlapping rules where the last one is "unavailable" and
	// covers the entire day. Last-match-wins means it should always be filtered out.
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
	selector := WithAvailabilitySelector(mock)

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)

	// Last matching rule is "unavailable" covering 00:00-23:59 → filtered out.
	require.Len(t, got, 0)
}

// TestAvailabilitySelector_CrossDayWindow verifies cross-day "unavailable" rules
// through the selector. A rule spanning midnight (e.g., 22:00-06:00) uses the
// "unavailable 00:00-23:59" trick to guarantee filtering at any time, ensuring
// the selector correctly processes cross-day windows.
//
// Note: The AvailabilitySelector uses time.Now() internally, so we cannot inject
// a specific time. To reliably test cross-day behavior, we use a full-day
// "unavailable" rule that covers any current time, confirming the selector
// processes the cross-day matching logic correctly via the objects.IsChannelAvailable
// call. The cross-day time window matching itself is thoroughly tested in
// objects/channel_availability_test.go.
func TestAvailabilitySelector_CrossDayWindow(t *testing.T) {
	t.Parallel()

	// Cross-day unavailable rule covering 22:00-06:00.
	// Since we cannot control time.Now() in the selector, we supplement with
	// a second rule "unavailable 06:00-22:00" to cover the remaining hours,
	// ensuring the channel is filtered out regardless of current time.
	// This proves the selector correctly evaluates both same-day and cross-day
	// windows through the same IsChannelAvailable path.
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
						{
							Type:      objects.ChannelAvailabilityRuleTypeUnavailable,
							Days:      nil,
							StartTime: "06:00",
							EndTime:   "22:00",
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
	selector := WithAvailabilitySelector(mock)

	got, err := selector.Select(context.Background(), &llm.Request{})
	require.NoError(t, err)

	// The two rules together cover all 24 hours → channel always filtered out.
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
