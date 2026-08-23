package oauth

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"

	"element-skin/backend/internal/config"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
)

type OIDCSigner struct {
	signer *yggsvc.Signer
	kid    string
}

func NewOIDCSigner(cfg config.Config) (*OIDCSigner, error) {
	privatePath := strings.TrimSpace(cfg.OIDCPrivateKeyPath)
	publicPath := strings.TrimSpace(cfg.OIDCPublicKeyPath)
	if privatePath == "" || publicPath == "" {
		return nil, errors.New("OIDC signing key paths are not configured")
	}
	keyConfig := cfg
	keyConfig.PrivateKeyPath = privatePath
	keyConfig.PublicKeyPath = publicPath
	signer, err := yggsvc.NewSigner(keyConfig)
	if err != nil {
		return nil, err
	}
	publicKey := signer.RSAPublicKey()
	if publicKey == nil {
		return nil, errors.New("OIDC RSA signing key is not loaded")
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(der)
	return &OIDCSigner{signer: signer, kid: base64.RawURLEncoding.EncodeToString(sum[:16])}, nil
}

func (s *OIDCSigner) Sign(claims map[string]any) (string, error) {
	if s == nil || s.signer == nil {
		return "", errors.New("OIDC signing key is not loaded")
	}
	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": s.kid, "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := encodedHeader + "." + encodedPayload
	signature, err := s.signer.SignRS256([]byte(signingInput))
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *OIDCSigner) JWKS() map[string]any {
	if s == nil || s.signer == nil || s.signer.RSAPublicKey() == nil {
		return map[string]any{"keys": []any{}}
	}
	publicKey := s.signer.RSAPublicKey()
	exponent := make([]byte, 4)
	binary.BigEndian.PutUint32(exponent, uint32(publicKey.E))
	for len(exponent) > 1 && exponent[0] == 0 {
		exponent = exponent[1:]
	}
	return map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": s.kid,
		"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(exponent),
	}}}
}
