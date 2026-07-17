package biz

import (
	"fmt"
	"runtime"
	"strings"
)

type kimiCodePlatform struct {
	OSType         string
	OSRelease      string
	Architecture   string
	ProductVersion string
}

func nodeArchitecture(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return goarch
	}
}

func formatKimiCodeDeviceModel(platform kimiCodePlatform) string {
	architecture := strings.TrimSpace(platform.Architecture)
	release := strings.TrimSpace(platform.OSRelease)
	switch strings.TrimSpace(platform.OSType) {
	case "Darwin":
		if productVersion := strings.TrimSpace(platform.ProductVersion); productVersion != "" {
			release = productVersion
		}
		return strings.TrimSpace(fmt.Sprintf("macOS %s %s", release, architecture))
	case "Windows_NT":
		return strings.TrimSpace(fmt.Sprintf("Windows %s %s", release, architecture))
	default:
		return strings.TrimSpace(fmt.Sprintf("%s %s %s", platform.OSType, release, architecture))
	}
}

func currentKimiCodeArchitecture() string {
	return nodeArchitecture(runtime.GOARCH)
}
