package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyReleaseChecksum(t *testing.T) {
	asset := "agentra_linux_amd64.tar.gz"
	archive := []byte("release archive")
	digest := sha256.Sum256(archive)
	checksums := []byte(fmt.Sprintf("%x  %s\n", digest, asset))

	if err := verifyReleaseChecksum(asset, archive, checksums); err != nil {
		t.Fatalf("verifyReleaseChecksum: %v", err)
	}
	if err := verifyReleaseChecksum(asset, []byte("tampered"), checksums); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered error = %v", err)
	}
}

func TestChecksumForReleaseAssetRequiresOneValidEntry(t *testing.T) {
	asset := "agentra_darwin_arm64.tar.gz"
	valid := strings.Repeat("a", 64)

	tests := []struct {
		name      string
		contents  string
		wantError string
	}{
		{name: "missing", contents: valid + "  other.tar.gz\n", wantError: "exactly one"},
		{name: "duplicate", contents: valid + "  " + asset + "\n" + valid + "  " + asset + "\n", wantError: "exactly one"},
		{name: "malformed", contents: "not-a-hash  " + asset + "\n", wantError: "invalid SHA-256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := checksumForReleaseAsset(asset, []byte(tt.contents))
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestDownloadReleaseFileEnforcesLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("12345"))
	}))
	defer server.Close()

	_, err := downloadReleaseFile(server.Client(), server.URL, 4)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want size limit", err)
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	contents := []byte("binary contents")
	if err := tw.WriteHeader(&tar.Header{Name: "agentra", Mode: 0o755, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extractBinaryFromTarGz(bytes.NewReader(archive.Bytes()), "agentra")
	if err != nil {
		t.Fatalf("extractBinaryFromTarGz: %v", err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("contents = %q, want %q", got, contents)
	}
}
