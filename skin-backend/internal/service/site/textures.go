package site

import (
	"context"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

func (s Site) ApplyTextureToProfile(ctx context.Context, userID, profileID, hash, textureType string) error {
	owns, err := s.DB.VerifyTextureOwnership(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !owns {
		return util.HTTPError{Status: 403, Detail: "Texture not found in your library"}
	}
	profileOwner, err := s.DB.VerifyProfileOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !profileOwner {
		return util.HTTPError{Status: 403, Detail: "Profile not yours"}
	}
	info, err := s.DB.GetTextureInfo(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if info == nil {
		return util.HTTPError{Status: 403, Detail: "Texture info not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		if err := s.DB.UpdateProfileSkin(ctx, profileID, &hash); err != nil {
			return err
		}
		model, _ := info["model"].(string)
		return s.DB.UpdateProfileModel(ctx, profileID, database.NormalizeProfileModel(model))
	case "cape":
		return s.DB.UpdateProfileCape(ctx, profileID, &hash)
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
}

func (s Site) TextureDetail(ctx context.Context, userID, hash, textureType string) (map[string]any, error) {
	info, err := s.DB.GetTextureInfo(ctx, userID, hash, textureType)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return info, nil
}

func (s Site) UpdateTexture(ctx context.Context, userID, hash, textureType string, body map[string]any) (map[string]any, error) {
	if v, ok := body["note"].(string); ok {
		if err := s.DB.UpdateTextureNote(ctx, userID, hash, textureType, v); err != nil {
			return nil, err
		}
	}
	if v, ok := body["model"].(string); ok {
		if err := s.DB.UpdateTextureModel(ctx, userID, hash, textureType, v); err != nil {
			return nil, err
		}
	}
	if v, ok := body["is_public"]; ok {
		pub := false
		switch x := v.(type) {
		case bool:
			pub = x
		case float64:
			pub = x != 0
		case int:
			pub = x != 0
		}
		if err := s.DB.UpdateTexturePublic(ctx, userID, hash, textureType, pub); err != nil {
			return nil, err
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
	_, err := s.DB.DeleteTextureFromLibrary(ctx, userID, hash, textureType)
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
