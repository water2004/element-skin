package site

import (
	"net/http"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	serviceAccountUpdateSelfPermission  = permission.MustDefinitionByCode("account.update.self")
	serviceAccountDeleteSelfPermission  = permission.MustDefinitionByCode("account.delete.self")
	serviceAccountDeleteAnyPermission   = permission.MustDefinitionByCode("account.delete.any")
	servicePasswordUpdateSelfPermission = permission.MustDefinitionByCode("account_password.update.self")
	serviceProfileReadOwnedPermission   = permission.MustDefinitionByCode("profile.read.owned")
	serviceProfileCreateOwnedPermission = permission.MustDefinitionByCode("profile.create.owned")
	serviceProfileUpdateOwnedPermission = permission.MustDefinitionByCode("profile.update.owned")
	serviceProfileDeleteOwnedPermission = permission.MustDefinitionByCode("profile.delete.owned")
	serviceTextureClearOwnedPermission  = permission.MustDefinitionByCode("texture.clear.owned")
	serviceWardrobeEntryAddPermission   = permission.MustDefinitionByCode("wardrobe_entry.add.owned")
)

func requireActorPermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}
