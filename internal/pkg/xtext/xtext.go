// Package xtext provides CJK-aware text helpers shared across the backend.
package xtext

import "strings"

// isCJKRune reports whether r is a CJK (Han/Hiragana/Katakana/Hangul) script character.
func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul Syllables
}

// IsCJKName reports whether any of the given names contains a CJK script character.
func IsCJKName(names ...string) bool {
	for _, name := range names {
		for _, r := range name {
			if isCJKRune(r) {
				return true
			}
		}
	}
	return false
}

// FormatUserName returns CJK names as surname followed by given name (no space),
// and other names in Western "First Last" order. Matches the frontend's
// formatUserName helper so display order stays consistent across layers.
func FormatUserName(firstName, lastName string) string {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)

	if first != "" && last != "" {
		if IsCJKName(first, last) {
			return last + first
		}
		return first + " " + last
	}

	if first != "" {
		return first
	}
	return last
}
