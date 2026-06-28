package site

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Site) ApplyTextureToProfile(ctx context.Context, actor permission.Actor, profileID, hash, textureType string) error {
	return s.applyTextureToProfile(ctx, actor, profileID, hash, textureType, nil)
}

func (s Site) ApplyTextureToProfileWithModel(ctx context.Context, actor permission.Actor, profileID, hash, textureType, skinModel string) error {
	return s.applyTextureToProfile(ctx, actor, profileID, hash, textureType, &skinModel)
}

func (s Site) applyTextureToProfile(ctx context.Context, actor permission.Actor, profileID, hash, textureType string, skinModel *string) error {
	if err := requireOwnedOrBoundProfilePermission(actor, profileID, serviceTextureApplyOwnedPermission, serviceTextureApplyBoundPermission); err != nil {
		return err
	}
	userID := actor.UserID
	owns, err := s.DB.Textures.VerifyOwnership(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !owns {
		return util.HTTPError{Status: 403, Detail: "Texture not found in your library"}
	}
	profileOwner, err := s.DB.Profiles.VerifyOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !profileOwner {
		return util.HTTPError{Status: 403, Detail: "Profile not yours"}
	}
	info, err := s.DB.Textures.GetInfo(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if info == nil {
		return util.HTTPError{Status: 403, Detail: "Texture info not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		modelName, _ := info["model"].(string)
		if skinModel != nil {
			modelName = *skinModel
		}
		return profileUpdateError(s.DB.Profiles.UpdateSkinAndModel(ctx, profileID, &hash, profile.NormalizeModel(modelName)))
	case "cape":
		return s.SetProfileTexture(ctx, profileID, "cape", &hash)
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
}

func (s Site) TextureDetail(ctx context.Context, actor permission.Actor, hash, textureType string) (map[string]any, error) {
	if err := requireActorPermission(actor, serviceTextureReadOwnedPermission); err != nil {
		return nil, err
	}
	userID := actor.UserID
	info, err := s.DB.Textures.GetInfo(ctx, userID, hash, textureType)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return info, nil
}

func (s Site) UpdateTexture(ctx context.Context, actor permission.Actor, hash, textureType string, body map[string]any) (map[string]any, error) {
	var patch texture.Patch
	if model, ok := body["model"].(string); ok && model != "default" && model != "slim" {
		return nil, util.HTTPError{Status: 400, Detail: "invalid model"}
	} else if ok {
		if err := requireActorPermission(actor, serviceTextureUpdateMetadataOwned); err != nil {
			return nil, err
		}
		patch.Model = &model
	}
	if note, ok := body["note"].(string); ok {
		if err := requireActorPermission(actor, serviceTextureUpdateMetadataOwned); err != nil {
			return nil, err
		}
		patch.Note = &note
	}
	if value, ok := body["is_public"]; ok {
		if err := requireActorPermission(actor, serviceTextureUpdateVisibilityOwned); err != nil {
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
			return nil, util.HTTPError{Status: 400, Detail: "invalid is_public"}
		}
		patch.IsPublic = &parsed
	}
	userID := actor.UserID
	if patch.Note != nil || patch.Model != nil || patch.IsPublic != nil {
		if err := s.DB.Textures.UpdateForUser(ctx, userID, hash, textureType, patch); err != nil {
			return nil, textureNotFoundError(err)
		}
	}
	info, err := s.TextureDetail(ctx, actor, hash, textureType)
	if err != nil {
		return nil, err
	}
	info["ok"] = true
	return info, nil
}

func (s Site) DeleteTexture(ctx context.Context, actor permission.Actor, hash, textureType string) error {
	if err := requireActorPermission(actor, serviceTextureDeleteOwnedPermission); err != nil {
		return err
	}
	userID := actor.UserID
	uploader, exists, err := s.DB.Textures.LibraryUploader(ctx, hash, textureType)
	if err != nil {
		return err
	}
	if exists && uploader == userID {
		return textureNotFoundError(s.DB.Textures.DeleteLibraryTexture(ctx, hash, textureType))
	}
	deleted, err := s.DB.Textures.DeleteFromLibrary(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !deleted {
		return util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return nil
}

func textureNotFoundError(err error) error {
	if errors.Is(err, texture.ErrNotFound) {
		return util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return err
}

func textureCursor(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	value, ok := util.CursorInt64(m["last_created_at"])
	h, hashOK := m[hashKey].(string)
	if !ok || !hashOK || h == "" {
		return nil, "", errors.New("invalid cursor")
	}
	created := &value
	return created, h, nil
}

func publicLibraryCursor(cursor string) (*int64, string, *int64, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", nil, err
	}
	createdValue, ok := util.CursorInt64(m["last_created_at"])
	h, hashOK := m["last_skin_hash"].(string)
	if !ok || !hashOK || h == "" {
		return nil, "", nil, errors.New("invalid cursor")
	}
	created := &createdValue
	var usage *int64
	if rawUsage, exists := m["last_usage_count"]; exists {
		usageValue, ok := util.CursorInt64(rawUsage)
		if !ok {
			return nil, "", nil, errors.New("invalid cursor")
		}
		usage = &usageValue
	}
	return created, h, usage, nil
}
