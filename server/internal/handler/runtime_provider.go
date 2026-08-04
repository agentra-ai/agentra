package handler

import (
	"fmt"
	"strings"

	runtimeagent "github.com/agentra-ai/agentra/server/pkg/agent"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// validateLocalRuntimeProvider returns the canonical Runtime Adapter v1
// provider. Local runtimes cannot register an uncontracted implementation.
func validateLocalRuntimeProvider(raw string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(raw))
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	if _, ok := runtimeagent.DescriptorFor(runtimeagent.ProviderType(provider)); !ok {
		return "", fmt.Errorf("unsupported local runtime provider %q", provider)
	}
	return provider, nil
}

// canonicalAgentProvider derives an agent's provider from its selected
// runtime. A client may repeat the provider for compatibility, but it may not
// contradict the runtime and create an unroutable agent.
func canonicalAgentProvider(runtime db.AgentRuntime, requested string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(runtime.Provider))
	if strings.EqualFold(strings.TrimSpace(runtime.RuntimeMode), "local") {
		var err error
		provider, err = validateLocalRuntimeProvider(provider)
		if err != nil {
			return "", err
		}
	} else if provider == "" {
		return "", fmt.Errorf("runtime provider is required")
	}

	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" && requested != provider {
		return "", fmt.Errorf("provider %q does not match selected runtime provider %q", requested, provider)
	}
	return provider, nil
}

// validateLoopRuntime fails before an Engineering Loop row or task is
// created when a local runtime cannot enforce the stage budgets and tool
// restrictions required by the loop contract.
func validateLoopRuntime(runtime db.AgentRuntime) error {
	if !strings.EqualFold(strings.TrimSpace(runtime.RuntimeMode), "local") {
		return nil
	}
	provider, err := validateLocalRuntimeProvider(runtime.Provider)
	if err != nil {
		return err
	}
	descriptor, _ := runtimeagent.DescriptorFor(runtimeagent.ProviderType(provider))
	return runtimeagent.ValidateExecOptions(descriptor, runtimeagent.ExecOptions{
		SystemPrompt: "engineering-loop-stage",
		MaxTurns:     1,
		Tools:        []string{"required-stage-tool"},
	})
}
