package mcp

import (
	"context"
	"io"
	"log/slog"
	"time"
)

// Server represents the MCP server
type Server struct {
	transport  *Transport
	auth       *Authenticator
	tools      *ToolRegistry
	resources  *ResourceRegistry
	prompts    *PromptRegistry
	logger     *slog.Logger
	timeout    time.Duration
}

// NewServer creates a new MCP server
func NewServer(t *Transport, auth *Authenticator, logger *slog.Logger, timeout time.Duration) *Server {
	return &Server{
		transport:  t,
		auth:      auth,
		tools:     NewToolRegistry(),
		resources: NewResourceRegistry(),
		prompts:   NewPromptRegistry(),
		logger:    logger,
		timeout:   timeout,
	}
}

// RegisterTool registers a tool with the server
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.tools.Register(tool, handler)
}

// Run starts the MCP server
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP server running")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := s.transport.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			s.logger.Error("read error", "error", err)
			continue
		}

		if req == nil {
			continue
		}

		go s.handleRequest(ctx, req)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *JSONRPCRequest) {
	start := time.Now()

	var resp *JSONRPCResponse

	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(ctx, req)
	case "resources/list":
		resp = s.handleResourcesList(req)
	case "resources/read":
		resp = s.handleResourcesRead(ctx, req)
	case "prompts/list":
		resp = s.handlePromptsList(req)
	case "prompts/get":
		resp = s.handlePromptsGet(req)
	case "prompts/call":
		resp = s.handlePromptsCall(ctx, req)
	default:
		resp = s.newErrorResponse(req.ID, ErrCodeInternal, "method not found: "+req.Method)
	}

	if err := s.transport.Write(resp); err != nil {
		s.logger.Error("write error", "error", err)
	}

	s.logger.Info("request handled",
		"method", req.Method,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    "agentra-mcp",
				"version": "0.1.0",
			},
			"capabilities": ServerCapabilities{
				Tools:     ToolCapabilities{ListChanged: true},
				Resources: ResourceCapabilities{ListChanged: true},
				Prompts:   PromptCapabilities{ListChanged: true},
			},
		},
	}
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	tools := s.tools.List()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	params, ok := req.Params["params"].(map[string]any)
	if !ok {
		params = map[string]any{}
	}

	name, ok := req.Params["name"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing tool name")
	}

	toolCtx := ToolContext{}

	result, err := s.tools.Handle(name, toolCtx, params)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			return s.newMCPErrorResponse(req.ID, mcpErr)
		}
		return s.newErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"content": []any{map[string]any{"type": "text", "text": result}}},
	}
}

func (s *Server) handleResourcesList(req *JSONRPCRequest) *JSONRPCResponse {
	resources := s.resources.List()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"resources": resources},
	}
}

func (s *Server) handleResourcesRead(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	uri, ok := req.Params["uri"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing uri")
	}

	toolCtx := ToolContext{}
	content, err := s.resources.Read(uri, toolCtx)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			return s.newMCPErrorResponse(req.ID, mcpErr)
		}
		return s.newErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{"contents": []any{
			map[string]any{"uri": uri, "mimeType": "text/plain", "text": content},
		}},
	}
}

func (s *Server) handlePromptsList(req *JSONRPCRequest) *JSONRPCResponse {
	prompts := s.prompts.List()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"prompts": prompts},
	}
}

func (s *Server) handlePromptsGet(req *JSONRPCRequest) *JSONRPCResponse {
	name, ok := req.Params["name"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing name")
	}

	prompt, ok := s.prompts.Get(name)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeNotFound, "prompt not found")
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"prompt": prompt},
	}
}

func (s *Server) handlePromptsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	name, ok := req.Params["name"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing name")
	}

	arguments, _ := req.Params["arguments"].(map[string]string)

	toolCtx := ToolContext{}
	content, err := s.prompts.Render(name, toolCtx, arguments)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			return s.newMCPErrorResponse(req.ID, mcpErr)
		}
		return s.newErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{"messages": []any{
			map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": content}},
		}},
	}
}

func (s *Server) newErrorResponse(id any, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}

func (s *Server) newMCPErrorResponse(id any, mcpErr *MCPError) *JSONRPCResponse {
	code := ErrCodeInternal
	switch mcpErr.Code {
	case ErrUnauthorized:
		code = ErrCodeUnauthorized
	case ErrForbidden:
		code = ErrCodeForbidden
	case ErrNotFound:
		code = ErrCodeNotFound
	case ErrValidation:
		code = ErrCodeValidation
	case ErrTimeout:
		code = ErrCodeTimeout
	}
	return s.newErrorResponse(id, code, mcpErr.Message)
}