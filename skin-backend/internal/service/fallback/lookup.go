package fallback

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

func (f Fallback) HasJoined(ctx context.Context, username, serverID, ip string) (*FallbackResponse, error) {
	eps, err := f.enabledEndpoints(ctx, "hasJoined")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, err := f.settings().Get(ctx, "fallback_strategy", "serial")
	if err != nil {
		return nil, err
	}
	call := func(ep map[string]any) (*FallbackResponse, error) {
		if ep["enable_whitelist"].(bool) {
			ok, err := f.DB.Fallbacks.IsUserInWhitelist(ctx, username, ep["id"].(int))
			if err != nil || !ok {
				return nil, err
			}
		}
		u := strings.TrimRight(ep["session_url"].(string), "/") + "/session/minecraft/hasJoined"
		q := url.Values{"username": {username}, "serverId": {serverID}}
		if ip != "" {
			q.Set("ip", ip)
		}
		return f.get(ctx, u+"?"+q.Encode())
	}
	return f.dispatch(ctx, eps, strategy, call)
}

func (f Fallback) GetProfile(ctx context.Context, uuid string, unsigned bool) (*FallbackResponse, error) {
	eps, err := f.enabledEndpoints(ctx, "profile")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, err := f.settings().Get(ctx, "fallback_strategy", "serial")
	if err != nil {
		return nil, err
	}
	call := func(ep map[string]any) (*FallbackResponse, error) {
		u := strings.TrimRight(ep["session_url"].(string), "/") + "/session/minecraft/profile/" + uuid
		u += "?unsigned=" + strconv.FormatBool(unsigned)
		return f.get(ctx, u)
	}
	return f.dispatch(ctx, eps, strategy, call)
}

func (f Fallback) GetProfileByName(ctx context.Context, playerName string) (*FallbackResponse, error) {
	eps, err := f.enabledEndpoints(ctx, "profile")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, err := f.settings().Get(ctx, "fallback_strategy", "serial")
	if err != nil {
		return nil, err
	}
	call := func(ep map[string]any) (*FallbackResponse, error) {
		accountURL := strings.TrimRight(ep["account_url"].(string), "/")
		if accountURL == "" {
			return nil, nil
		}
		u := accountURL + "/users/profiles/minecraft/" + url.PathEscape(playerName)
		return f.get(ctx, u)
	}
	return f.dispatch(ctx, eps, strategy, call)
}

func (f Fallback) ServicesLookup(ctx context.Context, playerName string) (*FallbackResponse, error) {
	eps, err := f.enabledEndpoints(ctx, "profile")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, err := f.settings().Get(ctx, "fallback_strategy", "serial")
	if err != nil {
		return nil, err
	}
	call := func(ep map[string]any) (*FallbackResponse, error) {
		servicesURL := strings.TrimRight(ep["services_url"].(string), "/")
		if servicesURL == "" {
			return nil, nil
		}
		u := servicesURL + "/minecraft/profile/lookup/name/" + url.PathEscape(playerName)
		return f.get(ctx, u)
	}
	return f.dispatch(ctx, eps, strategy, call)
}

func (f Fallback) BulkLookup(ctx context.Context, names []string) ([]map[string]any, error) {
	eps, err := f.enabledEndpoints(ctx, "profile")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, err := f.settings().Get(ctx, "fallback_strategy", "serial")
	if err != nil {
		return nil, err
	}
	call := func(ep map[string]any) (*FallbackResponse, error) {
		accountURL := strings.TrimRight(ep["account_url"].(string), "/")
		if accountURL == "" {
			return nil, nil
		}
		return f.postJSON(ctx, accountURL+"/profiles/minecraft", names)
	}
	resp, err := f.dispatch(ctx, eps, strategy, call)
	if err != nil || resp == nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
