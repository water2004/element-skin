package texture

import (
	"context"
	"net/http"

	texturedb "element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s LibraryService) UpdateTexture(ctx context.Context, actor permission.Actor, hash, textureType string, body map[string]any) (map[string]any, error) {
	var patch texturedb.Patch
	if model, ok := body["model"].(string); ok && model != "default" && model != "slim" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "texture_model", Operation: "validate", Reason: "invalid"}
	} else if ok {
		if err := requireActorPermission(actor, textureUpdateMetadataOwnedPermission); err != nil {
			return nil, err
		}
		patch.Model = &model
	}
	if note, ok := body["note"].(string); ok {
		if err := requireActorPermission(actor, textureUpdateMetadataOwnedPermission); err != nil {
			return nil, err
		}
		patch.Note = &note
	}
	if value, ok := body["is_public"]; ok {
		if err := requireActorPermission(actor, textureUpdateVisibilityOwnedPermission); err != nil {
			return nil, err
		}
		parsed := false
		switch x := value.(type) {
		case bool:
			parsed = x
		case float64:
			parsed = x != 0
		case int:
			parsed = x != 0
		default:
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "texture_visibility", Operation: "validate", Reason: "invalid"}
		}
		patch.IsPublic = &parsed
	}
	if patch.Note != nil || patch.Model != nil || patch.IsPublic != nil {
		if err := s.DB.Textures.UpdateForUser(ctx, actor.UserID, hash, textureType, patch); err != nil {
			return nil, textureNotFoundError(err)
		}
	}
	info, err := s.TextureDetail(ctx, actor, hash, textureType)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (s LibraryService) DeleteTexture(ctx context.Context, actor permission.Actor, hash, textureType string) error {
	if err := requireActorPermission(actor, textureDeleteOwnedPermission); err != nil {
		return err
	}
	uploader, exists, err := s.DB.Textures.LibraryUploader(ctx, hash, textureType)
	if err != nil {
		return err
	}
	if exists && uploader == actor.UserID {
		return textureNotFoundError(s.DB.Textures.DeleteLibraryTexture(ctx, hash, textureType))
	}
	deleted, err := s.DB.Textures.DeleteFromLibrary(ctx, actor.UserID, hash, textureType)
	if err != nil {
		return err
	}
	if !deleted {
		return util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}
