package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	lookPathVersionBinary = exec.LookPath
	resolveExecutablePath = os.Executable
	evalSymlinkPath       = filepath.EvalSymlinks
	runVersionBinary      = func(path string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		return exec.CommandContext(ctx, path, "version").Output()
	}
)

// ResolveReportedCLIVersion returns the version that should be reported to the
// server for the local Agentra daemon.
//
// Release binaries keep reporting their embedded version. Source-built binaries
// often carry "dev"; in that case we fall back to the installed "agentra"
// binary on PATH so the UI reflects the user's actual installed CLI version.
func ResolveReportedCLIVersion(currentVersion string) string {
	currentVersion = normalizeReportedVersion(currentVersion)
	if currentVersion != "" && currentVersion != "dev" {
		return currentVersion
	}

	installedVersion, err := detectInstalledCLIVersion()
	if err == nil {
		installedVersion = normalizeReportedVersion(installedVersion)
		if installedVersion != "" && installedVersion != "dev" {
			return installedVersion
		}
	}

	if currentVersion == "" {
		return "dev"
	}
	return currentVersion
}

func detectInstalledCLIVersion() (string, error) {
	executablePath, err := lookPathVersionBinary("agentra")
	if err != nil {
		return "", err
	}

	if resolvedPath, err := evalSymlinkPath(executablePath); err == nil {
		executablePath = resolvedPath
	}

	if currentPath, err := resolveExecutablePath(); err == nil {
		if resolvedCurrentPath, err := evalSymlinkPath(currentPath); err == nil {
			currentPath = resolvedCurrentPath
		}
		if currentPath == executablePath {
			// Still allow querying the binary if it's the same path. This keeps
			// release builds cheap while letting tests stub the command output.
		}
	}

	output, err := runVersionBinary(executablePath)
	if err != nil {
		return "", err
	}

	fields := strings.Fields(string(bytes.TrimSpace(output)))
	if len(fields) < 2 {
		return "", nil
	}
	return fields[1], nil
}

func normalizeReportedVersion(raw string) string {
	return strings.TrimSpace(raw)
}
