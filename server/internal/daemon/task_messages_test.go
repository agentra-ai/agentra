package daemon

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func TestSplitTaskMessageContentBoundsAndPreservesUTF8(t *testing.T) {
	input := strings.Repeat("界", protocol.TaskMessageFieldBytes)
	chunks := splitTaskMessageContent(input)
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want multiple", len(chunks))
	}
	var joined strings.Builder
	for i, chunk := range chunks {
		if len(chunk) > protocol.TaskMessageFieldBytes {
			t.Fatalf("chunk %d = %d bytes", i, len(chunk))
		}
		if !utf8.ValidString(chunk) {
			t.Fatalf("chunk %d is invalid UTF-8", i)
		}
		joined.WriteString(chunk)
	}
	if joined.String() != input {
		t.Fatal("split content did not round trip")
	}
}
