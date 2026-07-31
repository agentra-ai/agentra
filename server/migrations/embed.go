package migrations

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed *.up.sql
var upFiles embed.FS

// LatestVersion returns the version encoded in the lexically latest up migration.
func LatestVersion() (string, error) {
	entries, err := upFiles.ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("read embedded migrations: %w", err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		versions = append(
			versions,
			strings.TrimSuffix(entry.Name(), ".up.sql"),
		)
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no embedded up migrations")
	}

	sort.Strings(versions)
	return versions[len(versions)-1], nil
}
