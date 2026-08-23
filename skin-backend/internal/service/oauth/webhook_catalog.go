package oauth

import (
	"element-skin/backend/internal/permission"
	corewebhook "element-skin/backend/internal/webhook"
)

func (s Service) WebhookEventCatalog(actor permission.Actor) ([]corewebhook.Definition, error) {
	allowedPermissions := []string{
		"oauth_app.read.owned",
		"oauth_app.create.owned",
		"oauth_app.update.owned",
		"oauth_app.read.any",
		"oauth_app.update.any",
	}
	allowed := false
	for _, code := range allowedPermissions {
		if actor.Has(permission.MustDefinitionByCode(code)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, forbidden()
	}
	out := make([]corewebhook.Definition, len(corewebhook.Definitions))
	copy(out, corewebhook.Definitions)
	for i := range out {
		out[i].RequiredPermissions = append([]string{}, out[i].RequiredPermissions...)
	}
	return out, nil
}
