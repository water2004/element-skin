package texture

import (
	"context"
	"errors"
	"net/http"
	"strings"

	texturedb "element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	textureReadAnyPermission             = permission.MustDefinitionByCode("texture.read.any")
	textureUpdateMetadataAnyPermission   = permission.MustDefinitionByCode("texture.update_metadata.any")
	textureUpdateVisibilityAnyPermission = permission.MustDefinitionByCode("texture.update_visibility.any")
	textureDeleteAnyPermission           = permission.MustDefinitionByCode("texture.delete.any")
)

func (s LibraryService) ListAllTextures(ctx context.Context, actor permission.Actor, cursor string, limit int, query, typeFilter string) (map[string]any, error) {
	if err := requireActorPermission(actor, textureReadAnyPermission); err != nil {
		return nil, err
	}
	lastCreated, lastHash, err := textureCursor(cursor, "last_skin_hash")
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	res, err := s.DB.Textures.ListAll(ctx, limit, lastCreated, lastHash, strings.TrimSpace(query), typeFilter)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s LibraryService) UpdateAnyTexture(ctx context.Context, actor permission.Actor, hash, textureType string, body map[string]any) error {
	var patch texturedb.Patch
	if note, ok := body["note"].(string); ok {
		if err := requireActorPermission(actor, textureUpdateMetadataAnyPermission); err != nil {
			return err
		}
		patch.Note = &note
	}
	if model, ok := body["model"].(string); ok {
		if err := requireActorPermission(actor, textureUpdateMetadataAnyPermission); err != nil {
			return err
		}
		if model != "default" && model != "slim" {
			return util.HTTPError{Status: http.StatusBadRequest, Object: "texture_model", Operation: "validate", Reason: "invalid"}
		}
		patch.Model = &model
	}
	if value, ok := body["is_public"]; ok {
		if err := requireActorPermission(actor, textureUpdateVisibilityAnyPermission); err != nil {
			return err
		}
		parsed, err := publicBool(value)
		if err != nil {
			return err
		}
		patch.IsPublic = &parsed
	}
	if textureType != "skin" && textureType != "cape" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "texture_type", Operation: "validate", Reason: "invalid"}
	}
	if patch.Note == nil && patch.Model == nil && patch.IsPublic == nil {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "texture", Operation: "update", Reason: "required"}
	}
	if err := s.DB.Textures.AdminPatch(ctx, hash, textureType, patch); err != nil {
		if errors.Is(err, texturedb.ErrNotFound) {
			return util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
		}
		return err
	}
	return nil
}

func (s LibraryService) DeleteAnyTexture(ctx context.Context, actor permission.Actor, hash, textureType, userID string, force bool) error {
	if err := requireActorPermission(actor, textureDeleteAnyPermission); err != nil {
		return err
	}
	if textureType == "" {
		textureType = "skin"
	}
	if err := s.DB.Textures.AdminDelete(ctx, hash, textureType, userID, force); err != nil {
		if errors.Is(err, texturedb.ErrNotFound) {
			return util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
		}
		if errors.Is(err, texturedb.ErrUserIDRequired) {
			return util.HTTPError{Status: http.StatusBadRequest, Object: "texture", Operation: "delete", Reason: "required"}
		}
		return err
	}
	return nil
}

func publicBool(value any) (bool, error) {
	switch x := value.(type) {
	case bool:
		return x, nil
	case float64:
		if x == 0 {
			return false, nil
		}
		if x == 1 {
			return true, nil
		}
	case int:
		if x == 0 {
			return false, nil
		}
		if x == 1 {
			return true, nil
		}
	}
	return false, util.HTTPError{Status: http.StatusBadRequest, Object: "texture_visibility", Operation: "validate", Reason: "invalid"}
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
