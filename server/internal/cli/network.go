package cli

import (
	"fmt"
	"os"
	"strings"
)

func FirstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func TrimURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func ResolveSiteURLFromEnv() string {
	return TrimURL(FirstEnv("NEXT_PUBLIC_SITE_URL", "AGENTRA_APP_URL", "FRONTEND_ORIGIN"))
}

func ResolveLocalBindHost() string {
	if host := FirstEnv("AGENTRA_LOCAL_BIND_HOST", "AGENTRA_DAEMON_HOST"); host != "" {
		return host
	}
	return "127.0.0.1"
}

func ResolveLocalCallbackHost() string {
	if host := FirstEnv("AGENTRA_CALLBACK_HOST", "AGENTRA_LOCAL_CALLBACK_HOST"); host != "" {
		return host
	}
	if host := FirstEnv("AGENTRA_LOCAL_BIND_HOST", "AGENTRA_DAEMON_HOST"); host != "" {
		return host
	}
	return "localhost"
}

func ResolveLocalDaemonBaseURL(port string) string {
	return fmt.Sprintf("http://%s:%s", ResolveLocalBindHost(), strings.TrimSpace(port))
}
