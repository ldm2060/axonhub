//go:build windows

package zcode

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// osVersionWindows returns the Windows build number (e.g. "10.0.26200") the
// way os.version() does in the Electron desktop client.
func osVersionWindows() string {
	if v := windows.RtlGetVersion(); v != nil {
		return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
	}
	return "unknown"
}
