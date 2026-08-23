package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const secretBoxPrefix = "v1:"

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(encodedKey string) (SecretBox, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil {
		return SecretBox{}, fmt.Errorf("decode identity encryption key: %w", err)
	}
	if len(key) != 32 {
		return SecretBox{}, errors.New("identity encryption key must decode to exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return SecretBox{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return SecretBox{}, err
	}
	return SecretBox{aead: aead}, nil
}

func (b SecretBox) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if b.aead == nil {
		return "", errors.New("secret box is not initialized")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretBoxPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (b SecretBox) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if b.aead == nil {
		return "", errors.New("secret box is not initialized")
	}
	if !strings.HasPrefix(ciphertext, secretBoxPrefix) {
		return "", errors.New("unsupported encrypted secret version")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, secretBoxPrefix))
	if err != nil {
		return "", errors.New("invalid encrypted secret encoding")
	}
	if len(sealed) < b.aead.NonceSize() {
		return "", errors.New("invalid encrypted secret length")
	}
	nonce := sealed[:b.aead.NonceSize()]
	plaintext, err := b.aead.Open(nil, nonce, sealed[b.aead.NonceSize():], nil)
	if err != nil {
		return "", errors.New("decrypt secret")
	}
	return string(plaintext), nil
}
