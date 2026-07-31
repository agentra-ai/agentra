package service

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	signupDisabledEnv            = "AGENTRA_SIGNUP_DISABLED"
	signupAllowlistEnv           = "AGENTRA_SIGNUP_ALLOWLIST"
	workspaceCreationDisabledEnv = "AGENTRA_WORKSPACE_CREATION_DISABLED"
)

// AccessPolicy controls who may create an account and whether users may create
// workspaces. Pre-provisioned users can always sign in, which keeps invitations
// usable when public signup is disabled.
type AccessPolicy struct {
	SignupDisabled            bool
	WorkspaceCreationDisabled bool
	allowedEmails             map[string]struct{}
	allowedDomains            map[string]struct{}
}

// NewAccessPolicyFromEnv reads the process-wide access policy once at startup.
func NewAccessPolicyFromEnv() (AccessPolicy, error) {
	return newAccessPolicy(os.Getenv)
}

func newAccessPolicy(getenv func(string) string) (AccessPolicy, error) {
	signupDisabled, err := parseOptionalBool(signupDisabledEnv, getenv(signupDisabledEnv))
	if err != nil {
		return AccessPolicy{}, err
	}
	workspaceCreationDisabled, err := parseOptionalBool(workspaceCreationDisabledEnv, getenv(workspaceCreationDisabledEnv))
	if err != nil {
		return AccessPolicy{}, err
	}

	policy := AccessPolicy{
		SignupDisabled:            signupDisabled,
		WorkspaceCreationDisabled: workspaceCreationDisabled,
		allowedEmails:             make(map[string]struct{}),
		allowedDomains:            make(map[string]struct{}),
	}

	for _, rawEntry := range strings.Split(getenv(signupAllowlistEnv), ",") {
		entry := strings.ToLower(strings.TrimSpace(rawEntry))
		if entry == "" {
			continue
		}

		if strings.HasPrefix(entry, "@") {
			domain := strings.TrimPrefix(entry, "@")
			if !validDomain(domain) {
				return AccessPolicy{}, fmt.Errorf("%s contains invalid domain entry %q", signupAllowlistEnv, rawEntry)
			}
			policy.allowedDomains[domain] = struct{}{}
			continue
		}

		local, domain, ok := strings.Cut(entry, "@")
		if !ok || local == "" || !validDomain(domain) {
			return AccessPolicy{}, fmt.Errorf("%s contains invalid email entry %q", signupAllowlistEnv, rawEntry)
		}
		policy.allowedEmails[entry] = struct{}{}
	}

	return policy, nil
}

func parseOptionalBool(name, value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func validDomain(domain string) bool {
	return domain != "" &&
		!strings.ContainsAny(domain, "@, \t\r\n") &&
		!strings.HasPrefix(domain, ".") &&
		!strings.HasSuffix(domain, ".")
}

// AllowsNewSignup reports whether an email that is not already provisioned may
// create a new account. An empty allowlist permits all valid email addresses.
func (p AccessPolicy) AllowsNewSignup(email string) bool {
	if p.SignupDisabled {
		return false
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if len(p.allowedEmails) == 0 && len(p.allowedDomains) == 0 {
		return true
	}
	if _, ok := p.allowedEmails[email]; ok {
		return true
	}

	_, domain, ok := strings.Cut(email, "@")
	if !ok {
		return false
	}
	_, ok = p.allowedDomains[domain]
	return ok
}
