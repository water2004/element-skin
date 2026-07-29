package settings

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	siteSettingsReadPermission   = permission.MustDefinitionByCode("site_settings.read.any")
	siteSettingsUpdatePermission = permission.MustDefinitionByCode("site_settings.update.any")
)

func (s Settings) ReadGroup(ctx context.Context, actor permission.Actor, group string) (map[string]any, error) {
	if err := requirePermission(actor, siteSettingsReadPermission); err != nil {
		return nil, err
	}
	return s.GetGroup(ctx, group)
}

func (s Settings) UpdateGroup(ctx context.Context, actor permission.Actor, group string, body map[string]any) error {
	if err := requirePermission(actor, siteSettingsUpdatePermission); err != nil {
		return err
	}
	return s.SaveGroupAndInvalidate(ctx, group, body)
}

func (s Settings) GetGroup(ctx context.Context, group string) (map[string]any, error) {
	keys, ok := settingsGroups[group]
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "invalid settings group"}
	}
	if group == "easter_eggs" {
		enabled, err := s.DB.EasterEggs.ListEnabled(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"easter_eggs_enabled": enabled}, nil
	}
	out := map[string]any{}
	for _, key := range keys {
		raw, err := s.Get(ctx, key, SettingDefaults[key])
		if err != nil {
			return nil, err
		}
		out[key] = settingValue(key, raw)
	}
	if group == "fallback" {
		fallbacks, err := s.DB.Fallbacks.ListEndpoints(ctx)
		if err != nil {
			return nil, err
		}
		out["fallbacks"] = fallbacks
	}
	return out, nil
}

func (s Settings) SaveGroup(ctx context.Context, group string, body map[string]any) error {
	keys, ok := settingsGroups[group]
	if !ok {
		return util.HTTPError{Status: 400, Detail: "invalid settings group"}
	}
	if group == "easter_eggs" {
		raw, ok := body["easter_eggs_enabled"]
		if !ok {
			return nil
		}
		enabled, err := ValidateEasterEggs(raw)
		if err != nil {
			return err
		}
		return s.DB.EasterEggs.ReplaceEnabled(ctx, enabled)
	}
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
	}
	pending := make(map[string]any, len(keys))
	for key, value := range body {
		if !allowed[key] {
			continue
		}
		if key == "profile_uuid_mode" {
			mode := fmt.Sprint(value)
			if mode != "random" && mode != "offline" {
				return util.HTTPError{Status: 400, Detail: "invalid profile_uuid_mode"}
			}
		}
		if key == "fallback_strategy" {
			strategy := fmt.Sprint(value)
			if strategy != "serial" && strategy != "parallel" {
				return util.HTTPError{Status: 400, Detail: "fallback_strategy must be serial or parallel"}
			}
		}
		if key == "smtp_password" && fmt.Sprint(value) == "" {
			continue
		}
		if key == "fallback_probe_interval" {
			n, err := strconv.Atoi(fmt.Sprint(value))
			if err != nil || n < 60 || n > 86400 {
				return util.HTTPError{Status: 400, Detail: "fallback_probe_interval must be between 60 and 86400 seconds"}
			}
			value = strconv.Itoa(n)
		}
		pending[key] = value
	}
	var pendingFallbacks []fallback.Endpoint
	saveFallbacks := false
	if group == "fallback" {
		if raw, ok := body["fallbacks"]; ok {
			fallbacks, err := ValidateFallbackEndpoints(raw)
			if err != nil {
				return err
			}
			pendingFallbacks = fallbacks
			saveFallbacks = true
		}
	}
	updates := make([]database.SettingUpdate, 0, len(pending))
	for _, key := range keys {
		value, ok := pending[key]
		if !ok {
			continue
		}
		updates = append(updates, database.SettingUpdate{Key: key, Value: value})
	}
	err := s.DB.SaveSettingsGroup(ctx, updates, pendingFallbacks, saveFallbacks)
	if errors.Is(err, fallback.ErrEndpointNotFound) {
		return util.HTTPError{Status: 400, Detail: "fallback endpoint not found"}
	}
	if errors.Is(err, fallback.ErrDuplicateEndpoint) {
		return util.HTTPError{Status: 400, Detail: "fallback endpoint id is duplicated"}
	}
	return err
}

func (s Settings) SaveGroupAndInvalidate(ctx context.Context, group string, body map[string]any) error {
	if err := s.SaveGroup(ctx, group, body); err != nil {
		return err
	}
	if err := s.InvalidateCache(ctx); err != nil {
		return err
	}
	switch group {
	case "site", "fallback", "email", "easter_eggs":
		if err := s.Redis.InvalidatePublicSettings(ctx); err != nil {
			return err
		}
	}
	if group == "fallback" {
		if err := s.Redis.InvalidateFallbackPublicKeys(ctx); err != nil {
			return err
		}
	}
	return nil
}

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: 403, Detail: "permission denied"}
}
