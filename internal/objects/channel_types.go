package objects

// CodingChannelTypes defines channel types that support coding agent clients
var CodingChannelTypes = map[string]bool{
	"claudecode":             true,
	"codex":                  true,
	"github_copilot":         true,
	"antigravity":            true,
	"opencode_go":            true,
	"opencode_go_anthropic":  true,
	"moonshot_coding":        true,
}

// IsCodingChannel checks if a channel type is a coding channel
func IsCodingChannel(channelType string) bool {
	return CodingChannelTypes[channelType]
}
