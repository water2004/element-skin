package yggdrasil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"element-skin/backend/internal/config"
)

type Signer struct {
	privateKey *rsa.PrivateKey
	publicPEM  string
}

func NewSigner(cfg config.Config) (*Signer, error) {
	privateKey, err := readPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	publicPEM, publicKey, err := readPublicKey(cfg.PublicKeyPath)
	if err != nil {
		return nil, err
	}
	if err := verifyKeyPair(privateKey, publicKey); err != nil {
		return nil, err
	}
	return &Signer{privateKey: privateKey, publicPEM: publicPEM}, nil
}

func (s *Signer) PublicKeyPEM() string {
	if s == nil {
		return ""
	}
	return s.publicPEM
}

func (s *Signer) SignPropertyValue(value string) (string, error) {
	if s == nil || s.privateKey == nil {
		return "", errors.New("yggdrasil signing key is not loaded")
	}
	digest := sha1.Sum([]byte(value))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA1, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("keys.private_key 未配置")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Yggdrasil 私钥失败 %q: %w", path, err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("Yggdrasil 私钥不是 PEM 格式: %s", path)
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("Yggdrasil 私钥不是 RSA 密钥: %s", path)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("无法解析 Yggdrasil RSA 私钥: %s", path)
}

func readPublicKey(path string) (string, *rsa.PublicKey, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil, errors.New("keys.public_key 未配置")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("读取 Yggdrasil 公钥失败 %q: %w", path, err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "", nil, fmt.Errorf("Yggdrasil 公钥不是 PEM 格式: %s", path)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", nil, fmt.Errorf("无法解析 Yggdrasil 公钥: %s", path)
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return "", nil, fmt.Errorf("Yggdrasil 公钥不是 RSA 密钥: %s", path)
	}
	return string(b), publicKey, nil
}

func verifyKeyPair(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) error {
	const probe = "element-skin-yggdrasil-key-pair-check"
	digest := sha1.Sum([]byte(probe))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA1, digest[:])
	if err != nil {
		return err
	}
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, digest[:], signature); err != nil {
		return errors.New("Yggdrasil 公钥与私钥不匹配")
	}
	return nil
}
