package webhook_test

import (
	"reflect"
	"strings"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/webhook"
)

func TestEventCatalogUsesStableResourceActionNamesAndExistingPermissionsExactly(t *testing.T) {
	wantTypes := []string{
		"account.created",
		"account.deleted",
		"account.updated",
		"oauth_grant.created",
		"oauth_grant.revoked",
		"oauth_grant.updated",
		"official_whitelist.added",
		"official_whitelist.removed",
		"permission.updated",
		"profile.created",
		"profile.deleted",
		"profile.updated",
		"texture.created",
		"texture.deleted",
		"texture.updated",
	}
	if got := webhook.Types(); !reflect.DeepEqual(got, wantTypes) {
		t.Fatalf("webhook event types=%v want=%v", got, wantTypes)
	}
	seen := map[string]bool{}
	for _, definition := range webhook.Definitions {
		parts := strings.Split(definition.Type, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			t.Fatalf("event type %q must use resource.action", definition.Type)
		}
		if seen[definition.Type] {
			t.Fatalf("duplicate event type %q", definition.Type)
		}
		seen[definition.Type] = true
		if definition.Description == "" || len(definition.RequiredPermissions) == 0 {
			t.Fatalf("incomplete event definition: %#v", definition)
		}
		for _, code := range definition.RequiredPermissions {
			if _, ok := permission.DefinitionByCode(code); !ok {
				t.Fatalf("event %q references missing permission %q", definition.Type, code)
			}
		}
		lookedUp, ok := webhook.DefinitionByType(definition.Type)
		if !ok || !reflect.DeepEqual(lookedUp, definition) {
			t.Fatalf("event lookup mismatch for %q: %#v ok=%v", definition.Type, lookedUp, ok)
		}
	}
	wantPermissions := map[string][]string{
		"account.created":            {"account.read.any"},
		"account.updated":            {"account.read.any", "account.read.self"},
		"account.deleted":            {"account.read.any"},
		"official_whitelist.added":   {"official_whitelist.read.any"},
		"official_whitelist.removed": {"official_whitelist.read.any"},
		"permission.updated":         {"permission.read.any"},
	}
	for eventType, permissions := range wantPermissions {
		definition, ok := webhook.DefinitionByType(eventType)
		if !ok || !reflect.DeepEqual(definition.RequiredPermissions, permissions) {
			t.Fatalf("event %q permissions=%v want=%v ok=%v", eventType, definition.RequiredPermissions, permissions, ok)
		}
	}
	accountUpdated, _ := webhook.DefinitionByType("account.updated")
	if accountUpdated.DelegatedPermissionCode != "account.read.self" || accountUpdated.ApplicationPermissionCode != "account.read.any" {
		t.Fatalf("account.updated authorization modes=%#v", accountUpdated)
	}
	permissionUpdated, _ := webhook.DefinitionByType("permission.updated")
	if permissionUpdated.DelegatedPermissionCode != "" || permissionUpdated.ApplicationPermissionCode != "permission.read.any" {
		t.Fatalf("permission.updated authorization modes=%#v", permissionUpdated)
	}
}
