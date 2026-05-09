package mcp

import "sync"

// PromptHandler handles prompt rendering
type PromptHandler func(ctx ToolContext, arguments map[string]string) (string, error)

// PromptRegistry manages available prompts
type PromptRegistry struct {
	mu      sync.RWMutex
	prompts map[string]Prompt
	handlers map[string]PromptHandler
}

// NewPromptRegistry creates a new prompt registry
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		prompts:  make(map[string]Prompt),
		handlers: make(map[string]PromptHandler),
	}
}

// Register registers a prompt with its handler
func (r *PromptRegistry) Register(prompt Prompt, handler PromptHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prompts[prompt.Name] = prompt
	r.handlers[prompt.Name] = handler
}

// Get retrieves a prompt by name
func (r *PromptRegistry) Get(name string) (Prompt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prompt, ok := r.prompts[name]
	return prompt, ok
}

// List returns all registered prompts
func (r *PromptRegistry) List() []Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prompts := make([]Prompt, 0, len(r.prompts))
	for _, prompt := range r.prompts {
		prompts = append(prompts, prompt)
	}
	return prompts
}

// Render executes a prompt handler
func (r *PromptRegistry) Render(name string, ctx ToolContext, arguments map[string]string) (string, error) {
	r.mu.RLock()
	handler, ok := r.handlers[name]
	r.mu.RUnlock()

	if !ok {
		return "", NewNotFoundError("prompt not found: " + name)
	}

	return handler(ctx, arguments)
}