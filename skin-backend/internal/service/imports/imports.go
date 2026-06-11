package imports

import (
	"context"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

type TextureAsset struct {
	URL     string
	Kind    string
	Variant string
}

type ImportService struct {
	DB              *database.DB
	DownloadTexture func(context.Context, string) ([]byte, error)
	ProcessTexture  func([]byte, string) (string, error)
}

func (s ImportService) ImportProfile(ctx context.Context, userID, profileID, profileName string, assets []TextureAsset) (map[string]any, error) {
	if profileID == "" || profileName == "" {
		return nil, util.HTTPError{Status: 400, Detail: "profile_id and profile_name are required"}
	}
	if !util.ValidProfileName(profileName) {
		return nil, util.HTTPError{Status: 400, Detail: "invalid profile name"}
	}
	existing, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, util.HTTPError{Status: 400, Detail: "UUID already exists"}
	}
	modelName := "default"
	var skinHash *string
	var capeHash *string
	for _, asset := range assets {
		if asset.URL == "" {
			continue
		}
		data, err := s.download(ctx, asset.URL)
		if err != nil {
			continue
		}
		hash, err := s.process(data, asset.Kind)
		if err != nil {
			continue
		}
		if asset.Kind == "skin" {
			skinHash = &hash
			if asset.Variant == "slim" {
				modelName = "slim"
			}
		}
		if asset.Kind == "cape" {
			capeHash = &hash
		}
	}

	for attempt := 0; attempt < 100; attempt++ {
		name := util.ProfileNameCandidate(profileName, attempt)
		existing, err := s.DB.Profiles.GetByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			continue
		}

		p := model.Profile{ID: profileID, UserID: userID, Name: name, TextureModel: modelName, SkinHash: skinHash, CapeHash: capeHash}
		err = s.DB.Profiles.Create(ctx, p)
		switch {
		case err == nil:
			return map[string]any{"ok": true, "profile": profile.Summary(p)}, nil
		case profile.IsNameConflict(err):
			continue
		case profile.IsIDConflict(err):
			return nil, util.HTTPError{Status: 400, Detail: "UUID already exists"}
		default:
			return nil, err
		}
	}
	return nil, util.HTTPError{Status: 500, Detail: "无法生成唯一角色名"}
}

func (s ImportService) ImportProfiles(ctx context.Context, userID string, profiles []map[string]string, fetch func(context.Context, string) ([]TextureAsset, error)) map[string]any {
	var items []map[string]any
	var failed []map[string]any
	for _, p := range profiles {
		id := p["profile_id"]
		name := p["profile_name"]
		if id == "" || name == "" {
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": "profile_id and profile_name are required"})
			continue
		}
		assets, err := fetch(ctx, id)
		if err != nil {
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": "导入失败"})
			continue
		}
		res, err := s.ImportProfile(ctx, userID, id, name, assets)
		if err != nil {
			detail := "导入失败"
			if he, ok := err.(util.HTTPError); ok {
				detail = he.Detail
			}
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": detail})
			continue
		}
		items = append(items, res["profile"].(map[string]any))
	}
	return map[string]any{
		"success_count": len(items),
		"failure_count": len(failed),
		"items":         items,
		"failed":        failed,
	}
}

func (s ImportService) download(ctx context.Context, rawURL string) ([]byte, error) {
	if s.DownloadTexture != nil {
		return s.DownloadTexture(ctx, rawURL)
	}
	return []byte(rawURL), nil
}

func (s ImportService) process(data []byte, kind string) (string, error) {
	if s.ProcessTexture != nil {
		return s.ProcessTexture(data, kind)
	}
	return util.HashRefreshToken(string(data) + ":" + kind), nil
}
