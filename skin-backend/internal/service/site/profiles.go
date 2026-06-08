package site

import (
	"context"
	"regexp"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func (s Site) CreateProfile(ctx context.Context, userID, name, mdl string) (map[string]any, error) {
	if name == "" {
		return nil, util.HTTPError{Status: 400, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return nil, util.HTTPError{Status: 400, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p, err := s.DB.GetProfileByName(ctx, name); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色名已被占用，请换一个名称"}
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	mode, _ := s.DB.GetSetting(ctx, "profile_uuid_mode", "random")
	if mode == "offline" {
		id = util.OfflineUUIDNoDash(name)
	}
	if p, err := s.DB.GetProfileByID(ctx, id); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
	}
	mdl = database.NormalizeProfileModel(mdl)
	if err := s.DB.CreateProfile(ctx, model.Profile{ID: id, UserID: userID, Name: name, TextureModel: mdl}); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "model": mdl}, nil
}

func (s Site) PublicLibrary(ctx context.Context, cursor string, limit int, typ, q string) (map[string]any, error) {
	if enabled, _ := s.DB.GetSetting(ctx, "enable_skin_library", "true"); enabled != "true" {
		return nil, util.HTTPError{Status: 403, Detail: "Skin library is disabled by administrator"}
	}
	lastCreated, lastHash, err := textureCursor(cursor, "last_skin_hash")
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.ListPublicLibrary(ctx, limit, typ, strings.TrimSpace(q), lastCreated, lastHash)
}

func (s Site) ListMyProfiles(ctx context.Context, userID, cursor string, limit int) (map[string]any, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	res, err := s.DB.ListProfilesByUser(ctx, userID, limit, last)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Site) ListMyTextures(ctx context.Context, userID, cursor string, limit int, typ string) (map[string]any, error) {
	lastCreated, lastHash, err := textureCursor(cursor, "last_hash")
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.ListUserTextures(ctx, userID, typ, limit, lastCreated, lastHash)
}

func (s Site) AddTextureToWardrobe(ctx context.Context, userID, hash string) error {
	ok, err := s.DB.AddTextureToWardrobe(ctx, userID, hash)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 404, Detail: "Texture not found in library"}
	}
	return nil
}

func (s Site) UpdateProfile(ctx context.Context, userID, profileID, name string) error {
	p, err := s.DB.GetProfileByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	if name == "" {
		return util.HTTPError{Status: 400, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return util.HTTPError{Status: 400, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p.Name != name {
		existing, err := s.DB.GetProfileByName(ctx, name)
		if err != nil {
			return err
		}
		if existing != nil {
			return util.HTTPError{Status: 400, Detail: "角色名已被占用"}
		}
	}
	_, err = s.DB.UpdateProfileName(ctx, profileID, name)
	return err
}

func (s Site) DeleteProfile(ctx context.Context, userID, profileID string) error {
	p, err := s.DB.GetProfileByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	_, err = s.DB.DeleteProfileCascade(ctx, profileID)
	return err
}

func (s Site) ClearProfileTexture(ctx context.Context, userID, profileID, textureType string) error {
	ok, err := s.DB.VerifyProfileOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		return s.DB.UpdateProfileSkin(ctx, profileID, nil)
	case "cape":
		return s.DB.UpdateProfileCape(ctx, profileID, nil)
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
}
