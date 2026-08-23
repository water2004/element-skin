package profile

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) ClearProfileTexture(ctx context.Context, actor permission.Actor, profileID, textureType string) error {
	if err := requireOwnedOrBoundProfilePermission(actor, profileID, textureClearOwnedPermission, textureClearBoundPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	if p.UserID != actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
	}
	return s.setProfileTexture(ctx, profileID, textureType, nil)
}

func (s Service) SetProfileTexture(ctx context.Context, actor permission.Actor, profileID, textureType string, hash *string) error {
	if err := requireActorPermission(actor, profileUpdateAnyPermission); err != nil {
		return err
	}
	return s.setProfileTexture(ctx, profileID, textureType, hash)
}

func (s Service) setProfileTexture(ctx context.Context, profileID, textureType string, hash *string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	normalizedType := strings.ToLower(textureType)
	if normalizedType != "skin" && normalizedType != "cape" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "texture_type", Operation: "validate", Reason: "invalid"}
	}
	switch normalizedType {
	case "skin":
		if sameHash(p.SkinHash, hash) {
			return nil
		}
		if err := s.requireLibraryTexture(ctx, hash, normalizedType); err != nil {
			return err
		}
		if err := s.DB.Profiles.UpdateSkin(ctx, profileID, hash); err != nil {
			return profileUpdateError(err)
		}
	case "cape":
		if sameHash(p.CapeHash, hash) {
			return nil
		}
		if err := s.requireLibraryTexture(ctx, hash, normalizedType); err != nil {
			return err
		}
		if err := s.DB.Profiles.UpdateCape(ctx, profileID, hash); err != nil {
			return profileUpdateError(err)
		}
	}
	return nil
}

func (s Service) requireLibraryTexture(ctx context.Context, hash *string, textureType string) error {
	if hash == nil {
		return nil
	}
	ok, err := s.DB.Textures.Exists(ctx, *hash, textureType)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}

func sameHash(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func profileUpdateError(err error) error {
	if database.IsNoRows(err) {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	return err
}
