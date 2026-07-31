package service

import "testing"

func TestAccessPolicyDefaultsOpen(t *testing.T) {
	policy, err := newAccessPolicy(func(string) string { return "" })
	if err != nil {
		t.Fatalf("newAccessPolicy() error = %v", err)
	}
	if policy.SignupDisabled || policy.WorkspaceCreationDisabled {
		t.Fatal("default access policy should be open")
	}
	if !policy.AllowsNewSignup("person@example.com") {
		t.Fatal("empty allowlist should permit a new signup")
	}
}

func TestAccessPolicyMatchesExactEmailsAndDomains(t *testing.T) {
	values := map[string]string{
		signupAllowlistEnv: "owner@example.com, @agentra.ai",
	}
	policy, err := newAccessPolicy(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("newAccessPolicy() error = %v", err)
	}

	tests := []struct {
		email string
		want  bool
	}{
		{email: "OWNER@example.com", want: true},
		{email: "member@agentra.ai", want: true},
		{email: "member@sub.agentra.ai", want: false},
		{email: "other@example.com", want: false},
	}
	for _, tt := range tests {
		if got := policy.AllowsNewSignup(tt.email); got != tt.want {
			t.Errorf("AllowsNewSignup(%q) = %v, want %v", tt.email, got, tt.want)
		}
	}
}

func TestAccessPolicySignupDisabledOverridesAllowlist(t *testing.T) {
	values := map[string]string{
		signupDisabledEnv:  "true",
		signupAllowlistEnv: "@agentra.ai",
	}
	policy, err := newAccessPolicy(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("newAccessPolicy() error = %v", err)
	}
	if policy.AllowsNewSignup("member@agentra.ai") {
		t.Fatal("disabled signup must override the allowlist")
	}
}

func TestAccessPolicyRejectsInvalidConfiguration(t *testing.T) {
	tests := []map[string]string{
		{signupDisabledEnv: "sometimes"},
		{workspaceCreationDisabledEnv: "disabled"},
		{signupAllowlistEnv: "not-an-email"},
		{signupAllowlistEnv: "@"},
	}

	for _, values := range tests {
		if _, err := newAccessPolicy(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("newAccessPolicy(%v) expected an error", values)
		}
	}
}
