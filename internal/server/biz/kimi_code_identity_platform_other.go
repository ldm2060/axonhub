//go:build !windows && !linux && !freebsd && !darwin

package biz

import (
	"context"
	"errors"
)

func resolveKimiCodePlatform(context.Context) (kimiCodePlatform, error) {
	return kimiCodePlatform{}, errors.New("Kimi Code identity does not support this operating system")
}
