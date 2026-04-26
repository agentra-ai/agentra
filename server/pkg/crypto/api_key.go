package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "errors"
    "io"

    "golang.org/x/crypto/pbkdf2"
)

const (
    pbkdf2Iterations = 100000
    saltSize         = 32
)

// deriveKey creates a 32-byte key from passphrase and salt using PBKDF2
func deriveKey(passphrase string, salt []byte) []byte {
    return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, 32, sha256.New)
}

// EncryptAPIKey encrypts API key using AES-256-GCM with PBKDF2 key derivation
func EncryptAPIKey(key, passphrase string) (string, error) {
    salt := make([]byte, saltSize)
    if _, err := io.ReadFull(rand.Reader, salt); err != nil {
        return "", err
    }

    k := deriveKey(passphrase, salt)
    block, err := aes.NewCipher(k)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    // Format: salt (32 bytes) + nonce + ciphertext
    ciphertext := gcm.Seal(nonce, nonce, []byte(key), nil)
    ciphertext = append(salt, ciphertext...)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKey decrypts API key encrypted with EncryptAPIKey
func DecryptAPIKey(encrypted, passphrase string) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    if len(ciphertext) < saltSize {
        return "", errors.New("ciphertext too short")
    }

    salt, ciphertext := ciphertext[:saltSize], ciphertext[saltSize:]

    k := deriveKey(passphrase, salt)
    block, err := aes.NewCipher(k)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", errors.New("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}

// HashAPIKey creates a SHA-256 hash of the API key for lookup
func HashAPIKey(key string) string {
    h := sha256.New()
    h.Write([]byte(key))
    return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
