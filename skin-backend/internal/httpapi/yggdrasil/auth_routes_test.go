package yggdrasil_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestAuthRoutesValidateMissingTokenAndMetadataExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Metadata(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"implementationName"`) || !strings.Contains(rec.Body.String(), `"skinDomains"`) {
		t.Fatalf("metadata response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/authserver/validate", strings.NewReader(`{"accessToken":"missing"}`))
	rec = httptest.NewRecorder()
	h.Validate(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "Invalid token") {
		t.Fatalf("validate missing token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthRoutesAuthenticateRefreshJoinAndHasJoinedFlow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-flow@test.com", "Password123", "YggFlow", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_flow_profile", "YggFlowProfile")

	req := httptest.NewRequest(http.MethodPost, "/authserver/authenticate", strings.NewReader(`{"username":"ygg-flow@test.com","password":"Password123","clientToken":"client-flow","requestUser":true}`))
	rec := httptest.NewRecorder()
	h.Authenticate(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"clientToken":"client-flow"`) ||
		!strings.Contains(rec.Body.String(), `"selectedProfile":{"id":"`+profile.ID+`","name":"YggFlowProfile"}`) ||
		!strings.Contains(rec.Body.String(), `"user"`) {
		t.Fatalf("authenticate response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	authPayload := decodeYggJSON(t, rec.Body.String())
	access, _ := authPayload["accessToken"].(string)
	if _, err := redis.GetYggToken(context.Background(), access); err != nil {
		t.Fatalf("authenticate should store access token in redis: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/authserver/refresh", strings.NewReader(`{"accessToken":"`+access+`","clientToken":"client-flow","requestUser":true}`))
	rec = httptest.NewRecorder()
	h.Refresh(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"clientToken":"client-flow"`) ||
		!strings.Contains(rec.Body.String(), `"selectedProfile":{"id":"`+profile.ID+`","name":"YggFlowProfile"}`) {
		t.Fatalf("refresh response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	refreshPayload := decodeYggJSON(t, rec.Body.String())
	refreshed, _ := refreshPayload["accessToken"].(string)
	if refreshed == "" || refreshed == access {
		t.Fatalf("refresh should rotate access token: old=%q newBody=%q", access, rec.Body.String())
	}
	if _, err := redis.GetYggToken(context.Background(), access); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old access token should be gone from redis after refresh, got %v", err)
	}
	if _, err := redis.GetYggToken(context.Background(), refreshed); err != nil {
		t.Fatalf("new access token should be in redis after refresh: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/sessionserver/session/minecraft/join", strings.NewReader(`{"accessToken":"`+refreshed+`","selectedProfile":"`+profile.ID+`","serverId":"server-flow"}`))
	rec = httptest.NewRecorder()
	h.Join(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("join response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/hasJoined?username=YggFlowProfile&serverId=server-flow", nil)
	rec = httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"YggFlowProfile"`) {
		t.Fatalf("hasJoined response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/authserver/invalidate", strings.NewReader(`{"accessToken":"`+refreshed+`"}`))
	rec = httptest.NewRecorder()
	h.Invalidate(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("invalidate response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := redis.GetYggToken(context.Background(), refreshed); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidate should delete redis token, got %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/authserver/authenticate", strings.NewReader(`{"username":"ygg-flow@test.com","password":"Password123","clientToken":"client-flow"}`))
	rec = httptest.NewRecorder()
	h.Authenticate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticate before signout mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	signoutAccess := decodeYggJSON(t, rec.Body.String())["accessToken"].(string)
	req = httptest.NewRequest(http.MethodPost, "/authserver/signout", strings.NewReader(`{"username":"ygg-flow@test.com","password":"Password123"}`))
	rec = httptest.NewRecorder()
	h.Signout(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("signout response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := redis.GetYggToken(context.Background(), signoutAccess); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("signout should delete user redis tokens, got %v", err)
	}
}

func decodeYggJSON(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode yggdrasil response %q: %v", body, err)
	}
	return out
}
