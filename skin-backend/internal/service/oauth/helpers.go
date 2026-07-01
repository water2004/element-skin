package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
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
		return "", "", badRequest("could not generate user_code")
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

func tokenResponse(access, refresh string, codes []string) TokenResponse {
	return TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL / time.Second),
		RefreshToken: refresh,
		Scope:        strings.Join(codes, " "),
		Permissions:  codes,
	}
}

func parseScope(raw string) ([]string, error) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil, badRequest("scope is required")
	}
	return validateCodes(parts)
}

func validateCodes(codes []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		def, ok := permission.DefinitionByCode(code)
		if !ok || def.Scope.ID == permission.ScopeSystem {
			return nil, badRequest("invalid scope")
		}
		if !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out, nil
}

func permissionIDsFromCodes(codes []string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func permissionCodesFromIDs(ids []int64) []string {
	byID := map[int64]string{}
	for _, def := range permission.Definitions {
		byID[int64(def.ID)] = def.Code
	}
	codes := make([]string, 0, len(ids))
	for _, id := range ids {
		if code := byID[id]; code != "" {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	return codes
}

func permissionCodesFromBitSet(bits permission.BitSet) []string {
	codes := make([]string, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		if bits.Has(def.BitIndex) {
			codes = append(codes, def.Code)
		}
	}
	sort.Strings(codes)
	return codes
}

func clientCredentialsPolicyCodes() []string {
	for _, policy := range permission.SessionPolicies {
		if policy.SessionKind == permission.SessionKindClient && policy.Entrypoint == permission.EntrypointAPI {
			codes := make([]string, 0, len(policy.Permissions))
			for _, def := range policy.Permissions {
				codes = append(codes, def.Code)
			}
			sort.Strings(codes)
			return codes
		}
	}
	return []string{}
}

func bitSetFromPermissionIDs(ids []int64) permission.BitSet {
	byID := map[int64]int{}
	for _, def := range permission.Definitions {
		byID[int64(def.ID)] = def.BitIndex
	}
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, id := range ids {
		if bitIndex, ok := byID[id]; ok {
			bits.Set(bitIndex)
		}
	}
	return bits
}

func idSet(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func validPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func validClientStatus(status string) bool {
	switch status {
	case StatusPending, StatusActive, StatusRejected, StatusDisabled:
		return true
	default:
		return false
	}
}

func oauthError(detail string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Detail: detail}
}

func clientResponse(client model.OAuthClient, permissions []string, secret string) map[string]any {
	out := publicClient(client)
	out["permissions"] = permissions
	if secret != "" {
		out["client_secret"] = secret
	}
	return out
}

func publicClient(client model.OAuthClient) map[string]any {
	return map[string]any{
		"client_id":     client.ID,
		"owner_user_id": client.OwnerUserID,
		"name":          client.Name,
		"description":   client.Description,
		"redirect_uri":  client.RedirectURI,
		"website_url":   client.WebsiteURL,
		"client_type":   client.ClientType,
		"status":        client.Status,
		"created_at":    client.CreatedAt,
		"updated_at":    client.UpdatedAt,
	}
}

func grantResponse(grant model.OAuthGrant, permissions []string) map[string]any {
	return map[string]any{
		"id":          grant.ID,
		"user_id":     grant.UserID,
		"subject_id":  grant.SubjectID,
		"client_id":   grant.ClientID,
		"status":      grant.Status,
		"created_at":  grant.CreatedAt,
		"revoked_at":  grant.RevokedAt,
		"permissions": permissions,
	}
}

func permissionDetails(codes []string) []map[string]any {
	out := make([]map[string]any, 0, len(codes))
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		out = append(out, map[string]any{
			"code":                 def.Code,
			"description":          def.Description,
			"resource":             def.Resource.Code,
			"resource_description": def.Resource.Description,
			"action":               def.Action.Code,
			"action_description":   def.Action.Description,
			"scope":                def.Scope.Code,
			"scope_description":    def.Scope.Description,
		})
	}
	return out
}

func badRequest(detail string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Detail: detail}
}

func forbidden() error {
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func notFound(detail string) error {
	return util.HTTPError{Status: http.StatusNotFound, Detail: detail}
}
