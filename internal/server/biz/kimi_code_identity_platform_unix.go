//go:build linux || freebsd

package biz

import (
	"context"
	"fmt"

	"golang.org/x/sys/unix"
)

func resolveKimiCodePlatform(context.Context) (kimiCodePlatform, error) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		return kimiCodePlatform{}, fmt.Errorf("resolve Kimi Code OS release: %w", err)
	}
	return kimiCodePlatform{
		OSType:       unix.ByteSliceToString(utsname.Sysname[:]),
		OSRelease:    unix.ByteSliceToString(utsname.Release[:]),
		Architecture: currentKimiCodeArchitecture(),
	}, nil
}
