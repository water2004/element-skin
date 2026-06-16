package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/util"
)

func (s Settings) GetGroup(ctx context.Context, group string) (map[string]any, error) {
	keys, ok := settingsGroups[group]
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "invalid settings group"}
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
		if key == "smtp_password" && fmt.Sprint(value) == "" {
			continue
		}
		if key == "fallback_services" {
			normalized, err := ValidateFallbackServices(value)
			if err != nil {
				return err
			}
			b, err := json.Marshal(normalized)
			if err != nil {
				return err
			}
			value = string(b)
		}
		if key == "fallback_probe_interval" {
			n, err := strconv.Atoi(fmt.Sprint(value))
			if err != nil || n < 60 || n > 86400 {
				return util.HTTPError{Status: 400, Detail: "fallback_probe_interval must be between 60 and 86400 seconds"}
			}
			value = strconv.Itoa(n)
		}
		if key == "easter_eggs_enabled" {
			normalized, err := ValidateEasterEggs(value)
			if err != nil {
				return err
			}
			b, err := json.Marshal(normalized)
			if err != nil {
				return err
			}
			value = string(b)
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
	return s.DB.SaveSettingsGroup(ctx, updates, pendingFallbacks, saveFallbacks)
}
