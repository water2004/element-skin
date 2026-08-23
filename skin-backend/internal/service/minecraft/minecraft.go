package minecraft

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"
)

const maxBulkNames = 100

var (
	hasJoinedPermission             = permission.MustDefinitionByCode("minecraft_session.hasjoined.server")
	minecraftProfileReadPermission  = permission.MustDefinitionByCode("minecraft_profile.read.public")
	minecraftTexturesReadPermission = permission.MustDefinitionByCode("minecraft_texture_property.read.public")
)

type Service struct {
	DB  *database.DB
	Ygg yggsvc.Yggdrasil
}

type HasJoinedRequest struct {
	Username string
	ServerID string
	IP       string
}

func (s Service) ProfileByName(ctx context.Context, actor permission.Actor, name string) (map[string]any, error) {
	if err := requirePermission(actor, minecraftProfileReadPermission); err != nil {
		return nil, err
	}
	profile, err := s.DB.Profiles.GetByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, notFound("minecraft_profile", "resolve", "not_found")
	}
	return publicProfile(*profile, true), nil
}

func (s Service) ProfilesByNames(ctx context.Context, actor permission.Actor, names []string) (map[string]any, error) {
	if err := requirePermission(actor, minecraftProfileReadPermission); err != nil {
		return nil, err
	}
	if len(names) > maxBulkNames {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "minecraft_profile", Operation: "read", Reason: "exceeded"}
	}
	profiles, err := s.DB.Profiles.SearchByNames(ctx, names, maxBulkNames)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, publicProfile(profile, false))
	}
	return map[string]any{"items": items}, nil
}

func (s Service) ProfileByID(ctx context.Context, actor permission.Actor, profileID string) (map[string]any, error) {
	if err := requirePermission(actor, minecraftProfileReadPermission); err != nil {
		return nil, err
	}
	profile, err := s.DB.Profiles.GetByID(ctx, util.StripUUIDDashes(profileID))
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, notFound("minecraft_profile", "resolve", "not_found")
	}
	return publicProfile(*profile, false), nil
}

func (s Service) TexturesProperty(ctx context.Context, actor permission.Actor, profileID string) (map[string]any, error) {
	if err := requirePermission(actor, minecraftTexturesReadPermission); err != nil {
		return nil, err
	}
	profile, err := s.DB.Profiles.GetByID(ctx, util.StripUUIDDashes(profileID))
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, notFound("minecraft_profile", "resolve", "not_found")
	}
	property, err := s.texturesProperty(*profile)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"profile_id":        profile.ID,
		"profile_name":      profile.Name,
		"textures_property": property,
	}, nil
}

func (s Service) HasJoined(ctx context.Context, actor permission.Actor, req HasJoinedRequest) (map[string]any, error) {
	if actor.SessionKind != permission.SessionKindClient || actor.Entrypoint != permission.EntrypointAPI || actor.UserID != "" {
		return nil, forbidden()
	}
	if err := actor.Require(hasJoinedPermission); err != nil {
		return nil, forbidden()
	}
	username := strings.TrimSpace(req.Username)
	serverID := strings.TrimSpace(req.ServerID)
	if username == "" || serverID == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "minecraft_session", Operation: "join", Reason: "required"}
	}
	body, status, err := s.Ygg.HasJoined(ctx, username, serverID)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return map[string]any{"joined": false, "profile": nil}, nil
	}
	property := firstTexturesProperty(body)
	profile := map[string]any{
		"id":                body["id"],
		"name":              body["name"],
		"textures_property": property,
	}
	return map[string]any{"joined": true, "profile": profile}, nil
}

func (s Service) texturesProperty(profile model.Profile) (map[string]any, error) {
	body, err := s.Ygg.ProfileJSON(profile, true)
	if err != nil {
		return nil, err
	}
	property := firstTexturesProperty(body)
	if property == nil {
		return nil, util.HTTPError{Status: http.StatusInternalServerError, Object: "minecraft_profile", Operation: "read", Reason: "incomplete"}
	}
	return property, nil
}

func publicProfile(profile model.Profile, includeOwner bool) map[string]any {
	out := map[string]any{
		"id":            profile.ID,
		"name":          profile.Name,
		"texture_model": profile.TextureModel,
		"public":        true,
	}
	if includeOwner {
		out["owner_user_id"] = profile.UserID
	}
	return out
}

func firstTexturesProperty(body map[string]any) map[string]any {
	props, ok := body["properties"].([]map[string]any)
	if !ok {
		return nil
	}
	for _, prop := range props {
		if prop["name"] == "textures" {
			return prop
		}
	}
	return nil
}

func forbidden() error {
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if err := actor.Require(def); err != nil {
		return forbidden()
	}
	return nil
}

func notFound(object, operation, reason string) error {
	return util.HTTPError{Status: http.StatusNotFound, Object: object, Operation: operation, Reason: reason}
}
