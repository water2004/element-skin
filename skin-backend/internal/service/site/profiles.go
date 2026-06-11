package site

import (
	"context"
	"regexp"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/database/texture"
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
	if p, err := s.DB.Profiles.GetByName(ctx, name); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色名已被占用，请换一个名称"}
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	mode, err := s.settings().Get(ctx, "profile_uuid_mode", "random")
	if err != nil {
		return nil, err
	}
	if mode == "offline" {
		id = util.OfflineUUIDNoDash(name)
	}
	if p, err := s.DB.Profiles.GetByID(ctx, id); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
	}
	mdl = profile.NormalizeModel(mdl)
	if err := s.DB.Profiles.Create(ctx, model.Profile{ID: id, UserID: userID, Name: name, TextureModel: mdl}); err != nil {
		if profile.IsNameConflict(err) {
			return nil, util.HTTPError{Status: 400, Detail: "角色名已被占用，请换一个名称"}
		}
		if profile.IsIDConflict(err) {
			return nil, util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
		}
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "model": mdl}, nil
}

func (s Site) PublicLibrary(ctx context.Context, cursor string, limit int, typ, q, sort string) (map[string]any, error) {
	enabled, err := s.settings().Get(ctx, "enable_skin_library", "true")
	if err != nil {
		return nil, err
	}
	if enabled != "true" {
		return nil, util.HTTPError{Status: 403, Detail: "Skin library is disabled by administrator"}
	}
	lastCreated, lastHash, lastUsage, err := publicLibraryCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	parsedSort := texture.ParsePublicLibrarySort(sort)
	if cursor != "" && (lastCreated == nil || lastHash == "" ||
		(parsedSort == texture.PublicLibrarySortMostUsed && lastUsage == nil)) {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.Textures.ListPublic(ctx, texture.PublicListOptions{
		Limit:       limit,
		TextureType: typ,
		Query:       strings.TrimSpace(q),
		Sort:        parsedSort,
		LastCreated: lastCreated,
		LastHash:    lastHash,
		LastUsage:   lastUsage,
	})
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
	if cursor != "" && last == "" {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	res, err := s.DB.Profiles.ListByUser(ctx, userID, limit, last)
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
	if cursor != "" && (lastCreated == nil || lastHash == "") {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.Textures.ListForUser(ctx, userID, typ, limit, lastCreated, lastHash)
}

func (s Site) AddTextureToWardrobe(ctx context.Context, userID, hash, textureType string) error {
	ok, err := s.DB.Textures.AddToWardrobe(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 404, Detail: "Texture not found in library"}
	}
	return nil
}

func (s Site) UpdateProfile(ctx context.Context, userID, profileID, name string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
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
		existing, err := s.DB.Profiles.GetByName(ctx, name)
		if err != nil {
			return err
		}
		if existing != nil {
			return util.HTTPError{Status: 400, Detail: "角色名已被占用"}
		}
	}
	updated, err := s.DB.Profiles.UpdateName(ctx, profileID, name)
	if profile.IsNameConflict(err) {
		return util.HTTPError{Status: 400, Detail: "角色名已被占用"}
	}
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	return nil
}

func (s Site) DeleteProfile(ctx context.Context, userID, profileID string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	return s.deleteProfile(ctx, profileID)
}

func (s Site) ClearProfileTexture(ctx context.Context, userID, profileID, textureType string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	return s.SetProfileTexture(ctx, profileID, textureType, nil)
}

func (s Site) SetProfileTexture(ctx context.Context, profileID, textureType string, hash *string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		if sameHash(p.SkinHash, hash) {
			return nil
		}
		if err := s.DB.Profiles.UpdateSkin(ctx, profileID, hash); err != nil {
			return profileUpdateError(err)
		}
	case "cape":
		if sameHash(p.CapeHash, hash) {
			return nil
		}
		if err := s.DB.Profiles.UpdateCape(ctx, profileID, hash); err != nil {
			return profileUpdateError(err)
		}
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	return nil
}

func (s Site) DeleteProfileByID(ctx context.Context, profileID string) error {
	return s.deleteProfile(ctx, profileID)
}

func (s Site) DeleteUser(ctx context.Context, userID string) (bool, error) {
	if err := s.Redis.DeleteYggTokensByUser(ctx, userID); err != nil {
		return false, err
	}
	return s.DB.Users.Delete(ctx, userID)
}

func (s Site) deleteProfile(ctx context.Context, profileID string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	ok, err := s.DB.Profiles.DeleteCascade(ctx, profileID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
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
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	return err
}
