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
			found, ok := permission.DefinitionByCode(def.Code)
			if !ok {
				t.Fatalf("role %q references unknown permission code %q", role.ID, def.Code)
			}
			if found.ID != def.ID || found.BitIndex != def.BitIndex {
				t.Fatalf("role %q permission %q mismatch: id=%#x/%#x bit=%d/%d",
					role.ID, def.Code, uint64(found.ID), uint64(def.ID), found.BitIndex, def.BitIndex)
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
			found, ok := permission.DefinitionByCode(def.Code)
			if !ok {
				t.Fatalf("session policy %q/%q references unknown permission code %q",
					policy.SessionKind, policy.Entrypoint, def.Code)
			}
			if found.ID != def.ID || found.BitIndex != def.BitIndex {
				t.Fatalf("session policy %q/%q permission %q mismatch: id=%#x/%#x bit=%d/%d",
					policy.SessionKind, policy.Entrypoint, def.Code, uint64(found.ID), uint64(def.ID), found.BitIndex, def.BitIndex)
			}
		}
	}
}

func TestUserRoleDoesNotIncludeAdminScopedPermissions(t *testing.T) {
	userRole := roleByID(permission.RoleUser)
	if userRole == nil {
		t.Fatal("user role not found")
	}
	if len(userRole.Permissions) != 47 {
		t.Fatalf("user role has %d permissions, want 47", len(userRole.Permissions))
	}
	expectedCodes := []string{
		"account.read.self",
		"account.update.self",
		"account_password.update.self",
		"account.delete.self",
		"profile.read.owned",
		"profile.create.owned",
		"profile.update.owned",
		"profile.delete.owned",
		"profile.read.bound_profile",
		"profile.update.bound_profile",
		"profile.read.public",
		"texture.read.owned",
		"texture.read.public",
		"texture.create.owned",
		"texture.update_metadata.owned",
		"texture.update_visibility.owned",
		"texture.delete.owned",
		"texture.apply.owned",
		"texture.clear.owned",
		"texture.apply.bound_profile",
		"texture.clear.bound_profile",
		"wardrobe.read.owned",
		"wardrobe_entry.read.owned",
		"wardrobe_entry.add.owned",
		"wardrobe_entry.update.owned",
		"wardrobe_entry.remove.owned",
		"wardrobe_entry.apply.owned",
		"notice.read.owned",
		"notice.dismiss.owned",
		"site_public.read.public",
		"yggdrasil_session.create.owned",
		"yggdrasil_session.refresh.owned",
		"yggdrasil_session.validate.owned",
		"yggdrasil_session.invalidate.owned",
		"yggdrasil_session.signout.owned",
		"yggdrasil_server.join.bound_profile",
		"yggdrasil_server.hasjoined.bound_profile",
		"microsoft_import.start.owned",
		"microsoft_import.read_profile.owned",
		"microsoft_import.create_profile.owned",
		"oauth_app.read.owned",
		"oauth_app.create.owned",
		"oauth_app.update.owned",
		"oauth_app.delete.owned",
		"oauth_grant.read.owned",
		"oauth_grant.revoke.owned",
		"oauth_token.revoke.owned",
	}
	roleCodes := make(map[string]bool, len(userRole.Permissions))
	for _, def := range userRole.Permissions {
		roleCodes[def.Code] = true
	}
	for _, code := range expectedCodes {
		if !roleCodes[code] {
			t.Fatalf("user role must include %s", code)
		}
	}
	adminCodes := map[string]bool{
		"account.ban.any":               true,
		"account.unban.any":             true,
		"account.read.any":              true,
		"account.update.any":            true,
		"account.delete.any":            true,
		"profile.read.any":              true,
		"profile.update.any":            true,
		"profile.delete.any":            true,
		"texture.read.any":              true,
		"texture.update_metadata.any":   true,
		"texture.update_visibility.any": true,
		"texture.delete.any":            true,
		"notice.create.any":             true,
		"notice.update.any":             true,
		"notice.delete.any":             true,
		"permission.grant.any":          true,
		"permission.revoke.any":         true,
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
	if len(adminRole.Permissions) != 43 {
		t.Fatalf("admin role has %d permissions, want 43", len(adminRole.Permissions))
	}
	expectedCodes := []string{
		"account.ban.any",
		"account.unban.any",
		"account.read.any",
		"account.update.any",
		"account.delete.any",
		"user.read.any",
		"user.update.any",
		"profile.read.any",
		"profile.update.any",
		"profile.delete.any",
		"texture.read.any",
		"texture.update_metadata.any",
		"texture.update_visibility.any",
		"texture.delete.any",
		"wardrobe.read.any",
		"wardrobe_entry.remove.any",
		"notice.read.any",
		"notice.create.any",
		"notice.update.any",
		"notice.delete.any",
		"site_settings.read.any",
		"site_settings.update.any",
		"invite.read.any",
		"invite.create.any",
		"invite.delete.any",
		"homepage_media.read.any",
		"homepage_media.create.any",
		"homepage_media.update.any",
		"homepage_media.delete.any",
		"official_whitelist.read.any",
		"official_whitelist.add.any",
		"official_whitelist.remove.any",
		"permission.read.any",
		"permission.grant.any",
		"permission.revoke.any",
		"permission_audit.read.any",
		"audit.read.any",
		"cache.invalidate.any",
		"oauth_app.read.any",
		"oauth_app.update.any",
		"oauth_grant.read.any",
		"oauth_grant.revoke.any",
		"oauth_token.introspect.any",
	}
	roleCodes := make(map[string]bool, len(adminRole.Permissions))
	for _, def := range adminRole.Permissions {
		roleCodes[def.Code] = true
	}
	for _, code := range expectedCodes {
		if !roleCodes[code] {
			t.Fatalf("admin role must include %s", code)
		}
	}
	superAdminCodes := map[string]bool{
		"permission_protected.manage.any": true,
		"permission_role.create.any":      true,
		"permission_role.update.any":      true,
		"permission_role.delete.any":      true,
	}
	systemCodes := map[string]bool{
		"notice.delete.system":            true,
		"yggdrasil_session.delete.system": true,
		"audit.archive.system":            true,
		"cache.invalidate.system":         true,
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

func TestDelegatedDashboardSessionPolicyIncludesAllNonSystemDefinitions(t *testing.T) {
	for _, policy := range permission.SessionPolicies {
		if policy.SessionKind != permission.SessionKindDelegated || policy.Entrypoint != permission.EntrypointDashboard {
			continue
		}
		policyCodes := make(map[string]bool, len(policy.Permissions))
		for _, def := range policy.Permissions {
			policyCodes[def.Code] = true
		}
		for _, def := range permission.Definitions {
			if def.Scope.ID == permission.ScopeSystem {
				if policyCodes[def.Code] {
					t.Fatalf("delegated dashboard session policy should not include system permission %q", def.Code)
				}
				continue
			}
			if !policyCodes[def.Code] {
				t.Fatalf("delegated dashboard session policy missing non-system permission %q", def.Code)
			}
		}
		return
	}
	t.Fatal("delegated dashboard session policy not found")
}

func TestYggdrasilSessionPolicyOnlyIncludesYggdrasilOperations(t *testing.T) {
	for _, policy := range permission.SessionPolicies {
		if policy.SessionKind != permission.SessionKindYggdrasil {
			continue
		}
		if len(policy.Permissions) != 11 {
			t.Fatalf("yggdrasil session policy has %d permissions, want 11", len(policy.Permissions))
		}
		expectedCodes := []string{
			"profile.read.bound_profile",
			"profile.update.bound_profile",
			"texture.apply.bound_profile",
			"texture.clear.bound_profile",
			"yggdrasil_session.create.owned",
			"yggdrasil_session.refresh.owned",
			"yggdrasil_session.validate.owned",
			"yggdrasil_session.invalidate.owned",
			"yggdrasil_session.signout.owned",
			"yggdrasil_server.join.bound_profile",
			"yggdrasil_server.hasjoined.bound_profile",
		}
		policyCodes := make(map[string]bool, len(policy.Permissions))
		for _, def := range policy.Permissions {
			policyCodes[def.Code] = true
		}
		for _, code := range expectedCodes {
			if !policyCodes[code] {
				t.Fatalf("yggdrasil session policy must include %s", code)
			}
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

func TestDefinitionByCodeMiss(t *testing.T) {
	def, ok := permission.DefinitionByCode("nonexistent.code.here")
	if ok || def.ID != 0 {
		t.Fatalf("DefinitionByCode for unknown code should return zero: def=%#v ok=%v", def, ok)
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
