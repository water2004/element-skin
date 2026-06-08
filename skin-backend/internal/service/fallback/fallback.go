package fallback

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"element-skin/backend/internal/database"
)

type Fallback struct {
	DB     *database.DB
	Client *http.Client
}

type FallbackResponse struct {
	Status int
	Body   []byte
}

func (f Fallback) HasJoined(ctx context.Context, username, serverID, ip string) (*FallbackResponse, error) {
	eps, err := f.enabledEndpoints(ctx, "hasJoined")
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	strategy, _ := f.DB.GetSetting(ctx, "fallback_strategy", "serial")
	call := func(ep map[string]any) (*FallbackResponse, error) {
		if ep["enable_whitelist"].(bool) {
			ok, err := f.DB.IsUserInWhitelist(ctx, username, ep["id"].(int))
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
	strategy, _ := f.DB.GetSetting(ctx, "fallback_strategy", "serial")
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
	strategy, _ := f.DB.GetSetting(ctx, "fallback_strategy", "serial")
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
	strategy, _ := f.DB.GetSetting(ctx, "fallback_strategy", "serial")
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
	strategy, _ := f.DB.GetSetting(ctx, "fallback_strategy", "serial")
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

func (f Fallback) enabledEndpoints(ctx context.Context, kind string) ([]map[string]any, error) {
	eps, err := f.DB.ListFallbackEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(eps))
	for _, ep := range eps {
		switch kind {
		case "profile":
			if ep["enable_profile"].(bool) {
				out = append(out, ep)
			}
		case "hasJoined":
			if ep["enable_hasjoined"].(bool) {
				out = append(out, ep)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i]["priority"].(int) < out[j]["priority"].(int)
	})
	return out, nil
}

func (f Fallback) dispatch(ctx context.Context, eps []map[string]any, strategy string, call func(map[string]any) (*FallbackResponse, error)) (*FallbackResponse, error) {
	if strategy != "parallel" {
		for _, ep := range eps {
			resp, err := call(ep)
			if err != nil {
				continue
			}
			if resp != nil {
				return resp, nil
			}
		}
		return nil, nil
	}
	type result struct {
		resp *FallbackResponse
		err  error
	}
	ch := make(chan result, len(eps))
	var wg sync.WaitGroup
	for _, ep := range eps {
		ep := ep
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := call(ep)
			ch <- result{resp: resp, err: err}
		}()
	}
	wg.Wait()
	close(ch)
	for r := range ch {
		if r.err != nil {
			continue
		}
		if r.resp != nil {
			return r.resp, nil
		}
	}
	return nil, nil
}

func (f Fallback) get(ctx context.Context, rawURL string) (*FallbackResponse, error) {
	return f.do(ctx, http.MethodGet, rawURL, nil)
}

func (f Fallback) postJSON(ctx context.Context, rawURL string, body any) (*FallbackResponse, error) {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return nil, err
	}
	return f.do(ctx, http.MethodPost, rawURL, &b)
}

func (f Fallback) do(ctx context.Context, method, rawURL string, reqBody io.Reader) (*FallbackResponse, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reqBody)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &FallbackResponse{Status: resp.StatusCode, Body: respBody}, nil
}
