package mcp

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params map[string]any  `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID     any             `json:"id"`
	Result any             `json:"result,omitempty"`
	Error  *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    any            `json:"data,omitempty"`
}

// ServerCapabilities represents MCP Server capabilities
type ServerCapabilities struct {
	Tools    ToolCapabilities    `json:"tools"`
	Resources ResourceCapabilities `json:"resources"`
	Prompts  PromptCapabilities   `json:"prompts"`
}

// ToolCapabilities represents tools capability
type ToolCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// ResourceCapabilities represents resources capability
type ResourceCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// PromptCapabilities represents prompts capability
type PromptCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolInputSchema represents a tool's input schema
type ToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}
