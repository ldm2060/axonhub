package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeArchitecture(t *testing.T) {
	tests := map[string]string{
		"amd64":   "x64",
		"386":     "ia32",
		"arm":     "arm",
		"arm64":   "arm64",
		"loong64": "loong64",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, want, nodeArchitecture(input))
		})
	}
}

func TestFormatKimiCodeDeviceModel(t *testing.T) {
	tests := []struct {
		name     string
		platform kimiCodePlatform
		want     string
	}{
		{name: "Windows", platform: kimiCodePlatform{OSType: "Windows_NT", OSRelease: "10.0.26200", Architecture: "x64"}, want: "Windows 10.0.26200 x64"},
		{name: "Linux", platform: kimiCodePlatform{OSType: "Linux", OSRelease: "6.8.0-101-generic", Architecture: "x64"}, want: "Linux 6.8.0-101-generic x64"},
		{name: "FreeBSD", platform: kimiCodePlatform{OSType: "FreeBSD", OSRelease: "14.2-RELEASE", Architecture: "arm64"}, want: "FreeBSD 14.2-RELEASE arm64"},
		{name: "Darwin product version", platform: kimiCodePlatform{OSType: "Darwin", OSRelease: "24.5.0", Architecture: "arm64", ProductVersion: "15.5"}, want: "macOS 15.5 arm64"},
		{name: "Darwin fallback", platform: kimiCodePlatform{OSType: "Darwin", OSRelease: "24.5.0", Architecture: "x64"}, want: "macOS 24.5.0 x64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatKimiCodeDeviceModel(tt.platform))
		})
	}
}

func TestResolveKimiCodePlatform(t *testing.T) {
	platform, err := resolveKimiCodePlatform(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, platform.OSType)
	require.NotEmpty(t, platform.OSRelease)
	require.NotEmpty(t, platform.Architecture)
	require.NotEmpty(t, formatKimiCodeDeviceModel(platform))
}
