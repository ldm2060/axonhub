package objects

import (
	"testing"
	"time"
)

func TestIsChannelAvailable_NoRules(t *testing.T) {
	ch := ChannelPolicies{}
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	if !IsChannelAvailable(ch, now) {
		t.Error("expected available when no rules")
	}
}

func TestIsChannelAvailable_NilAvailability(t *testing.T) {
	ch := ChannelPolicies{Availability: nil}
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	if !IsChannelAvailable(ch, now) {
		t.Error("expected available when availability is nil")
	}
}

func TestIsChannelAvailable_EmptyRules(t *testing.T) {
	ch := ChannelPolicies{Availability: &ChannelAvailability{Rules: nil}}
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	if !IsChannelAvailable(ch, now) {
		t.Error("expected available when rules are empty")
	}
}

func TestIsChannelAvailable_AvailableRule(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeAvailable, Days: []int{1, 2, 3, 4, 5}, StartTime: "09:00", EndTime: "18:00", Enabled: true},
			},
		},
	}
	// Monday 10:00 = available
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available on Mon 10:00")
	}
	// Monday 20:00 = not available (outside window)
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 20, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable on Mon 20:00")
	}
	// Saturday 10:00 = not available (not in days)
	if IsChannelAvailable(ch, time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable on Sat 10:00")
	}
}

func TestIsChannelAvailable_UnavailableRule(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeUnavailable, Days: nil, StartTime: "22:00", EndTime: "06:00", Enabled: true},
			},
		},
	}
	// 23:00 = unavailable
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 23, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable at 23:00")
	}
	// 05:00 = unavailable
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 5, 0, 0, 0, time.UTC)) {
		t.Error("expected unavailable at 05:00")
	}
	// 10:00 = available (outside window)
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available at 10:00")
	}
}

func TestIsChannelAvailable_DisabledRuleIgnored(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeUnavailable, StartTime: "00:00", EndTime: "23:59", Enabled: false},
			},
		},
	}
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available when rule is disabled")
	}
}

func TestIsChannelAvailable_LastMatchWins(t *testing.T) {
	ch := ChannelPolicies{
		Availability: &ChannelAvailability{
			Rules: []ChannelAvailabilityRule{
				{Type: ChannelAvailabilityRuleTypeAvailable, StartTime: "00:00", EndTime: "23:59", Enabled: true},
				{Type: ChannelAvailabilityRuleTypeUnavailable, StartTime: "12:00", EndTime: "13:00", Enabled: true},
			},
		},
	}
	// 10:00 matches first rule → available
	if !IsChannelAvailable(ch, time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected available at 10:00")
	}
	// 12:30 matches both, last wins → unavailable
	if IsChannelAvailable(ch, time.Date(2026, 5, 25, 12, 30, 0, 0, time.UTC)) {
		t.Error("expected unavailable at 12:30")
	}
}

func TestMatchesTimeWindow_SameDay(t *testing.T) {
	if !MatchesTimeWindow("10:00", "09:00", "18:00") {
		t.Error("10:00 in 09:00–18:00")
	}
	if MatchesTimeWindow("08:00", "09:00", "18:00") {
		t.Error("08:00 not in 09:00–18:00")
	}
	if MatchesTimeWindow("18:00", "09:00", "18:00") {
		t.Error("18:00 not in 09:00–18:00 (exclusive end)")
	}
}

func TestMatchesTimeWindow_CrossDay(t *testing.T) {
	if !MatchesTimeWindow("23:00", "22:00", "06:00") {
		t.Error("23:00 in 22:00–06:00 cross-day")
	}
	if !MatchesTimeWindow("03:00", "22:00", "06:00") {
		t.Error("03:00 in 22:00–06:00 cross-day")
	}
	if MatchesTimeWindow("10:00", "22:00", "06:00") {
		t.Error("10:00 not in 22:00–06:00 cross-day")
	}
}

func TestISOWeekday(t *testing.T) {
	// 2026-05-25 is Monday
	if w := ISOWeekday(time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)); w != 1 {
		t.Errorf("Monday = %d, want 1", w)
	}
	// 2026-05-31 is Sunday
	if w := ISOWeekday(time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)); w != 7 {
		t.Errorf("Sunday = %d, want 7", w)
	}
}
