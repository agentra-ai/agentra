package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
)

// PlaceholderDefault is the well-known dev secret. Production deployments
// that forget to set JWT_SECRET would otherwise silently use this value,
// letting any attacker forge tokens. JWTSecret() refuses to return it
// unless AGENTRA_ALLOW_INSECURE_JWT=1 is set (local development only).
const PlaceholderDefault = "agentra-dev-secret-change-in-production"

// minSecretLength is the minimum acceptable JWT secret length in bytes.
// 32 bytes = 256 bits, matching the output of crypto/rand and the
// recommended minimum for HS256.
const minSecretLength = 32

var (
	jwtSecretMu   sync.Mutex
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

// JWTSecret returns the JWT signing secret. The secret is read once from
// the JWT_SECRET env var and cached for the lifetime of the process.
//
// The function panics if the secret is missing, matches the placeholder
// default, or is shorter than minSecretLength bytes. This is intentional:
// continuing to boot with a weak secret is a security incident, not a
// recoverable error. For local development, set AGENTRA_ALLOW_INSECURE_JWT=1
// to opt into the placeholder default.
func JWTSecret() []byte {
	jwtSecretOnce.Do(loadSecret)
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()
	return jwtSecret
}

// ResetSecretForTesting clears the cached secret. It is exported for
// tests that need to re-read JWT_SECRET after changing the env var.
// Must not be called from production code.
func ResetSecretForTesting() {
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()
	jwtSecret = nil
	jwtSecretOnce = sync.Once{}
}

func loadSecret() {
	raw := os.Getenv("JWT_SECRET")
	allowInsecure := os.Getenv("AGENTRA_ALLOW_INSECURE_JWT") == "1"

	if raw == "" {
		// No env var set. Allow only if developer opted in via flag.
		if allowInsecure {
			jwtSecret = []byte(PlaceholderDefault)
			return
		}
		panic("auth: JWT_SECRET is not set. Generate one with `openssl rand -hex 32` and set it in the environment. For local dev only, set AGENTRA_ALLOW_INSECURE_JWT=1 to use the placeholder default.")
	}

	if !allowInsecure && raw == PlaceholderDefault {
		panic("auth: JWT_SECRET is set to the placeholder default (\"" + PlaceholderDefault + "\"). This secret is publicly known — generate a new one with `openssl rand -hex 32`.")
	}

	if len(raw) < minSecretLength {
		panic(fmt.Sprintf("auth: JWT_SECRET must be at least %d bytes (got %d). Generate a stronger secret with `openssl rand -hex 32`.", minSecretLength, len(raw)))
	}

	jwtSecret = []byte(raw)
}

// GeneratePATToken creates a new personal access token: "mul_" + 40 random hex chars.
func GeneratePATToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate PAT token: %w", err)
	}
	return "mul_" + hex.EncodeToString(b), nil
}

// GenerateDaemonToken creates a new daemon auth token: "mdt_" + 40 random hex chars.
func GenerateDaemonToken() (string, error) {
	b := make([]byte, 20) // 20 bytes = 40 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate daemon token: %w", err)
	}
	return "mdt_" + hex.EncodeToString(b), nil
}

// HashToken returns the hex-encoded SHA-256 hash of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

