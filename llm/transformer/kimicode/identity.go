package kimicode

import (
	"errors"
	"net/http"
	"strings"
)

const platform = "kimi_code_cli"

// Identity is the stable host and device identity Kimi Code requires for every
// OAuth, catalog, and inference request. Its values are resolved by the host
// application; this package deliberately has no dependency on internal code.
type Identity struct {
	UserAgentProduct string
	Version          string
	UserAgentSuffix  string
	Hostname         string
	DeviceModel      string
	OSVersion        string
	DeviceID         string
}

func ValidateIdentity(identity Identity) error {
	if requiredASCIIHeader(identity.UserAgentProduct) == "" {
		return errors.New("Kimi identity product must be a non-empty ASCII string")
	}
	if requiredASCIIHeader(identity.Version) == "" {
		return errors.New("Kimi identity version must be a non-empty ASCII string")
	}
	return nil
}

// BuildIdentityHeaders returns the Kimi Code identity headers after removing
// non-printable bytes. Product and version are required; device values fall
// back to a safe non-empty value when host inspection returns unusable text.
func BuildIdentityHeaders(identity Identity) (http.Header, error) {
	if err := ValidateIdentity(identity); err != nil {
		return nil, err
	}
	product := requiredASCIIHeader(identity.UserAgentProduct)
	version := requiredASCIIHeader(identity.Version)
	userAgent := product + "/" + version
	if suffix := asciiHeader(identity.UserAgentSuffix, ""); suffix != "" {
		userAgent += " (" + suffix + ")"
	}

	headers := make(http.Header)
	headers.Set("User-Agent", userAgent)
	headers.Set("X-Msh-Platform", platform)
	headers.Set("X-Msh-Version", version)
	headers.Set("X-Msh-Device-Name", asciiHeader(identity.Hostname, "unknown"))
	headers.Set("X-Msh-Device-Model", asciiHeader(identity.DeviceModel, "unknown"))
	headers.Set("X-Msh-Os-Version", asciiHeader(identity.OSVersion, "unknown"))
	headers.Set("X-Msh-Device-Id", asciiHeader(identity.DeviceID, "unknown"))
	return headers, nil
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

func requiredASCIIHeader(value string) string {
	return asciiHeader(value, "")
}
