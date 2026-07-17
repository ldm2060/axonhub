//go:build darwin

package biz

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func resolveKimiCodePlatform(ctx context.Context) (kimiCodePlatform, error) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		return kimiCodePlatform{}, fmt.Errorf("resolve Kimi Code OS release: %w", err)
	}
	platform := kimiCodePlatform{
		OSType:       unix.ByteSliceToString(utsname.Sysname[:]),
		OSRelease:    unix.ByteSliceToString(utsname.Release[:]),
		Architecture: currentKimiCodeArchitecture(),
	}

	versionCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if output, err := exec.CommandContext(versionCtx, "/usr/bin/sw_vers", "-productVersion").Output(); err == nil {
		platform.ProductVersion = strings.TrimSpace(string(output))
	}
	return platform, nil
}
