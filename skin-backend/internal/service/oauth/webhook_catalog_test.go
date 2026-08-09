package oauth_test

import (
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
)

func TestWebhookEventCatalogAllowsEveryApplicationManagementConsumerExactly(t *testing.T) {
	allowedCodes := []string{
		"oauth_app.read.owned",
		"oauth_app.create.owned",
		"oauth_app.update.owned",
		"oauth_app.read.any",
		"oauth_app.update.any",
	}
	service := oauth.Service{}
	for _, code := range allowedCodes {
		t.Run(code, func(t *testing.T) {
			items, err := service.WebhookEventCatalog(oauthActorWithPermissions(code))
			if err != nil {
				t.Fatalf("WebhookEventCatalog(%q) error=%v", code, err)
			}
			if len(items) != 15 || items[0].Type != "account.created" || items[14].Type != "texture.deleted" {
				t.Fatalf("WebhookEventCatalog(%q)=%#v", code, items)
			}
		})
	}
}

func TestWebhookEventCatalogRejectsUnrelatedApplicationPermissionsExactly(t *testing.T) {
	service := oauth.Service{}
	for _, codes := range [][]string{
		nil,
		{"account.read.self"},
		{"oauth_app.delete.owned"},
		{"oauth_grant.read.owned"},
	} {
		_, err := service.WebhookEventCatalog(oauthActorWithPermissions(codes...))
		assertHTTPError(t, err, 403, "permission denied")
	}
}

func oauthActorWithPermissions(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{SubjectID: "user:test", UserID: "test", Permissions: bits}
}
