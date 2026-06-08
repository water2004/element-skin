package settings

import (
	"context"
	"encoding/json"
	"fmt"

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
		if err := s.DB.Settings.Set(ctx, key, value); err != nil {
			return err
		}
	}
	if group == "fallback" {
		if raw, ok := body["fallbacks"]; ok {
			fallbacks, err := ValidateFallbackEndpoints(raw)
			if err != nil {
				return err
			}
			if err := s.DB.Fallbacks.SaveEndpoints(ctx, fallbacks); err != nil {
				return err
			}
		}
	}
	return nil
}
