package handler

import "testing"

func TestRuntimeAdapterContractExposesCompleteKnownLocalProvider(t *testing.T) {
	t.Parallel()

	contract := runtimeAdapterContract("local", "codex")
	if contract == nil {
		t.Fatal("known local provider returned no adapter contract")
	}
	if contract.Version != "v1" || contract.Transport != "cli" {
		t.Fatalf("contract identity = %#v", contract)
	}
	if len(contract.Capabilities) != 14 {
		t.Fatalf("capabilities = %d, want 14", len(contract.Capabilities))
	}
	if got := contract.Capabilities["usage"].Level; got != "native" {
		t.Fatalf("usage level = %q, want native", got)
	}
	if got := contract.Capabilities["artifacts"].Level; got != "unsupported" {
		t.Fatalf("artifacts level = %q, want unsupported", got)
	}
}

func TestRuntimeAdapterContractDoesNotInventUnknownOrCloudCapabilities(t *testing.T) {
	t.Parallel()

	if contract := runtimeAdapterContract("local", "legacy-provider"); contract != nil {
		t.Fatalf("unknown provider contract = %#v, want nil", contract)
	}
	if contract := runtimeAdapterContract("cloud", "claude"); contract != nil {
		t.Fatalf("cloud runtime contract = %#v, want nil", contract)
	}
}
