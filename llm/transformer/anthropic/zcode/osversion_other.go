//go:build !windows

package zcode

// osVersionWindows is only defined on Windows; other platforms report their
// version through osVersion() directly.
func osVersionWindows() string { return "unknown" }
