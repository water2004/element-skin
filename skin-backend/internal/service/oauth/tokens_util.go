package oauth

import (
	"strings"
	"time"

	"element-skin/backend/internal/util"
)

func generateSecret() (string, string, error) {
	return generateToken()
}

func generateToken() (string, string, error) {
	raw, hash, err := util.GenerateRefreshToken()
	return raw, hash, err
}

func generateUserCode() (string, string, error) {
	raw, _, err := util.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	var compact strings.Builder
	for _, r := range strings.ToUpper(raw) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			compact.WriteRune(r)
		}
		if compact.Len() >= 8 {
			break
		}
	}
	code := compact.String()
	if len(code) < 8 {
		return "", "", badRequest("device_code", "generate", "failed")
	}
	formatted := code[:4] + "-" + code[4:]
	return formatted, util.HashRefreshToken(formatted), nil
}

func tokenPair() (string, string, string, string, error) {
	accessRaw, accessHash, err := generateToken()
	if err != nil {
		return "", "", "", "", err
	}
	refreshRaw, refreshHash, err := generateToken()
	if err != nil {
		return "", "", "", "", err
	}
	return accessRaw, accessHash, refreshRaw, refreshHash, nil
}

func tokenResponse(access, refresh string, codes, oidcScopes []string, idToken string) TokenResponse {
	return TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL / time.Second),
		RefreshToken: refresh,
		Scope:        strings.Join(combinedScopes(codes, oidcScopes), " "),
		Permissions:  codes,
		IDToken:      idToken,
	}
}
