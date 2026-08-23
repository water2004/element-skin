package oauth

import (
	"sort"
	"strings"

	"element-skin/backend/internal/permission"
)

var supportedOIDCScopes = map[string]bool{
	"openid":         true,
	"profile":        true,
	"email":          true,
	"offline_access": true,
}

type authorizationScopes struct {
	Permissions []string
	OIDC        []string
}

func parseScope(raw string) ([]string, error) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil, badRequest("oauth_scope", "validate", "required")
	}
	return validateCodes(parts)
}

func parseAuthorizationScopes(raw string) (authorizationScopes, error) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return authorizationScopes{}, badRequest("oauth_scope", "validate", "required")
	}
	permissionCodes := make([]string, 0, len(parts))
	oidcScopes := make([]string, 0, 4)
	seenOIDC := map[string]bool{}
	for _, scope := range parts {
		if supportedOIDCScopes[scope] {
			if !seenOIDC[scope] {
				seenOIDC[scope] = true
				oidcScopes = append(oidcScopes, scope)
			}
			continue
		}
		permissionCodes = append(permissionCodes, scope)
	}
	if len(oidcScopes) > 0 && !seenOIDC["openid"] {
		return authorizationScopes{}, badRequest("oidc_scope", "validate", "required")
	}
	codes, err := validateCodes(permissionCodes)
	if err != nil {
		return authorizationScopes{}, err
	}
	sort.Strings(oidcScopes)
	return authorizationScopes{Permissions: codes, OIDC: oidcScopes}, nil
}

func combinedScopes(permissionCodes, oidcScopes []string) []string {
	out := append([]string(nil), permissionCodes...)
	out = append(out, oidcScopes...)
	sort.Strings(out)
	return out
}

func hasOIDCScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

func validateCodes(codes []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		def, ok := permission.DefinitionByCode(code)
		if !ok || def.Scope.ID == permission.ScopeSystem {
			return nil, badRequest("oauth_scope", "validate", "invalid")
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

func intersectPermissionCodes(left, right []string) []string {
	allowed := make(map[string]bool, len(right))
	for _, code := range right {
		allowed[code] = true
	}
	out := make([]string, 0, len(left))
	for _, code := range left {
		if allowed[code] {
			out = append(out, code)
		}
	}
	return out
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
