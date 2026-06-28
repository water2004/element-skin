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
	serviceTextureReadOwnedPermission   = permission.MustDefinitionByCode("texture.read.owned")
	serviceTextureUpdateMetadataOwned   = permission.MustDefinitionByCode("texture.update_metadata.owned")
	serviceTextureUpdateVisibilityOwned = permission.MustDefinitionByCode("texture.update_visibility.owned")
	serviceTextureDeleteOwnedPermission = permission.MustDefinitionByCode("texture.delete.owned")
	serviceTextureApplyOwnedPermission  = permission.MustDefinitionByCode("texture.apply.owned")
	serviceTextureClearOwnedPermission  = permission.MustDefinitionByCode("texture.clear.owned")
	serviceTextureApplyBoundPermission  = permission.MustDefinitionByCode("texture.apply.bound_profile")
	serviceTextureClearBoundPermission  = permission.MustDefinitionByCode("texture.clear.bound_profile")
	serviceWardrobeEntryAddPermission   = permission.MustDefinitionByCode("wardrobe_entry.add.owned")
)

func requireActorPermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func requireOwnedOrBoundProfilePermission(actor permission.Actor, profileID string, owned, bound permission.Definition) error {
	if actor.Has(owned) {
		return nil
	}
	if actor.BoundProfileID == profileID && actor.Has(bound) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}
