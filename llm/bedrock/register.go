package bedrock

import (
	"github.com/ldm2060/axonhub/llm/httpclient"
)

// init registers the AWS EventStream decoder.
func init() {
	httpclient.RegisterDecoder("application/vnd.amazon.eventstream", NewAWSEventStreamDecoder)
}
