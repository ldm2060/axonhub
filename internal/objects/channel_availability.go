package objects

import "time"

// IsChannelAvailable evaluates a channel's availability rules against the given time.
// Returns true if the channel should be considered available.
// No rules (nil Availability or empty Rules) → default available.
func IsChannelAvailable(policies ChannelPolicies, now time.Time) bool {
	if policies.Availability == nil || len(policies.Availability.Rules) == 0 {
		return true
	}

	available := false
	weekday := ISOWeekday(now)
	hhmm := now.Format("15:04")
	matched := false

	for _, rule := range policies.Availability.Rules {
		if !rule.Enabled {
			continue
		}
		if len(rule.Days) > 0 && !containsInt(rule.Days, weekday) {
			continue
		}
		if !MatchesTimeWindow(hhmm, rule.StartTime, rule.EndTime) {
			continue
		}
		matched = true
		available = (rule.Type == ChannelAvailabilityRuleTypeAvailable)
	}

	if !matched {
		// When no rule matches, the default depends on the rule types:
		// - If any enabled "available" rule exists, the rules act as a whitelist → unavailable.
		// - If only "unavailable" rules exist, the rules act as a blacklist → available.
		for _, rule := range policies.Availability.Rules {
			if rule.Enabled && rule.Type == ChannelAvailabilityRuleTypeAvailable {
				return false
			}
		}
		return true
	}

	return available
}

// ISOWeekday returns the ISO 8601 weekday: 1=Monday ... 7=Sunday.
func ISOWeekday(t time.Time) int {
	w := int(t.Weekday())
	if w == 0 {
		return 7
	}
	return w
}

// MatchesTimeWindow checks whether hhmm ("HH:MM") falls within the time window [start, end).
// Supports cross-day windows where start > end (e.g. "22:00"–"06:00").
func MatchesTimeWindow(hhmm, start, end string) bool {
	if start <= end {
		return hhmm >= start && hhmm < end
	}
	// Cross-day: matches >= start OR < end
	return hhmm >= start || hhmm < end
}

func containsInt(slice []int, v int) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}
