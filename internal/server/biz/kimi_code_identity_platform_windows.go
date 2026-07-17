//go:build windows

package biz

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

func resolveKimiCodePlatform(context.Context) (kimiCodePlatform, error) {
	version := windows.RtlGetVersion()
	release := fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
	return kimiCodePlatform{
		OSType:       "Windows_NT",
		OSRelease:    release,
		Architecture: currentKimiCodeArchitecture(),
	}, nil
}
