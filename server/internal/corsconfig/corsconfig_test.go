package corsconfig

import (
	"os"
	"reflect"
	"testing"
)

// setEnv sets env vars for the test and restores them on cleanup.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	old := map[string]string{}
	for k := range kv {
		old[k] = os.Getenv(k)
	}
	for k, v := range kv {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
	t.Cleanup(func() {
		for k, v := range old {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
}

func clearAll(t *testing.T) {
	setEnv(t, map[string]string{
		"CORS_ALLOWED_ORIGINS": "",
		"FRONTEND_ORIGIN":      "",
		"NEXT_PUBLIC_SITE_URL": "",
		"AGENTRA_APP_URL":      "",
	})
}

func TestAllowedOrigins_AllUnset_ReturnsNil(t *testing.T) {
	clearAll(t)
	got := AllowedOrigins()
	if got != nil {
		t.Fatalf("expected nil (refuse unsafe default), got %v", got)
	}
}

func TestAllowedOrigins_SingleOrigin(t *testing.T) {
	clearAll(t)
	setEnv(t, map[string]string{
		"CORS_ALLOWED_ORIGINS": "https://app.example.com",
	})
	got := AllowedOrigins()
	want := []string{"https://app.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAllowedOrigins_CommaSeparated(t *testing.T) {
	clearAll(t)
	setEnv(t, map[string]string{
		"CORS_ALLOWED_ORIGINS": "https://a.example.com, https://b.example.com ,https://c.example.com",
	})
	got := AllowedOrigins()
	want := []string{
		"https://a.example.com",
		"https://b.example.com",
		"https://c.example.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAllowedOrigins_FallsBackToFrontendOrigin(t *testing.T) {
	clearAll(t)
	setEnv(t, map[string]string{
		"FRONTEND_ORIGIN": "https://frontend.example.com",
	})
	got := AllowedOrigins()
	want := []string{"https://frontend.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAllowedOrigins_FallsBackToNextPublicSiteURL(t *testing.T) {
	clearAll(t)
	setEnv(t, map[string]string{
		"NEXT_PUBLIC_SITE_URL": "https://site.example.com",
	})
	got := AllowedOrigins()
	want := []string{"https://site.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAllowedOrigins_FallsBackToAgentraAppURL(t *testing.T) {
	clearAll(t)
	setEnv(t, map[string]string{
		"AGENTRA_APP_URL": "https://app.example.com",
	})
	got := AllowedOrigins()
	want := []string{"https://app.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAllowedOrigins_WhitespaceOnly_ReturnsNil(t *testing.T) {
	clearAll(t)
	setEnv(t, map[string]string{
		"CORS_ALLOWED_ORIGINS": "   ,   ",
	})
	got := AllowedOrigins()
	if got != nil {
		t.Fatalf("expected nil (no usable origin), got %v", got)
	}
}

func TestAllowedOrigins_ResolveOrder(t *testing.T) {
	// CORS_ALLOWED_ORIGINS wins over all fallbacks.
	clearAll(t)
	setEnv(t, map[string]string{
		"CORS_ALLOWED_ORIGINS": "https://a.example.com",
		"FRONTEND_ORIGIN":      "https://b.example.com",
		"NEXT_PUBLIC_SITE_URL": "https://c.example.com",
		"AGENTRA_APP_URL":      "https://d.example.com",
	})
	got := AllowedOrigins()
	want := []string{"https://a.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
