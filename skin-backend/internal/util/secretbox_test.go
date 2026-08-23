package util

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestSecretBoxRoundTripAndRandomNonce(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, err := NewSecretBox(key)
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	first, err := box.Encrypt("client-secret")
	if err != nil {
		t.Fatalf("Encrypt first: %v", err)
	}
	second, err := box.Encrypt("client-secret")
	if err != nil {
		t.Fatalf("Encrypt second: %v", err)
	}
	if first == second || !strings.HasPrefix(first, "v1:") {
		t.Fatalf("ciphertexts should be versioned and use distinct nonces: %q %q", first, second)
	}
	plaintext, err := box.Decrypt(first)
	if err != nil || plaintext != "client-secret" {
		t.Fatalf("Decrypt mismatch plaintext=%q err=%v", plaintext, err)
	}
	if empty, err := box.Encrypt(""); err != nil || empty != "" {
		t.Fatalf("empty secret should remain empty, got %q err=%v", empty, err)
	}
}

func TestSecretBoxRejectsInvalidKeysAndCiphertextsExactly(t *testing.T) {
	if _, err := NewSecretBox("not-base64"); err == nil || !strings.Contains(err.Error(), "decode identity encryption key") {
		t.Fatalf("invalid base64 should fail exactly, got %v", err)
	}
	shortKey := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := NewSecretBox(shortKey); err == nil || err.Error() != "identity encryption key must decode to exactly 32 bytes" {
		t.Fatalf("short key mismatch: %v", err)
	}
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, _ := NewSecretBox(key)
	if _, err := box.Decrypt("v0:abc"); err == nil || err.Error() != "unsupported encrypted secret version" {
		t.Fatalf("version error mismatch: %v", err)
	}
	if _, err := box.Decrypt("v1:not-valid-*"); err == nil || err.Error() != "invalid encrypted secret encoding" {
		t.Fatalf("encoding error mismatch: %v", err)
	}
}
