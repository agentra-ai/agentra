package gateway

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

type capturedLogFrame struct {
	taskID  string
	runID   string
	seq     int
	stream  string
	content string
}

type capturingLogSender struct {
	frames []capturedLogFrame
	errAt  int
}

func (s *capturingLogSender) SendTaskLogs(taskID, runID string, seq int, stream, content string) error {
	if s.errAt > 0 && seq == s.errAt {
		return fmt.Errorf("send failed")
	}
	s.frames = append(s.frames, capturedLogFrame{taskID: taskID, runID: runID, seq: seq, stream: stream, content: content})
	return nil
}

func TestTaskLogEmitterChunksAndSequencesAcrossStreams(t *testing.T) {
	sender := &capturingLogSender{}
	tail := newBoundedTailBuffer(protocol.GatewayTaskResultBytes)
	emitter := &taskLogEmitter{sender: sender, taskID: "task-1", runID: "run-1", tail: tail}

	stdout := strings.Repeat("a", protocol.GatewayLogChunkBytes+7)
	if _, err := emitter.writer(protocol.GatewayStreamStdout).Write([]byte(stdout)); err != nil {
		t.Fatal(err)
	}
	if _, err := emitter.writer(protocol.GatewayStreamStderr).Write([]byte("boom")); err != nil {
		t.Fatal(err)
	}

	if len(sender.frames) != 3 {
		t.Fatalf("frames = %d, want 3", len(sender.frames))
	}
	for i, frame := range sender.frames {
		if frame.runID != "run-1" {
			t.Fatalf("frame %d run_id = %q", i, frame.runID)
		}
		if frame.seq != i+1 {
			t.Fatalf("frame %d seq = %d, want %d", i, frame.seq, i+1)
		}
		if len(frame.content) > protocol.GatewayLogChunkBytes {
			t.Fatalf("frame %d content = %d bytes", i, len(frame.content))
		}
	}
	if sender.frames[2].stream != protocol.GatewayStreamStderr {
		t.Fatalf("last stream = %q, want stderr", sender.frames[2].stream)
	}
	if got := tail.String(); got != stdout+"boom" {
		t.Fatalf("tail length = %d, want %d", len(got), len(stdout)+4)
	}
}

func TestBoundedTailBufferKeepsNewestBytes(t *testing.T) {
	buffer := newBoundedTailBuffer(5)
	_, _ = buffer.Write([]byte("abc"))
	_, _ = buffer.Write([]byte("defg"))
	if got := buffer.String(); got != "cdefg" {
		t.Fatalf("tail = %q, want cdefg", got)
	}
	_, _ = buffer.Write([]byte("0123456789"))
	if got := buffer.String(); got != "56789" {
		t.Fatalf("tail = %q, want 56789", got)
	}
}
