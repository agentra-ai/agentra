package mcp

import "sync"

// ResourceHandler handles resource reads
type ResourceHandler func(ctx ToolContext, uri string) (string, error)

// ResourceRegistry manages available resources
type ResourceRegistry struct {
	mu       sync.RWMutex
	resources map[string]Resource
	handlers map[string]ResourceHandler
}

// NewResourceRegistry creates a new resource registry
func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources: make(map[string]Resource),
		handlers:  make(map[string]ResourceHandler),
	}
}

// Register registers a resource with its handler
func (r *ResourceRegistry) Register(resource Resource, handler ResourceHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resources[resource.URI] = resource
	r.handlers[resource.URI] = handler
}

// Get retrieves a resource by URI
func (r *ResourceRegistry) Get(uri string) (Resource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resource, ok := r.resources[uri]
	return resource, ok
}

// List returns all registered resources
func (r *ResourceRegistry) List() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resources := make([]Resource, 0, len(r.resources))
	for _, resource := range r.resources {
		resources = append(resources, resource)
	}
	return resources
}

// Read executes a resource handler
func (r *ResourceRegistry) Read(uri string, ctx ToolContext) (string, error) {
	r.mu.RLock()
	handler, ok := r.handlers[uri]
	r.mu.RUnlock()

	if !ok {
		return "", NewNotFoundError("resource not found: " + uri)
	}

	return handler(ctx, uri)
}