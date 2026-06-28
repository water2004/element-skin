package permission_test

import (
	"strings"
	"testing"

	"element-skin/backend/internal/permission"
)

func TestCatalogCodesAreStrictTriplesAndIDsMatchParts(t *testing.T) {
	seenCodes := map[string]struct{}{}
	seenBits := map[int]struct{}{}
	seenIDs := map[permission.ID]struct{}{}
	for _, def := range permission.Definitions {
		if parts := strings.Split(def.Code, "."); len(parts) != 3 {
			t.Fatalf("permission code %q is not a strict triple", def.Code)
		}
		wantCode := def.Resource.Code + "." + def.Action.Code + "." + def.Scope.Code
		if def.Code != wantCode {
			t.Fatalf("code mismatch: got %q want %q", def.Code, wantCode)
		}
		if !def.ID.Valid() || def.ID.ResourceID() != def.Resource.ID || def.ID.ActionID() != def.Action.ID || def.ID.ScopeID() != def.Scope.ID {
			t.Fatalf("id mismatch for %s: id=%#x resource=%d/%d action=%d/%d scope=%d/%d", def.Code, uint64(def.ID), def.ID.ResourceID(), def.Resource.ID, def.ID.ActionID(), def.Action.ID, def.ID.ScopeID(), def.Scope.ID)
		}
		if _, ok := seenCodes[def.Code]; ok {
			t.Fatalf("duplicate code %q", def.Code)
		}
		seenCodes[def.Code] = struct{}{}
		if _, ok := seenIDs[def.ID]; ok {
			t.Fatalf("duplicate id %#x", uint64(def.ID))
		}
		seenIDs[def.ID] = struct{}{}
		if _, ok := seenBits[def.BitIndex]; ok {
			t.Fatalf("duplicate bit index %d", def.BitIndex)
		}
		seenBits[def.BitIndex] = struct{}{}
	}
	if len(permission.Definitions) != len(seenBits) {
		t.Fatalf("bit indexes should be dense count=%d seen=%d", len(permission.Definitions), len(seenBits))
	}
	for i := range permission.Definitions {
		if _, ok := seenBits[i]; !ok {
			t.Fatalf("missing dense bit index %d", i)
		}
	}
	if _, ok := seenCodes["permission_protected.manage.any"]; !ok {
		t.Fatalf("permission_protected.manage.any is missing")
	}
	if _, ok := seenCodes["texture.update_visibility.owned"]; !ok {
		t.Fatalf("texture.update_visibility.owned is missing")
	}
	if _, ok := seenCodes["yggdrasil_server.join.bound_profile"]; !ok {
		t.Fatalf("yggdrasil_server.join.bound_profile is missing")
	}
}

func TestRolesOnlyReferenceKnownDefinitions(t *testing.T) {
	for _, role := range permission.Roles {
		for _, def := range role.Permissions {
			if !def.ID.Valid() {
				t.Fatalf("role %q references invalid permission id: code=%q id=%#x", role.ID, def.Code, uint64(def.ID))
			}
		}
	}
}

func TestSessionPoliciesOnlyReferenceKnownDefinitions(t *testing.T) {
	for _, policy := range permission.SessionPolicies {
		for _, def := range policy.Permissions {
			if !def.ID.Valid() {
				t.Fatalf("session policy %q/%q references invalid permission id: code=%q id=%#x",
					policy.SessionKind, policy.Entrypoint, def.Code, uint64(def.ID))
			}
		}
	}
}

func TestUserRoleDoesNotIncludeAdminScopedPermissions(t *testing.T) {
	userRole := roleByID(permission.RoleUser)
	if userRole == nil {
		t.Fatal("user role not found")
	}
	adminCodes := map[string]bool{
		"account.ban.any":            true,
		"account.unban.any":          true,
		"account.read.any":           true,
		"account.update.any":         true,
		"account.delete.any":         true,
		"profile.read.any":           true,
		"profile.update.any":         true,
		"profile.delete.any":         true,
		"texture.read.any":           true,
		"texture.update_metadata.any": true,
		"texture.update_visibility.any": true,
		"texture.delete.any":         true,
		"notice.create.any":          true,
		"notice.update.any":          true,
		"notice.delete.any":          true,
		"permission.grant.any":       true,
		"permission.revoke.any":      true,
	}
	for _, def := range userRole.Permissions {
		if adminCodes[def.Code] {
			t.Fatalf("user role should not include admin permission %q", def.Code)
		}
	}
}

func TestAdminRoleDoesNotIncludeSuperAdminOrSystemPermissions(t *testing.T) {
	adminRole := roleByID(permission.RoleAdmin)
	if adminRole == nil {
		t.Fatal("admin role not found")
	}
	superAdminCodes := map[string]bool{
		"permission_protected.manage.any": true,
		"permission_role.create.any":      true,
		"permission_role.update.any":      true,
		"permission_role.delete.any":      true,
	}
	systemCodes := map[string]bool{
		"notice.delete.system":           true,
		"yggdrasil_session.delete.system": true,
		"audit.archive.system":           true,
		"cache.invalidate.system":        true,
	}
	for _, def := range adminRole.Permissions {
		if superAdminCodes[def.Code] {
			t.Fatalf("admin role should not include super-admin permission %q", def.Code)
		}
		if systemCodes[def.Code] {
			t.Fatalf("admin role should not include system-scope permission %q", def.Code)
		}
	}
}

func TestWebSessionPolicyIncludesAllNonSystemDefinitions(t *testing.T) {
	for _, policy := range permission.SessionPolicies {
		if policy.SessionKind != permission.SessionKindWeb {
			continue
		}
		policyCodes := make(map[string]bool, len(policy.Permissions))
		for _, def := range policy.Permissions {
			policyCodes[def.Code] = true
		}
		for _, def := range permission.Definitions {
			if def.Scope.ID == permission.ScopeSystem {
				if policyCodes[def.Code] {
					t.Fatalf("web session policy %q should not include system permission %q",
						policy.Entrypoint, def.Code)
				}
				continue
			}
			if !policyCodes[def.Code] {
				t.Fatalf("web session policy %q missing non-system permission %q",
					policy.Entrypoint, def.Code)
			}
		}
	}
}

func TestYggdrasilSessionPolicyOnlyIncludesYggdrasilOperations(t *testing.T) {
	for _, policy := range permission.SessionPolicies {
		if policy.SessionKind != permission.SessionKindYggdrasil {
			continue
		}
		for _, def := range policy.Permissions {
			if def.Resource.ID != permission.ResourceYggdrasilSession &&
				def.Resource.ID != permission.ResourceYggdrasilServer &&
				def.Resource.ID != permission.ResourceProfile &&
				def.Resource.ID != permission.ResourceTexture {
				t.Fatalf("yggdrasil session policy includes unexpected resource %q: %s",
					def.Resource.Code, def.Code)
			}
		}
	}
}

func TestMustDefinitionByCodePanicsOnUnknownCode(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustDefinitionByCode should panic on unknown code")
		}
	}()
	permission.MustDefinitionByCode("nonexistent.permission.code")
}

func roleByID(id string) *permission.Role {
	for _, role := range permission.Roles {
		if role.ID == id {
			return &role
		}
	}
	return nil
}
