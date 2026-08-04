package protocol

const (
	// TaskMessageBatchSize and TaskMessageFieldBytes define the shared daemon
	// and server ingestion contract for durable task output.
	TaskMessageBatchSize  = 100
	TaskMessageFieldBytes = 64 * 1024

	// GatewayLogChunkBytes bounds one WebSocket log frame. Keeping frames small
	// gives the socket natural backpressure without allowing an agent process to
	// force large allocations in the gateway or server.
	GatewayLogChunkBytes = 32 * 1024

	// GatewayTaskResultBytes bounds the diagnostic tail attached to terminal
	// task events. The complete log remains available through task messages.
	GatewayTaskResultBytes = 256 * 1024
)

const (
	GatewayStreamStdout = "stdout"
	GatewayStreamStderr = "stderr"
)

// GatewayEnvelope is decoded first so each event can then be unmarshaled into
// its exact schema. Avoid map[string]any here: JSON numbers become float64 and
// silently broke exit_code handling in the original gateway protocol.
type GatewayEnvelope struct {
	Type string `json:"type"`
}

type GatewayTaskDispatchMessage struct {
	Type   string         `json:"type"`
	TaskID string         `json:"task_id"`
	RunID  string         `json:"run_id"`
	Config map[string]any `json:"config"`
}

type GatewayTaskCancelMessage struct {
	Type   string `json:"type"`
	TaskID string `json:"task_id"`
}

type GatewayTaskDispatchedMessage struct {
	Type        string `json:"type"`
	TaskID      string `json:"task_id"`
	RunID       string `json:"run_id"`
	ContainerID string `json:"container_id"`
}

// GatewayTaskLogsMessage is the canonical gateway log frame. Seq is
// monotonically increasing per task and makes retries idempotent.
type GatewayTaskLogsMessage struct {
	Type    string `json:"type"`
	TaskID  string `json:"task_id"`
	RunID   string `json:"run_id"`
	Seq     int    `json:"seq"`
	Stream  string `json:"stream"`
	Content string `json:"content"`
}

type GatewayTaskCompletedMessage struct {
	Type     string `json:"type"`
	TaskID   string `json:"task_id"`
	RunID    string `json:"run_id"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
}

type GatewayTaskFailedMessage struct {
	Type      string `json:"type"`
	TaskID    string `json:"task_id"`
	RunID     string `json:"run_id"`
	Error     string `json:"error"`
	Retryable bool   `json:"retryable"`
}

type GatewayHeartbeatMessage struct {
	Type string `json:"type"`
}
