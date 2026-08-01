package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/agentra-ai/agentra/server/pkg/agent/types"
)

// ProviderType is the stable identifier for a Runtime Adapter v1 provider.
type ProviderType string

const (
	ProviderClaude   ProviderType = "claude"
	ProviderCodex    ProviderType = "codex"
	ProviderOpenCode ProviderType = "opencode"
)

// Capability identifies one independently testable adapter behavior.
type Capability string

const (
	CapabilityDiscover         Capability = "discover"
	CapabilityModels           Capability = "models"
	CapabilityExecute          Capability = "execute"
	CapabilityStream           Capability = "stream"
	CapabilityResume           Capability = "resume"
	CapabilityCancel           Capability = "cancel"
	CapabilityModelSelection   Capability = "model_selection"
	CapabilitySystemPrompt     Capability = "system_prompt"
	CapabilityMaxTurns         Capability = "max_turns"
	CapabilityToolRestrictions Capability = "tool_restrictions"
	CapabilitySkills           Capability = "skills"
	CapabilityMCP              Capability = "mcp"
	CapabilityUsage            Capability = "usage"
	CapabilityArtifacts        Capability = "artifacts"
)

var allCapabilities = []Capability{
	CapabilityDiscover,
	CapabilityModels,
	CapabilityExecute,
	CapabilityStream,
	CapabilityResume,
	CapabilityCancel,
	CapabilityModelSelection,
	CapabilitySystemPrompt,
	CapabilityMaxTurns,
	CapabilityToolRestrictions,
	CapabilitySkills,
	CapabilityMCP,
	CapabilityUsage,
	CapabilityArtifacts,
}

// SupportLevel distinguishes provider-native behavior from behavior supplied
// by the Agentra adapter. Unsupported is explicit and fails before launch.
type SupportLevel string

const (
	SupportNative      SupportLevel = "native"
	SupportAdapter     SupportLevel = "adapter"
	SupportUnsupported SupportLevel = "unsupported"
)

// CapabilitySupport records both the support level and its operational limit.
type CapabilitySupport struct {
	Level  SupportLevel `json:"level"`
	Detail string       `json:"detail,omitempty"`
}

// AdapterDescriptor is the machine-readable Runtime Adapter v1 contract.
type AdapterDescriptor struct {
	Provider     ProviderType                     `json:"provider"`
	Transport    string                           `json:"transport"`
	Capabilities map[Capability]CapabilitySupport `json:"capabilities"`
}

// Discovery describes the CLI binary that will execute tasks.
type Discovery struct {
	Provider   ProviderType `json:"provider"`
	Executable string       `json:"executable"`
	Version    string       `json:"version"`
}

// Model describes a model reported by an adapter. The current CLI adapters do
// not expose a reliable model-list operation and return an explicit error.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// UnsupportedCapabilityError is returned before a provider process starts.
type UnsupportedCapabilityError struct {
	Provider   ProviderType
	Capability Capability
	Option     string
}

func (e *UnsupportedCapabilityError) Error() string {
	if e.Option == "" {
		return fmt.Sprintf("runtime adapter %q does not support %s", e.Provider, e.Capability)
	}
	return fmt.Sprintf("runtime adapter %q does not support %s requested by %s", e.Provider, e.Capability, e.Option)
}

// IsUnsupportedCapability reports whether err represents an explicit adapter
// capability rejection.
func IsUnsupportedCapability(err error) bool {
	var target *UnsupportedCapabilityError
	return errors.As(err, &target)
}

// InvalidExecOptionsError reports an invalid value independent of provider.
type InvalidExecOptionsError struct {
	Option string
	Reason string
}

func (e *InvalidExecOptionsError) Error() string {
	return fmt.Sprintf("invalid execution option %s: %s", e.Option, e.Reason)
}

// KnownAdapters returns descriptors in stable provider order.
func KnownAdapters() []AdapterDescriptor {
	providers := []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode}
	result := make([]AdapterDescriptor, 0, len(providers))
	for _, provider := range providers {
		descriptor, _ := DescriptorFor(provider)
		result = append(result, descriptor)
	}
	return result
}

// DescriptorFor returns a defensive copy of a known adapter descriptor.
func DescriptorFor(provider ProviderType) (AdapterDescriptor, bool) {
	descriptor, ok := adapterDescriptors[provider]
	if !ok {
		return AdapterDescriptor{}, false
	}
	return cloneDescriptor(descriptor), true
}

// Supports reports whether a capability is available natively or through the
// Agentra adapter.
func (d AdapterDescriptor) Supports(capability Capability) bool {
	support, ok := d.Capabilities[capability]
	return ok && support.Level != SupportUnsupported
}

// Validate verifies that the descriptor is complete and contains no unknown
// support levels. A missing capability is a contract error, not unsupported.
func (d AdapterDescriptor) Validate() error {
	if d.Provider == "" {
		return fmt.Errorf("adapter provider is required")
	}
	if d.Transport == "" {
		return fmt.Errorf("adapter %q transport is required", d.Provider)
	}
	for _, capability := range allCapabilities {
		support, ok := d.Capabilities[capability]
		if !ok {
			return fmt.Errorf("adapter %q does not declare capability %q", d.Provider, capability)
		}
		switch support.Level {
		case SupportNative, SupportAdapter, SupportUnsupported:
		default:
			return fmt.Errorf("adapter %q capability %q has invalid support level %q", d.Provider, capability, support.Level)
		}
	}
	return nil
}

// CapabilityNames returns the complete v1 capability vocabulary in stable
// order for matrix generation and conformance checks.
func CapabilityNames() []Capability {
	result := append([]Capability(nil), allCapabilities...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// ValidateExecOptions rejects unsupported or malformed options before a child
// process starts. Adapters must call it at the beginning of Execute.
func ValidateExecOptions(descriptor AdapterDescriptor, opts types.ExecOptions) error {
	if err := descriptor.Validate(); err != nil {
		return err
	}
	if opts.Timeout < 0 {
		return &InvalidExecOptionsError{Option: "timeout", Reason: "must not be negative"}
	}
	if opts.MaxTurns < 0 {
		return &InvalidExecOptionsError{Option: "max_turns", Reason: "must not be negative"}
	}
	for _, tool := range opts.Tools {
		if strings.TrimSpace(tool) == "" {
			return &InvalidExecOptionsError{Option: "tools", Reason: "must not contain an empty tool name"}
		}
	}

	requested := []struct {
		active     bool
		capability Capability
		option     string
	}{
		{opts.Model != "", CapabilityModelSelection, "model"},
		{opts.SystemPrompt != "", CapabilitySystemPrompt, "system_prompt"},
		{opts.MaxTurns > 0, CapabilityMaxTurns, "max_turns"},
		{opts.ResumeSessionID != "", CapabilityResume, "resume_session_id"},
		{len(opts.Tools) > 0, CapabilityToolRestrictions, "tools"},
	}
	for _, request := range requested {
		if request.active && !descriptor.Supports(request.capability) {
			return &UnsupportedCapabilityError{
				Provider:   descriptor.Provider,
				Capability: request.capability,
				Option:     request.option,
			}
		}
	}
	return nil
}

func discoverCLI(ctx context.Context, provider ProviderType, configuredPath, defaultPath string) (Discovery, error) {
	executable := configuredPath
	if executable == "" {
		executable = defaultPath
	}
	resolved, err := exec.LookPath(executable)
	if err != nil {
		return Discovery{}, fmt.Errorf("%s executable not found at %q: %w", provider, executable, err)
	}
	version, err := DetectVersion(ctx, resolved)
	if err != nil {
		return Discovery{}, fmt.Errorf("detect %s version: %w", provider, err)
	}
	return Discovery{Provider: provider, Executable: resolved, Version: version}, nil
}

func unsupportedModels(provider ProviderType) ([]Model, error) {
	return nil, &UnsupportedCapabilityError{Provider: provider, Capability: CapabilityModels}
}

func cloneDescriptor(descriptor AdapterDescriptor) AdapterDescriptor {
	cloned := descriptor
	cloned.Capabilities = make(map[Capability]CapabilitySupport, len(descriptor.Capabilities))
	for capability, support := range descriptor.Capabilities {
		cloned.Capabilities[capability] = support
	}
	return cloned
}

func descriptor(provider ProviderType, native, adapter []Capability, details map[Capability]string) AdapterDescriptor {
	capabilities := make(map[Capability]CapabilitySupport, len(allCapabilities))
	for _, capability := range allCapabilities {
		capabilities[capability] = CapabilitySupport{Level: SupportUnsupported}
	}
	for _, capability := range native {
		capabilities[capability] = CapabilitySupport{Level: SupportNative, Detail: details[capability]}
	}
	for _, capability := range adapter {
		capabilities[capability] = CapabilitySupport{Level: SupportAdapter, Detail: details[capability]}
	}
	return AdapterDescriptor{Provider: provider, Transport: "cli", Capabilities: capabilities}
}

var adapterDescriptors = map[ProviderType]AdapterDescriptor{
	ProviderClaude: descriptor(
		ProviderClaude,
		[]Capability{
			CapabilityExecute,
			CapabilityStream,
			CapabilityResume,
			CapabilityModelSelection,
			CapabilitySystemPrompt,
			CapabilityMaxTurns,
			CapabilityToolRestrictions,
		},
		[]Capability{CapabilityDiscover, CapabilityCancel, CapabilitySkills},
		map[Capability]string{
			CapabilityCancel: "context cancellation terminates the CLI process; descendant process cleanup is not yet guaranteed",
			CapabilitySkills: "skills are installed into the per-task .claude/skills directory before launch",
		},
	),
	ProviderCodex: descriptor(
		ProviderCodex,
		[]Capability{
			CapabilityExecute,
			CapabilityStream,
			CapabilityResume,
			CapabilityModelSelection,
			CapabilitySystemPrompt,
		},
		[]Capability{CapabilityDiscover, CapabilityCancel, CapabilitySkills},
		map[Capability]string{
			CapabilityCancel: "context cancellation terminates app-server; descendant process cleanup is not yet guaranteed",
			CapabilitySkills: "skills are installed into the isolated per-task CODEX_HOME before launch",
		},
	),
	ProviderOpenCode: descriptor(
		ProviderOpenCode,
		[]Capability{
			CapabilityExecute,
			CapabilityStream,
			CapabilityResume,
			CapabilityModelSelection,
			CapabilitySystemPrompt,
		},
		[]Capability{CapabilityDiscover, CapabilityCancel, CapabilitySkills},
		map[Capability]string{
			CapabilityCancel: "context cancellation terminates the CLI process; descendant process cleanup is not yet guaranteed",
			CapabilitySkills: "skills are installed into the per-task .config/opencode/skills directory before launch",
		},
	),
}
