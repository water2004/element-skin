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
	"path/filepath"
	"strings"

	"element-skin/backend/internal/config"
)

type Signer struct {
	privateKey *rsa.PrivateKey
	publicPEM  string
}

func NewSigner(cfg config.Config) (*Signer, error) {
	privatePath := strings.TrimSpace(cfg.PrivateKeyPath)
	if privatePath == "" {
		return nil, errors.New("keys.private_key 未配置")
	}
	publicPath := strings.TrimSpace(cfg.PublicKeyPath)
	if publicPath == "" {
		return nil, errors.New("keys.public_key 未配置")
	}

	if err := ensureSigningKeyFiles(privatePath, publicPath); err != nil {
		return nil, err
	}
	privateKey, err := readPrivateKey(privatePath)
	if err != nil {
		return nil, err
	}
	publicPEM, publicKey, err := readPublicKey(publicPath)
	if err != nil {
		return nil, err
	}
	if err := verifyKeyPair(privateKey, publicKey); err != nil {
		return nil, err
	}
	return &Signer{privateKey: privateKey, publicPEM: publicPEM}, nil
}

func ensureSigningKeyFiles(privatePath, publicPath string) error {
	privateExists, err := fileExists(privatePath)
	if err != nil {
		return err
	}
	publicExists, err := fileExists(publicPath)
	if err != nil {
		return err
	}
	if privateExists && publicExists {
		return nil
	}
	if !privateExists && publicExists {
		return nil
	}

	var privateKey *rsa.PrivateKey
	if privateExists {
		privateKey, err = readPrivateKey(privatePath)
		if err != nil {
			return err
		}
	} else {
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("生成 Yggdrasil RSA 密钥失败: %w", err)
		}
		privatePEM, err := marshalPrivateKeyPEM(privateKey)
		if err != nil {
			return err
		}
		if err := writeKeyFile(privatePath, privatePEM, 0o600); err != nil {
			return err
		}
	}

	publicPEM, err := marshalPublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	if err := writeKeyFile(publicPath, publicPEM, 0o644); err != nil {
		return err
	}
	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func writeKeyFile(path string, data []byte, mode os.FileMode) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("创建 Yggdrasil 密钥目录失败 %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("写入 Yggdrasil 密钥失败 %q: %w", path, err)
	}
	return nil
}

func marshalPrivateKeyPEM(privateKey *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("编码 Yggdrasil 私钥失败: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func marshalPublicKeyPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("编码 Yggdrasil 公钥失败: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
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
