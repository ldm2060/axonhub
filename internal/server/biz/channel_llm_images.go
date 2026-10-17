package biz

import "github.com/ldm2060/axonhub/llm"

func isNativeImageFormat(format string) bool {
	return format == llm.APIFormatOpenAIImageGeneration.String() || format == llm.APIFormatOpenAIImageEdit.String()
}
