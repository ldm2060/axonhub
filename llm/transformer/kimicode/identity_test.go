package kimicode

import (
	"testing"
)

func TestBuildIdentityHeadersMatchesOfficialCLI(t *testing.T) {
	headers, err := BuildIdentityHeaders(Identity{
		UserAgentProduct: CLIUserAgentProduct,
		Version:          CLIVersion,
		Hostname:         "desktop",
		DeviceModel:      "Windows 10.0.26200 x64",
		OSVersion:        "10.0.26200",
		DeviceID:         "stable-device-id",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"User-Agent":         "kimi-code-cli/0.26.0",
		"X-Msh-Platform":     "kimi_code_cli",
		"X-Msh-Version":      "0.26.0",
		"X-Msh-Device-Name":  "desktop",
		"X-Msh-Device-Model": "Windows 10.0.26200 x64",
		"X-Msh-Os-Version":   "10.0.26200",
		"X-Msh-Device-Id":    "stable-device-id",
	}
	for name, expected := range want {
		if got := headers.Get(name); got != expected {
			t.Fatalf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestBuildIdentityHeadersRejectsInvalidRequiredValues(t *testing.T) {
	tests := []struct {
		name     string
		identity Identity
	}{
		{name: "empty product", identity: Identity{Version: "1.0"}},
		{name: "non-ASCII product", identity: Identity{UserAgentProduct: "月", Version: "1.0"}},
		{name: "empty version", identity: Identity{UserAgentProduct: CLIUserAgentProduct}},
		{name: "non-ASCII version", identity: Identity{UserAgentProduct: CLIUserAgentProduct, Version: "月"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildIdentityHeaders(tt.identity); err == nil {
				t.Fatal("expected identity validation error")
			}
		})
	}
}

func TestBuildIdentityHeadersUsesUnknownDeviceFallbacks(t *testing.T) {
	headers, err := BuildIdentityHeaders(Identity{
		UserAgentProduct: CLIUserAgentProduct,
		Version:          CLIVersion,
		Hostname:         "月",
		DeviceModel:      "月",
		OSVersion:        "月",
		DeviceID:         "月",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"X-Msh-Device-Name", "X-Msh-Device-Model", "X-Msh-Os-Version", "X-Msh-Device-Id"} {
		if got := headers.Get(name); got != "unknown" {
			t.Fatalf("%s = %q, want unknown", name, got)
		}
	}
}
