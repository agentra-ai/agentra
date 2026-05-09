package mcp

import "sync"

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx ToolContext, params map[string]any) (any, error)

// ToolContext contains context for a tool call
type ToolContext struct {
	WorkspaceID string
	UserID      string
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	handlers map[string]ToolHandler
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
	}
}

// Register registers a tool with its handler
func (r *ToolRegistry) Register(tool Tool, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
	r.handlers[tool.Name] = handler
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Handle executes a tool handler
func (r *ToolRegistry) Handle(name string, ctx ToolContext, params map[string]any) (any, error) {
	r.mu.RLock()
	handler, ok := r.handlers[name]
	r.mu.RUnlock()

	if !ok {
		return nil, NewNotFoundError("tool not found: " + name)
	}

	return handler(ctx, params)
}
