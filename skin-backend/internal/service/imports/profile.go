package imports

import (
	"context"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s ImportService) ImportProfile(ctx context.Context, actor permission.Actor, profileID, profileName string, assets []TextureAsset) (map[string]any, error) {
	if !actor.Has(profileCreateOwnedPermission) {
		return nil, util.HTTPError{Status: 403, Object: "permission", Operation: "check", Reason: "denied"}
	}
	if hasTextureAsset(assets) && !actor.Has(textureCreateOwnedPermission) {
		return nil, util.HTTPError{Status: 403, Object: "permission", Operation: "check", Reason: "denied"}
	}
	if profileID == "" || profileName == "" {
		return nil, util.HTTPError{Status: 400, Object: "profile", Operation: "import", Reason: "required"}
	}
	userID := actor.UserID
	if !util.ValidProfileName(profileName) {
		return nil, util.HTTPError{Status: 400, Object: "profile_name", Operation: "validate", Reason: "invalid"}
	}
	existing, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, util.HTTPError{Status: 400, Object: "profile_uuid", Operation: "reserve", Reason: "conflict"}
	}
	skinHash, capeHash, modelName := s.importTextureAssets(ctx, actor, assets)

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
			return map[string]any{"profile": profile.Summary(p)}, nil
		case profile.IsNameConflict(err):
			continue
		case profile.IsIDConflict(err):
			return nil, util.HTTPError{Status: 400, Object: "profile_uuid", Operation: "reserve", Reason: "conflict"}
		default:
			return nil, err
		}
	}
	return nil, util.HTTPError{Status: 500, Object: "profile_name", Operation: "allocate", Reason: "failed"}
}
