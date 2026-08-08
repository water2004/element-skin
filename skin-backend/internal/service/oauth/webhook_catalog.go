package oauth

import (
	"element-skin/backend/internal/permission"
	corewebhook "element-skin/backend/internal/webhook"
)

func (s Service) WebhookEventCatalog(actor permission.Actor) ([]corewebhook.Definition, error) {
	owned := permission.MustDefinitionByCode("oauth_app.read.owned")
	any := permission.MustDefinitionByCode("oauth_app.read.any")
	if !actor.Has(owned) && !actor.Has(any) {
		return nil, forbidden()
	}
	out := make([]corewebhook.Definition, len(corewebhook.Definitions))
	copy(out, corewebhook.Definitions)
	for i := range out {
		out[i].RequiredPermissions = append([]string{}, out[i].RequiredPermissions...)
	}
	return out, nil
}
