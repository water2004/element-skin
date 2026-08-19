package imports

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

type RemoteYggService struct {
	DB          *database.DB
	TexturesDir string
	HTTPClient  *http.Client
}

type RemoteYggProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s RemoteYggService) PreviewProfiles(ctx context.Context, actor permission.Actor, apiURL, username, password string) ([]RemoteYggProfile, error) {
	if err := requireImportPermission(actor, profileCreateOwnedPermission); err != nil {
		return nil, err
	}
	apiURL = strings.TrimSpace(apiURL)
	username = strings.TrimSpace(username)
	if apiURL == "" || username == "" || password == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "credentials", Operation: "validate", Reason: "required"}
	}
	var out struct {
		AvailableProfiles []RemoteYggProfile `json:"availableProfiles"`
	}
	if err := s.doJSON(ctx, http.MethodPost, remoteYggURL(apiURL, "authserver/authenticate"), map[string]any{
		"username": username,
		"password": password,
		"agent": map[string]any{
			"name":    "Minecraft",
			"version": 1,
		},
		"requestUser": true,
	}, &out); err != nil {
		return nil, err
	}
	profiles := make([]RemoteYggProfile, 0, len(out.AvailableProfiles))
	for _, profile := range out.AvailableProfiles {
		profile.ID = strings.TrimSpace(profile.ID)
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.ID == "" || profile.Name == "" {
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (s RemoteYggService) ImportProfile(ctx context.Context, actor permission.Actor, apiURL, profileID, profileName string) (map[string]any, error) {
	if err := requireRemoteImportPermissions(actor); err != nil {
		return nil, err
	}
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "api_url", Operation: "validate", Reason: "required"}
	}
	assets, err := s.FetchTextureAssets(ctx, apiURL, profileID)
	if err != nil {
		return nil, err
	}
	return (ImportService{DB: s.DB, TexturesDir: s.TexturesDir, HTTPClient: s.HTTPClient}).ImportProfile(ctx, actor, profileID, profileName, assets)
}

func (s RemoteYggService) ImportProfiles(ctx context.Context, actor permission.Actor, apiURL string, profiles []map[string]string) (map[string]any, error) {
	if err := requireRemoteImportPermissions(actor); err != nil {
		return nil, err
	}
	importer := ImportService{DB: s.DB, TexturesDir: s.TexturesDir, HTTPClient: s.HTTPClient}
	return importer.ImportProfiles(ctx, actor, profiles, func(ctx context.Context, id string) ([]TextureAsset, error) {
		apiURL = strings.TrimSpace(apiURL)
		if apiURL == "" {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "api_url", Operation: "validate", Reason: "required"}
		}
		return s.FetchTextureAssets(ctx, apiURL, id)
	}), nil
}

func requireRemoteImportPermissions(actor permission.Actor) error {
	if err := requireImportPermission(actor, profileCreateOwnedPermission); err != nil {
		return err
	}
	return requireImportPermission(actor, textureCreateOwnedPermission)
}

func requireImportPermission(actor permission.Actor, definition permission.Definition) error {
	if actor.Has(definition) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func (s RemoteYggService) FetchTextureAssets(ctx context.Context, apiURL, profileID string) ([]TextureAsset, error) {
	profileID = strings.TrimSpace(strings.ReplaceAll(profileID, "-", ""))
	if apiURL == "" || profileID == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "texture_asset", Operation: "validate", Reason: "required"}
	}
	var out remoteYggProfileResponse
	if err := s.doJSON(ctx, http.MethodGet, remoteYggURL(apiURL, "sessionserver/session/minecraft/profile", profileID), nil, &out); err != nil {
		return nil, err
	}
	return out.textureAssets(), nil
}
