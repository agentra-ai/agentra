package util

import (
	"testing"
)

func TestParseUUID_ValidString(t *testing.T) {
	got := ParseUUID("11111111-2222-3333-4444-555555555555")
	if !got.Valid {
		t.Fatal("expected Valid=true for a well-formed UUID")
	}
	want := [16]byte{
		0x11, 0x11, 0x11, 0x11,
		0x22, 0x22,
		0x33, 0x33,
		0x44, 0x44,
		0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
	}
	if got.Bytes != want {
		t.Fatalf("bytes mismatch: got %x want %x", got.Bytes, want)
	}
}

func TestParseUUID_EmptyString(t *testing.T) {
	got := ParseUUID("")
	if got.Valid {
		t.Fatal("expected Valid=false for empty string")
	}
}

func TestParseUUID_GarbageString(t *testing.T) {
	// Crucially, this must NOT panic. The previous code path used
	// uuid.MustParse which would panic on malformed input — turning
	// a 400 (bad request) into a 500 (server crash) for any client
	// that sent a non-UUID path parameter.
	got := ParseUUID("not-a-uuid")
	if got.Valid {
		t.Fatal("expected Valid=false for garbage input")
	}
}

func TestParseUUID_TooShort(t *testing.T) {
	got := ParseUUID("11111111-2222-3333-4444")
	if got.Valid {
		t.Fatal("expected Valid=false for truncated UUID")
	}
}

func TestParseUUID_Roundtrip(t *testing.T) {
	const s = "abcdef01-2345-6789-abcd-ef0123456789"
	u := ParseUUID(s)
	if !u.Valid {
		t.Fatalf("parse failed for %q", s)
	}
	if got := UUIDToString(u); got != s {
		t.Fatalf("roundtrip mismatch: got %q want %q", got, s)
	}
}
