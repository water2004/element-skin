package profile

import (
	"context"
	"net/http"

	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) UpdateProfile(ctx context.Context, actor permission.Actor, profileID, name string) error {
	if err := requireActorPermission(actor, profileUpdateOwnedPermission); err != nil {
		return err
	}
	validName := util.ValidProfileName(name)
	result, err := s.DB.Profiles.UpdateOwnedName(ctx, profileID, actor.UserID, name, validName)
	if profilestore.IsNameConflict(err) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "reserve", Reason: "conflict"}
	}
	if err != nil {
		return err
	}
	if !result.Found {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	if !result.Owned {
		return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
	}
	if name == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "validate", Reason: "required"}
	}
	if !validName {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "validate", Reason: "invalid"}
	}
	if !result.Updated {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}

func (s Service) UpdateAnyProfile(ctx context.Context, actor permission.Actor, profileID, name string) error {
	if err := requireActorPermission(actor, profileUpdateAnyPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	if name == "" {
		return nil
	}
	if !util.ValidProfileName(name) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "profile_name", Operation: "validate", Reason: "invalid"}
	}
	ok, err := s.DB.Profiles.UpdateName(ctx, profileID, name)
	if profilestore.IsNameConflict(err) {
		return util.HTTPError{Status: http.StatusConflict, Object: "profile_name", Operation: "reserve", Reason: "conflict"}
	}
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}

func (s Service) DeleteProfile(ctx context.Context, actor permission.Actor, profileID string) error {
	if err := requireActorPermission(actor, profileDeleteOwnedPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	if p.UserID != actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
	}
	return s.deleteProfile(ctx, profileID)
}

func (s Service) DeleteProfileByID(ctx context.Context, actor permission.Actor, profileID string) error {
	if err := requireActorPermission(actor, profileDeleteAnyPermission); err != nil {
		return err
	}
	return s.deleteProfile(ctx, profileID)
}

func (s Service) deleteProfile(ctx context.Context, profileID string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	ok, err := s.DB.Profiles.DeleteCascade(ctx, profileID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "profile", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}
