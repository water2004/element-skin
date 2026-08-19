package texture

import (
	"context"
	"net/http"
	"strings"

	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s LibraryService) AddTextureToWardrobe(ctx context.Context, actor permission.Actor, hash, textureType string) error {
	if err := requireActorPermission(actor, wardrobeEntryAddPermission); err != nil {
		return err
	}
	ok, err := s.DB.Textures.AddToWardrobe(ctx, actor.UserID, hash, textureType)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}

func (s LibraryService) ApplyTextureToProfile(ctx context.Context, actor permission.Actor, profileID, hash, textureType string) error {
	if err := requireOwnedOrBoundProfilePermission(actor, profileID, textureApplyOwnedPermission, textureApplyBoundPermission); err != nil {
		return err
	}
	userID := actor.UserID
	owns, err := s.DB.Textures.VerifyOwnership(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !owns {
		return util.HTTPError{Status: http.StatusForbidden, Object: "texture", Operation: "authorize", Reason: "denied"}
	}
	profileOwner, err := s.DB.Profiles.VerifyOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !profileOwner {
		return util.HTTPError{Status: http.StatusForbidden, Object: "profile", Operation: "authorize", Reason: "denied"}
	}
	info, err := s.DB.Textures.GetInfo(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if info == nil {
		return util.HTTPError{Status: http.StatusForbidden, Object: "texture", Operation: "resolve", Reason: "not_found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		modelName, _ := info["model"].(string)
		return profileUpdateError(s.DB.Profiles.UpdateSkinAndModel(ctx, profileID, &hash, profilestore.NormalizeModel(modelName)))
	case "cape":
		return s.setProfileCape(ctx, profileID, &hash)
	default:
		return util.HTTPError{Status: http.StatusBadRequest, Object: "texture_type", Operation: "validate", Reason: "invalid"}
	}
}

func (s LibraryService) setProfileCape(ctx context.Context, profileID string, hash *string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	if sameHash(p.CapeHash, hash) {
		return nil
	}
	return profileUpdateError(s.DB.Profiles.UpdateCape(ctx, profileID, hash))
}
