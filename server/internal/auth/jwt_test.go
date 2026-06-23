package auth

import (
	"os"
	"strings"
	"testing"
)

// withSecret sets JWT_SECRET for the test and resets the cached secret.
// The cached secret is package-global (sync.Once), so tests must reset
// jwtSecret = nil and force a fresh read of env on the next JWTSecret() call.
func withSecret(t *testing.T, value string) {
	t.Helper()
	old := os.Getenv("JWT_SECRET")
	if value == "" {
		os.Unsetenv("JWT_SECRET")
	} else {
		os.Setenv("JWT_SECRET", value)
	}
	t.Cleanup(func() {
		if old == "" {
			os.Unsetenv("JWT_SECRET")
		} else {
			os.Setenv("JWT_SECRET", old)
		}
		ResetSecretForTesting()
	})
	ResetSecretForTesting()
}



func TestJWTSecret_RejectsPlaceholderDefault(t *testing.T) {
	// Production deploys that forget JWT_SECRET should not silently boot
	// with a well-known key.
	withSecret(t, "agentra-dev-secret-change-in-production")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected JWTSecret() to panic on placeholder default")
		}
	}()
	_ = JWTSecret()
}

func TestJWTSecret_RejectsTooShort(t *testing.T) {
	withSecret(t, "short")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected JWTSecret() to panic on short secret")
		}
	}()
	_ = JWTSecret()
}

func TestJWTSecret_AcceptsStrongSecret(t *testing.T) {
	withSecret(t, "this-is-a-very-long-random-secret-with-32+bytes-of-entropy-12345")

	got := JWTSecret()
	if len(got) < 32 {
		t.Fatalf("expected >= 32 bytes, got %d", len(got))
	}
}

func TestJWTSecret_RejectsEmptyWhenProdEnvSet(t *testing.T) {
	// Even if APP_ENV/ENV=production, an empty JWT_SECRET must panic.
	withSecret(t, "")
	os.Setenv("APP_ENV", "production")
	t.Cleanup(func() { os.Unsetenv("APP_ENV") })

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected JWTSecret() to panic on empty secret in production")
		}
	}()
	_ = JWTSecret()
}

func TestJWTSecret_AllowsEmptyWhenDevAndDevFlag(t *testing.T) {
	// Documented "dev only" escape hatch: setting AGENTRA_ALLOW_INSECURE_JWT=1
	// lets a developer boot with the placeholder secret. This must never
	// be the default and is intended for local hacking.
	withSecret(t, "agentra-dev-secret-change-in-production")
	os.Setenv("AGENTRA_ALLOW_INSECURE_JWT", "1")
	t.Cleanup(func() { os.Unsetenv("AGENTRA_ALLOW_INSECURE_JWT") })

	got := JWTSecret()
	if len(got) == 0 {
		t.Fatal("expected non-empty secret in dev escape hatch")
	}
}

func TestGeneratePATToken_Format(t *testing.T) {
	tok, err := GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok, "mul_") {
		t.Fatalf("PAT must start with mul_, got %q", tok)
	}
	if got := len(tok) - len("mul_"); got != 40 {
		t.Fatalf("PAT must have 40 hex chars, got %d", got)
	}
}

func TestGenerateDaemonToken_Format(t *testing.T) {
	tok, err := GenerateDaemonToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok, "mdt_") {
		t.Fatalf("daemon token must start with mdt_, got %q", tok)
	}
	if got := len(tok) - len("mdt_"); got != 40 {
		t.Fatalf("daemon token must have 40 hex chars, got %d", got)
	}
}
