package profile

import (
	"context"
	"net/http"
	"regexp"

	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) CreateProfile(ctx context.Context, actor permission.Actor, name, modelName string) (map[string]any, error) {
	if err := requireActorPermission(actor, profileCreateOwnedPermission); err != nil {
		return nil, err
	}
	userID := actor.UserID
	if name == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "validate", Reason: "required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "validate", Reason: "invalid"}
	}
	if p, err := s.DB.Profiles.GetByName(ctx, name); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "reserve", Reason: "conflict"}
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	mode, err := s.Settings.Get(ctx, "profile_uuid_mode", "random")
	if err != nil {
		return nil, err
	}
	if mode == "offline" {
		id = util.OfflineUUIDNoDash(name)
	}
	if p, err := s.DB.Profiles.GetByID(ctx, id); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "profile_uuid", Operation: "reserve", Reason: "conflict"}
	}
	modelName = profilestore.NormalizeModel(modelName)
	if err := s.DB.Profiles.Create(ctx, model.Profile{ID: id, UserID: userID, Name: name, TextureModel: modelName}); err != nil {
		if profilestore.IsNameConflict(err) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "reserve", Reason: "conflict"}
		}
		if profilestore.IsIDConflict(err) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "profile_uuid", Operation: "reserve", Reason: "conflict"}
		}
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "model": modelName}, nil
}
