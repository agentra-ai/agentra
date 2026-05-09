package memory

type MemoryType string

const (
	MemoryTypeLearning    MemoryType = "learning"
	MemoryTypeTaskResult   MemoryType = "task_result"
	MemoryTypeContext      MemoryType = "context"
	MemoryTypePattern      MemoryType = "pattern"
)

type AgentMemory struct {
	ID          string      `json:"id"`
	AgentID     string      `json:"agent_id"`
	WorkspaceID string      `json:"workspace_id"`
	MemoryType  MemoryType  `json:"memory_type"`
	Content     string      `json:"content"`
	Metadata    map[string]any `json:"metadata"`
	IsPrivate   bool        `json:"is_private"`
	Score       float64     `json:"score,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

type TeamMemory struct {
	ID          string      `json:"id"`
	WorkspaceID string      `json:"workspace_id"`
	MemoryType  MemoryType  `json:"memory_type"`
	Content     string      `json:"content"`
	Metadata    map[string]any `json:"metadata"`
	CreatedBy   string      `json:"created_by,omitempty"`
	Score       float64     `json:"score,omitempty"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

type StoreResult struct {
	ID        string     `json:"id"`
	MemoryType MemoryType `json:"memory_type"`
	Content   string     `json:"content"`
	CreatedAt string     `json:"created_at"`
}

type RecallResult struct {
	Memories []MemoryEntry `json:"memories"`
}

type MemoryEntry struct {
	ID         string     `json:"id"`
	MemoryType MemoryType  `json:"memory_type"`
	Content    string     `json:"content"`
	AgentID    string     `json:"agent_id,omitempty"`
	Score      float64    `json:"score"`
	CreatedAt  string     `json:"created_at"`
}
