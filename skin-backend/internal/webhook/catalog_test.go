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
		"oauth_grant.created",
		"oauth_grant.revoked",
		"oauth_grant.updated",
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
}
