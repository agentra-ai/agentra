package crypto

import (
    "encoding/base64"
    "testing"
)

func TestEncryptDecryptAPIKey(t *testing.T) {
    key := "sk-ant-api03-xxxxx"
    passphrase := "workspace-secret-123"

    encrypted, err := EncryptAPIKey(key, passphrase)
    if err != nil {
        t.Fatalf("EncryptAPIKey failed: %v", err)
    }

    if encrypted == key {
        t.Fatal("Encrypted key should differ from original")
    }

    decrypted, err := DecryptAPIKey(encrypted, passphrase)
    if err != nil {
        t.Fatalf("DecryptAPIKey failed: %v", err)
    }

    if decrypted != key {
        t.Errorf("Decrypted key mismatch: got %q, want %q", decrypted, key)
    }
}

func TestHashAPIKey(t *testing.T) {
    key := "sk-ant-api03-xxxxx"
    hash := HashAPIKey(key)

    if hash == "" {
        t.Fatal("Hash should not be empty")
    }

    if hash == key {
        t.Fatal("Hash should differ from key")
    }

    // Same key should produce same hash
    hash2 := HashAPIKey(key)
    if hash != hash2 {
        t.Errorf("Same key should produce same hash")
    }
}

func TestEncryptDecryptDifferentPassphrases(t *testing.T) {
    key := "sk-ant-api03-xxxxx"
    passphrase1 := "passphrase1"
    passphrase2 := "passphrase2"

    encrypted1, _ := EncryptAPIKey(key, passphrase1)
    encrypted2, _ := EncryptAPIKey(key, passphrase2)

    // Different passphrases should produce different ciphertexts
    if encrypted1 == encrypted2 {
        t.Error("Different passphrases should produce different ciphertext")
    }

    // Decrypting with wrong passphrase should fail
    _, err := DecryptAPIKey(encrypted1, passphrase2)
    if err == nil {
        t.Error("Decrypting with wrong passphrase should fail")
    }
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
    key := "sk-ant-api03-xxxxx"
    passphrase := "workspace-secret-123"

    encrypted, err := EncryptAPIKey(key, passphrase)
    if err != nil {
        t.Fatalf("EncryptAPIKey failed: %v", err)
    }

    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        t.Fatalf("DecodeString failed: %v", err)
    }

    // Corrupt the ciphertext (flip a byte in the middle)
    corrupted := make([]byte, len(ciphertext))
    copy(corrupted, ciphertext)
    if len(corrupted) > 50 {
        corrupted[50] ^= 0xFF
    }

    corruptedEncoded := base64.StdEncoding.EncodeToString(corrupted)
    _, err = DecryptAPIKey(corruptedEncoded, passphrase)
    if err == nil {
        t.Error("Decrypting corrupted ciphertext should fail")
    }
}

func TestDecryptEmptyCiphertext(t *testing.T) {
    _, err := DecryptAPIKey("", "passphrase")
    if err == nil {
        t.Error("Decrypting empty string should fail")
    }

    // Valid base64 but too short to contain salt + nonce
    _, err = DecryptAPIKey("c2hvcnQ=", "passphrase") // "short" base64 encoded
    if err == nil {
        t.Error("Decrypting too-short ciphertext should fail")
    }
}

func TestEncryptEmptyKey(t *testing.T) {
    _, err := EncryptAPIKey("", "passphrase")
    if err != nil {
        t.Fatalf("EncryptAPIKey with empty key failed unexpectedly: %v", err)
    }

    decrypted, err := DecryptAPIKey("", "passphrase")
    if err == nil && decrypted != "" {
        t.Error("Decrypting empty ciphertext should fail or return empty")
    }
}