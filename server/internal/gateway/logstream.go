package gateway

import (
	"fmt"
	"strings"
	"sync"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

type taskLogSender interface {
	SendTaskLogs(taskID, runID string, seq int, stream, content string) error
}

// taskLogEmitter serializes stdout and stderr onto one monotonic per-task
// cursor. Sending is synchronous: socket pressure propagates to Docker's log
// reader instead of accumulating an unbounded queue in memory.
type taskLogEmitter struct {
	mu     sync.Mutex
	sender taskLogSender
	taskID string
	runID  string
	seq    int
	tail   *boundedTailBuffer
}

func (e *taskLogEmitter) writer(stream string) *taskLogWriter {
	return &taskLogWriter{emitter: e, stream: stream}
}

type taskLogWriter struct {
	emitter *taskLogEmitter
	stream  string
}

func (w *taskLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.emitter == nil || w.emitter.sender == nil {
		return 0, fmt.Errorf("gateway log sender is not configured")
	}

	w.emitter.mu.Lock()
	defer w.emitter.mu.Unlock()

	written := 0
	for len(p) > 0 {
		size := min(len(p), protocol.GatewayLogChunkBytes)
		chunk := p[:size]
		content := strings.ToValidUTF8(string(chunk), "?")

		w.emitter.seq++
		if w.emitter.tail != nil {
			_, _ = w.emitter.tail.Write([]byte(content))
		}
		if err := w.emitter.sender.SendTaskLogs(w.emitter.taskID, w.emitter.runID, w.emitter.seq, w.stream, content); err != nil {
			return written, err
		}
		written += size
		p = p[size:]
	}
	return written, nil
}

// boundedTailBuffer retains only the newest max bytes for the terminal task
// event. The durable task_message stream remains the source of truth.
type boundedTailBuffer struct {
	mu   sync.Mutex
	max  int
	data []byte
}

func newBoundedTailBuffer(maxBytes int) *boundedTailBuffer {
	return &boundedTailBuffer{max: maxBytes}
}

func (b *boundedTailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max <= 0 {
		return len(p), nil
	}
	if len(p) >= b.max {
		b.data = append(b.data[:0], p[len(p)-b.max:]...)
		return len(p), nil
	}
	if overflow := len(b.data) + len(p) - b.max; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *boundedTailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...))
}
