package kimicode

import (
	"net/http"
	"strings"
)

// Identity is the stable host and device identity Kimi Code requires for every
// OAuth, catalog, and inference request. Its values are resolved by the host
// application; this package deliberately has no dependency on internal code.
type Identity struct {
	Version     string
	Hostname    string
	DeviceModel string
	OSVersion   string
	DeviceID    string
	UserAgent   string
}

// BuildIdentityHeaders returns the Kimi Code identity headers after removing
// non-printable bytes. Required values fall back to a safe non-empty value so a
// malformed local hostname cannot create an invalid HTTP header.
func BuildIdentityHeaders(identity Identity) http.Header {
	headers := make(http.Header)
	headers.Set("User-Agent", asciiHeader(identity.UserAgent, "AxonHub"))
	headers.Set("X-Msh-Platform", "kimi_code_cli")
	headers.Set("X-Msh-Version", asciiHeader(identity.Version, "development"))
	headers.Set("X-Msh-Device-Name", asciiHeader(identity.Hostname, "unknown"))
	headers.Set("X-Msh-Device-Model", asciiHeader(identity.DeviceModel, "unknown"))
	headers.Set("X-Msh-Os-Version", asciiHeader(identity.OSVersion, "unknown"))
	headers.Set("X-Msh-Device-Id", asciiHeader(identity.DeviceID, "unknown"))
	return headers
}

func asciiHeader(value, fallback string) string {
	var builder strings.Builder
	for _, char := range value {
		if char >= 0x20 && char <= 0x7e {
			builder.WriteRune(char)
		}
	}
	cleaned := strings.TrimSpace(builder.String())
	if cleaned == "" {
		return fallback
	}
	return cleaned
}
