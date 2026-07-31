package cli

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxReleaseArchiveBytes  = 256 << 20
	maxReleaseChecksumBytes = 1 << 20
)

// GitHubRelease is the subset of the GitHub releases API response we need.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// FetchLatestRelease fetches the latest release tag from the agentra GitHub repo.
func FetchLatestRelease() (*GitHubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/agentra-ai/agentra/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// IsBrewInstall checks whether the running agentra binary was installed via Homebrew.
func IsBrewInstall() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}

	for _, prefix := range []string{"/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"} {
		if strings.HasPrefix(resolved, prefix+"/Cellar/") {
			return true
		}
	}
	return false
}

// UpdateViaDownload downloads the latest release binary from GitHub and replaces
// the current executable in-place. Returns the combined output message and any error.
func UpdateViaDownload(targetVersion string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("in-place self-update is not supported on Windows while agentra.exe is running; download install.ps1 from the release and run it after stopping the daemon")
	}

	// Determine current binary path.
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink: %w", err)
	}

	// Build download URL: agentra_{os}_{arch}.tar.gz
	tag := targetVersion
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	assetName := fmt.Sprintf("agentra_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	releaseURL := fmt.Sprintf("https://github.com/agentra-ai/agentra/releases/download/%s", tag)

	client := &http.Client{Timeout: 120 * time.Second}
	archiveData, err := downloadReleaseFile(client, releaseURL+"/"+assetName, maxReleaseArchiveBytes)
	if err != nil {
		return "", fmt.Errorf("download release archive: %w", err)
	}
	checksums, err := downloadReleaseFile(client, releaseURL+"/checksums.txt", maxReleaseChecksumBytes)
	if err != nil {
		return "", fmt.Errorf("download release checksums: %w", err)
	}
	if err := verifyReleaseChecksum(assetName, archiveData, checksums); err != nil {
		return "", err
	}

	// Extract the "agentra" binary from the tarball.
	binaryData, err := extractBinaryFromTarGz(bytes.NewReader(archiveData), "agentra")
	if err != nil {
		return "", fmt.Errorf("extract binary: %w", err)
	}

	// Atomic replace: write to temp file, then rename over the original.
	dir := filepath.Dir(exePath)
	tmpFile, err := os.CreateTemp(dir, "agentra-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(binaryData); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Preserve original file permissions.
	info, err := os.Stat(exePath)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("stat original binary: %w", err)
	}
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("chmod temp file: %w", err)
	}

	// Replace the original binary.
	if err := os.Rename(tmpPath, exePath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("replace binary: %w", err)
	}

	return fmt.Sprintf("Downloaded %s and replaced %s", assetName, exePath), nil
}

func downloadReleaseFile(client *http.Client, downloadURL string, maxBytes int64) ([]byte, error) {
	resp, err := client.Get(downloadURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, downloadURL)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("release file exceeds %d bytes", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("release file exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func verifyReleaseChecksum(assetName string, archiveData, checksums []byte) error {
	expected, err := checksumForReleaseAsset(assetName, checksums)
	if err != nil {
		return err
	}
	actual := sha256.Sum256(archiveData)
	if !bytes.Equal(actual[:], expected) {
		return fmt.Errorf("SHA-256 checksum mismatch for %s", assetName)
	}
	return nil
}

func checksumForReleaseAsset(assetName string, checksums []byte) ([]byte, error) {
	var matched string
	matches := 0
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == assetName {
			matched = fields[0]
			matches++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	if matches != 1 {
		return nil, fmt.Errorf("checksums.txt must contain exactly one entry for %s", assetName)
	}
	decoded, err := hex.DecodeString(matched)
	if err != nil || len(decoded) != sha256.Size {
		return nil, fmt.Errorf("invalid SHA-256 checksum for %s", assetName)
	}
	return decoded, nil
}

// extractBinaryFromTarGz reads a .tar.gz stream and returns the contents of the
// named file entry.
func extractBinaryFromTarGz(r io.Reader, name string) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("binary %q not found in archive", name)
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		// Match the binary name (may be prefixed with a directory).
		if filepath.Base(hdr.Name) == name && hdr.Typeflag == tar.TypeReg {
			if hdr.Size < 0 || hdr.Size > maxReleaseArchiveBytes {
				return nil, fmt.Errorf("binary %q exceeds size limit", name)
			}
			data, err := io.ReadAll(io.LimitReader(tr, hdr.Size+1))
			if err != nil {
				return nil, fmt.Errorf("read binary: %w", err)
			}
			if int64(len(data)) != hdr.Size {
				return nil, fmt.Errorf("binary %q has an invalid size", name)
			}
			return data, nil
		}
	}
}
