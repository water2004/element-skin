package site

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/util"
)

func (s Site) ApplyTextureToProfile(ctx context.Context, userID, profileID, hash, textureType string) error {
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
		model, _ := info["model"].(string)
		return s.DB.Profiles.UpdateSkinAndModel(ctx, profileID, &hash, profile.NormalizeModel(model))
	case "cape":
		return s.SetProfileTexture(ctx, profileID, "cape", &hash)
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
}

func (s Site) TextureDetail(ctx context.Context, userID, hash, textureType string) (map[string]any, error) {
	info, err := s.DB.Textures.GetInfo(ctx, userID, hash, textureType)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return info, nil
}

func (s Site) UpdateTexture(ctx context.Context, userID, hash, textureType string, body map[string]any) (map[string]any, error) {
	var patch texture.Patch
	if model, ok := body["model"].(string); ok && model != "default" && model != "slim" {
		return nil, util.HTTPError{Status: 400, Detail: "invalid model"}
	} else if ok {
		patch.Model = &model
	}
	if note, ok := body["note"].(string); ok {
		patch.Note = &note
	}
	if value, ok := body["is_public"]; ok {
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
	if patch.Note != nil || patch.Model != nil || patch.IsPublic != nil {
		if err := s.DB.Textures.UpdateForUser(ctx, userID, hash, textureType, patch); err != nil {
			return nil, textureNotFoundError(err)
		}
	}
	info, err := s.TextureDetail(ctx, userID, hash, textureType)
	if err != nil {
		return nil, err
	}
	info["ok"] = true
	return info, nil
}

func (s Site) DeleteTexture(ctx context.Context, userID, hash, textureType string) error {
	uploader, exists, err := s.DB.Textures.LibraryUploader(ctx, hash, textureType)
	if err != nil {
		return err
	}
	if exists && uploader == userID {
		return s.DB.Textures.DeleteLibraryTexture(ctx, hash, textureType)
	}
	deleted, err := s.DB.Textures.DeleteFromLibrary(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !deleted {
		return util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return s.DB.Textures.RecountUsage(ctx, hash, textureType)
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
	var created *int64
	switch v := m["last_created_at"].(type) {
	case float64:
		x := int64(v)
		created = &x
	case int64:
		created = &v
	}
	h, _ := m[hashKey].(string)
	return created, h, nil
}

func publicLibraryCursor(cursor string) (*int64, string, *int64, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", nil, err
	}
	var created *int64
	switch v := m["last_created_at"].(type) {
	case float64:
		x := int64(v)
		created = &x
	case int64:
		created = &v
	}
	var usage *int64
	switch v := m["last_usage_count"].(type) {
	case float64:
		x := int64(v)
		usage = &x
	case int64:
		usage = &v
	}
	h, _ := m["last_skin_hash"].(string)
	return created, h, usage, nil
}
