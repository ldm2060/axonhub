package kimicode

import (
	"testing"
)

func TestBuildIdentityHeadersRejectsInvalidRequiredValues(t *testing.T) {
	tests := []struct {
		name     string
		identity Identity
	}{
		{name: "empty product", identity: Identity{Version: "1.0"}},
		{name: "non-ASCII product", identity: Identity{UserAgentProduct: "月", Version: "1.0"}},
		{name: "empty version", identity: Identity{UserAgentProduct: "AxonHub"}},
		{name: "non-ASCII version", identity: Identity{UserAgentProduct: "AxonHub", Version: "月"}},
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
		UserAgentProduct: "AxonHub",
		Version:          "1.0",
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
