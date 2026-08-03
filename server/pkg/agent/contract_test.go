package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRuntimeAdapterDescriptorsConform(t *testing.T) {
	t.Parallel()

	expected := []struct {
		provider ProviderType
		native   []Capability
		adapter  []Capability
	}{
		{
			provider: ProviderClaude,
			native: []Capability{
				CapabilityExecute,
				CapabilityStream,
				CapabilityResume,
				CapabilityModelSelection,
				CapabilitySystemPrompt,
				CapabilityMaxTurns,
				CapabilityToolRestrictions,
			},
			adapter: []Capability{CapabilityDiscover, CapabilityCancel, CapabilitySkills},
		},
		{
			provider: ProviderCodex,
			native: []Capability{
				CapabilityExecute,
				CapabilityStream,
				CapabilityResume,
				CapabilityModelSelection,
				CapabilitySystemPrompt,
			},
			adapter: []Capability{CapabilityDiscover, CapabilityCancel, CapabilitySkills},
		},
		{
			provider: ProviderOpenCode,
			native: []Capability{
				CapabilityExecute,
				CapabilityStream,
				CapabilityResume,
				CapabilityModelSelection,
				CapabilitySystemPrompt,
			},
			adapter: []Capability{CapabilityDiscover, CapabilityCancel, CapabilitySkills},
		},
	}

	descriptors := KnownAdapters()
	if len(descriptors) != len(expected) {
		t.Fatalf("KnownAdapters() returned %d descriptors, want %d", len(descriptors), len(expected))
	}

	capabilities := CapabilityNames()
	if len(capabilities) != len(allCapabilities) {
		t.Fatalf("CapabilityNames() returned %d capabilities, want %d", len(capabilities), len(allCapabilities))
	}

	for i, want := range expected {
		want := want
		descriptor := descriptors[i]
		t.Run(string(want.provider), func(t *testing.T) {
			t.Parallel()

			if descriptor.Provider != want.provider {
				t.Fatalf("provider = %q, want %q", descriptor.Provider, want.provider)
			}
			if descriptor.Transport != "cli" {
				t.Fatalf("transport = %q, want cli", descriptor.Transport)
			}
			if err := descriptor.Validate(); err != nil {
				t.Fatalf("descriptor validation failed: %v", err)
			}
			if len(descriptor.Capabilities) != len(capabilities) {
				t.Fatalf("descriptor declares %d capabilities, want %d", len(descriptor.Capabilities), len(capabilities))
			}

			native := capabilitySet(want.native)
			adapter := capabilitySet(want.adapter)
			for _, capability := range capabilities {
				wantLevel := SupportUnsupported
				if native[capability] {
					wantLevel = SupportNative
				} else if adapter[capability] {
					wantLevel = SupportAdapter
				}
				if got := descriptor.Capabilities[capability].Level; got != wantLevel {
					t.Errorf("capability %q level = %q, want %q", capability, got, wantLevel)
				}
			}
		})
	}
}

func TestRuntimeAdapterBackendsExposeRegisteredDescriptors(t *testing.T) {
	t.Parallel()

	for _, registered := range KnownAdapters() {
		registered := registered
		t.Run(string(registered.Provider), func(t *testing.T) {
			t.Parallel()

			backend, err := New(string(registered.Provider), Config{})
			if err != nil {
				t.Fatalf("New(%q) error: %v", registered.Provider, err)
			}
			if got := backend.Descriptor(); !reflect.DeepEqual(got, registered) {
				t.Fatalf("backend descriptor differs from registry\ngot:  %#v\nwant: %#v", got, registered)
			}

			models, err := backend.Models(context.Background())
			if models != nil {
				t.Fatalf("Models() = %#v, want nil", models)
			}
			assertUnsupportedCapability(t, err, registered.Provider, CapabilityModels, "")
		})
	}
}

func TestDescriptorForReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	descriptor, ok := DescriptorFor(ProviderClaude)
	if !ok {
		t.Fatal("Claude descriptor not found")
	}
	descriptor.Capabilities[CapabilityExecute] = CapabilitySupport{Level: SupportUnsupported}

	fresh, ok := DescriptorFor(ProviderClaude)
	if !ok {
		t.Fatal("Claude descriptor not found after mutation")
	}
	if got := fresh.Capabilities[CapabilityExecute].Level; got != SupportNative {
		t.Fatalf("registry was mutated through returned descriptor: execute = %q", got)
	}
}

func TestValidateExecOptionsRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	descriptor, _ := DescriptorFor(ProviderClaude)
	tests := []struct {
		name       string
		opts       ExecOptions
		wantOption string
	}{
		{name: "negative timeout", opts: ExecOptions{Timeout: -time.Second}, wantOption: "timeout"},
		{name: "negative max turns", opts: ExecOptions{MaxTurns: -1}, wantOption: "max_turns"},
		{name: "empty tool", opts: ExecOptions{Tools: []string{"Read", "  "}}, wantOption: "tools"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateExecOptions(descriptor, tt.opts)
			var invalid *InvalidExecOptionsError
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %T %v, want InvalidExecOptionsError", err, err)
			}
			if invalid.Option != tt.wantOption {
				t.Fatalf("invalid option = %q, want %q", invalid.Option, tt.wantOption)
			}
		})
	}
}

func TestValidateExecOptionsCapabilityMatrix(t *testing.T) {
	t.Parallel()

	requests := []struct {
		name       string
		opts       ExecOptions
		capability Capability
		option     string
	}{
		{name: "model", opts: ExecOptions{Model: "test-model"}, capability: CapabilityModelSelection, option: "model"},
		{name: "system prompt", opts: ExecOptions{SystemPrompt: "system"}, capability: CapabilitySystemPrompt, option: "system_prompt"},
		{name: "max turns", opts: ExecOptions{MaxTurns: 2}, capability: CapabilityMaxTurns, option: "max_turns"},
		{name: "resume", opts: ExecOptions{ResumeSessionID: "session-1"}, capability: CapabilityResume, option: "resume_session_id"},
		{name: "tools", opts: ExecOptions{Tools: []string{"Read"}}, capability: CapabilityToolRestrictions, option: "tools"},
	}

	for _, descriptor := range KnownAdapters() {
		descriptor := descriptor
		t.Run(string(descriptor.Provider), func(t *testing.T) {
			t.Parallel()
			for _, request := range requests {
				request := request
				t.Run(request.name, func(t *testing.T) {
					t.Parallel()

					err := ValidateExecOptions(descriptor, request.opts)
					if descriptor.Supports(request.capability) {
						if err != nil {
							t.Fatalf("supported capability rejected: %v", err)
						}
						return
					}
					assertUnsupportedCapability(t, err, descriptor.Provider, request.capability, request.option)
				})
			}
		})
	}
}

func TestUnsupportedOptionsFailBeforeBinaryLookup(t *testing.T) {
	t.Parallel()

	for _, provider := range []ProviderType{ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			backend, err := New(string(provider), Config{ExecutablePath: "/agentra-conformance/missing-runtime"})
			if err != nil {
				t.Fatalf("New(%q) error: %v", provider, err)
			}

			_, err = backend.Execute(context.Background(), "prompt", ExecOptions{MaxTurns: 1})
			assertUnsupportedCapability(t, err, provider, CapabilityMaxTurns, "max_turns")

			_, err = backend.Execute(context.Background(), "prompt", ExecOptions{Tools: []string{"Read"}})
			assertUnsupportedCapability(t, err, provider, CapabilityToolRestrictions, "tools")
		})
	}
}

func capabilitySet(capabilities []Capability) map[Capability]bool {
	result := make(map[Capability]bool, len(capabilities))
	for _, capability := range capabilities {
		result[capability] = true
	}
	return result
}

func assertUnsupportedCapability(t *testing.T, err error, provider ProviderType, capability Capability, option string) {
	t.Helper()

	var unsupported *UnsupportedCapabilityError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %T %v, want UnsupportedCapabilityError", err, err)
	}
	if unsupported.Provider != provider {
		t.Errorf("unsupported provider = %q, want %q", unsupported.Provider, provider)
	}
	if unsupported.Capability != capability {
		t.Errorf("unsupported capability = %q, want %q", unsupported.Capability, capability)
	}
	if unsupported.Option != option {
		t.Errorf("unsupported option = %q, want %q", unsupported.Option, option)
	}
}
