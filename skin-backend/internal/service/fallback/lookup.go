package fallback

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"element-skin/backend/internal/util"
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
		return f.get(ctx, ep, u+"?"+q.Encode())
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
		return f.get(ctx, ep, u)
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
		return f.get(ctx, ep, u)
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
		return f.get(ctx, ep, u)
	}
	return f.dispatch(ctx, eps, strategy, call)
}

func (f Fallback) BulkLookup(ctx context.Context, names []string, api LookupAPI) ([]map[string]any, error) {
	eps, err := f.enabledEndpoints(ctx, "profile")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, err := f.settings().Get(ctx, "fallback_strategy", "serial")
	if err != nil {
		return nil, err
	}
	call := func(ep map[string]any) (*FallbackResponse, error) {
		baseKey, path := "account_url", "/profiles/minecraft"
		if api == ServicesAPI {
			baseKey, path = "services_url", "/minecraft/profile/lookup/bulk/byname"
		}
		baseURL := strings.TrimRight(ep[baseKey].(string), "/")
		if baseURL == "" {
			return nil, nil
		}
		return f.postJSON(ctx, ep, baseURL+path, names)
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

func (f Fallback) LookupNames(ctx context.Context, names []string, api LookupAPI) ([]map[string]any, error) {
	limit := 100
	if api == ServicesAPI {
		limit = 10
	}
	if names == nil || len(names) > limit || (api == ServicesAPI && len(names) == 0) {
		return nil, util.YggError{Status: 400, Code: "IllegalArgumentException", Message: "Invalid number of profile names."}
	}
	unique := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" {
			return nil, util.YggError{Status: 400, Code: "IllegalArgumentException", Message: "Profile names must not be empty."}
		}
		key := strings.ToLower(name)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, name)
		}
	}
	names = unique
	profiles, err := f.DB.Profiles.SearchByNames(ctx, names, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, map[string]any{"id": p.ID, "name": p.Name})
	}
	found := map[string]bool{}
	for _, p := range profiles {
		found[strings.ToLower(p.Name)] = true
	}
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if !found[strings.ToLower(name)] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	fallbackProfiles, err := f.BulkLookup(ctx, missing, api)
	if err != nil {
		return nil, err
	}
	if len(fallbackProfiles) > 0 {
		out = append(out, fallbackProfiles...)
	}
	return out, nil
}

// LookupAPI selects the upstream protocol and its request limit.
type LookupAPI int

const (
	AccountAPI LookupAPI = iota
	ServicesAPI
)
