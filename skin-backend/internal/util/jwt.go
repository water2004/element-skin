package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const DefaultJWTSecret = "dev-secret-default-key-at-least-32-chars-long"
const ShippedPlaceholderJWTSecret = "dev-secret-please-change-to-a-very-long-string-in-production"

func ValidateJWTSecret(secret string) error {
	if secret == "" {
		return errors.New("jwt.secret 未配置：请在配置文件中设置高熵密钥后再启动")
	}
	if secret == DefaultJWTSecret || secret == ShippedPlaceholderJWTSecret {
		return errors.New("jwt.secret 仍为默认/占位值：必须改为随机高熵密钥后再启动")
	}
	if len([]byte(secret)) < 32 {
		return errors.New("jwt.secret 过短：至少 32 字节")
	}
	return nil
}

func CreateAccessToken(secret, userID string, isAdmin bool, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":      userID,
		"is_admin": isAdmin,
		"type":     "access",
		"exp":      time.Now().Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func DecodeAccessToken(secret, tokenString string) (map[string]any, bool) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid alg")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "access" {
		return nil, false
	}
	return map[string]any(claims), true
}

func GenerateRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashRefreshToken(raw), nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
