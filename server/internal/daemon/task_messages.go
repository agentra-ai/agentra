package daemon

import (
	"strings"
	"unicode/utf8"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

// splitTaskMessageContent keeps every message within the server ingestion
// contract without cutting a UTF-8 rune in half.
func splitTaskMessageContent(content string) []string {
	content = strings.ToValidUTF8(content, "\uFFFD")
	if content == "" {
		return nil
	}

	chunks := make([]string, 0, len(content)/protocol.TaskMessageFieldBytes+1)
	for len(content) > protocol.TaskMessageFieldBytes {
		end := protocol.TaskMessageFieldBytes
		for end > 0 && !utf8.RuneStart(content[end]) {
			end--
		}
		if end == 0 {
			end = protocol.TaskMessageFieldBytes
		}
		chunks = append(chunks, content[:end])
		content = content[end:]
	}
	if content != "" {
		chunks = append(chunks, content)
	}
	return chunks
}
