package fallback

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

const fallbackRequestTTL = 30 * time.Second

func (f Fallback) get(ctx context.Context, ep map[string]any, rawURL string) (*FallbackResponse, error) {
	return f.do(ctx, ep, http.MethodGet, rawURL, nil, nil)
}

func (f Fallback) postJSON(ctx context.Context, ep map[string]any, rawURL string, body any) (*FallbackResponse, error) {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return nil, err
	}
	payload := append([]byte(nil), b.Bytes()...)
	return f.do(ctx, ep, http.MethodPost, rawURL, &b, payload)
}

func (f Fallback) do(ctx context.Context, ep map[string]any, method, rawURL string, reqBody io.Reader, payload []byte) (*FallbackResponse, error) {
	marked, endpoint, request, err := f.markRequest(ctx, ep, method, rawURL, payload)
	if err != nil || !marked {
		return nil, err
	}
	if marked && f.Redis != nil {
		defer func() { _ = f.Redis.DeleteFallbackRequest(context.Background(), endpoint, request) }()
	}
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

func (f Fallback) markRequest(ctx context.Context, ep map[string]any, method, rawURL string, payload []byte) (bool, string, string, error) {
	if f.Redis == nil {
		return true, "", "", nil
	}
	endpoint := fallbackEndpointKey(ep)
	request := fallbackRequestKey(method, rawURL, payload)
	ok, err := f.Redis.MarkFallbackRequest(ctx, endpoint, request, fallbackTTL(ep))
	return ok, endpoint, request, err
}

func fallbackEndpointKey(ep map[string]any) string {
	if id, ok := ep["id"].(int); ok && id > 0 {
		return strconv.Itoa(id)
	}
	if sessionURL, _ := ep["session_url"].(string); sessionURL != "" {
		return sessionURL
	}
	if accountURL, _ := ep["account_url"].(string); accountURL != "" {
		return accountURL
	}
	if servicesURL, _ := ep["services_url"].(string); servicesURL != "" {
		return servicesURL
	}
	return "unknown"
}

func fallbackRequestKey(method, rawURL string, payload []byte) string {
	sum := sha256.Sum256(append([]byte(method+"\n"+rawURL+"\n"), payload...))
	return hex.EncodeToString(sum[:])
}

func fallbackTTL(ep map[string]any) time.Duration {
	seconds, _ := ep["cache_ttl"].(int)
	if seconds <= 0 {
		return fallbackRequestTTL
	}
	ttl := time.Duration(seconds) * time.Second
	if ttl < fallbackRequestTTL {
		return fallbackRequestTTL
	}
	return ttl
}
