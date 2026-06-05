package openai

// toolCallWhitespaceGuard tracks consecutive whitespace in tool-call argument
// streams and suppresses a tool call after a long whitespace run. Some
// OpenAI-compatible providers can get stuck emitting whitespace-only JSON
// argument deltas forever; dropping that tool call prevents unbounded buffering
// and invalid downstream Anthropic tool_use streams.
type toolCallWhitespaceGuard struct {
	whitespaceByIndex map[int]int
	abortedByIndex    map[int]bool
}

func newToolCallWhitespaceGuard() *toolCallWhitespaceGuard {
	return &toolCallWhitespaceGuard{
		whitespaceByIndex: make(map[int]int),
		abortedByIndex:    make(map[int]bool),
	}
}

func (g *toolCallWhitespaceGuard) allow(index int, args string) bool {
	if g == nil {
		return true
	}

	if g.abortedByIndex[index] {
		return false
	}

	if args == "" {
		return true
	}

	count := g.whitespaceByIndex[index]
	for _, ch := range args {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\v' || ch == '\f' {
			count++
			if count >= infiniteWhitespaceThreshold {
				g.whitespaceByIndex[index] = count
				g.abortedByIndex[index] = true
				return false
			}
			continue
		}

		count = 0
	}

	g.whitespaceByIndex[index] = count
	return true
}
