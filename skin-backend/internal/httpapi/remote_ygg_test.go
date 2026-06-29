package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi"
	sitesvc "element-skin/backend/internal/service/site"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestRemoteYggRoutesValidateAndReturnExactBodies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "remote-ygg@test.com", "Password123", "RemoteYgg", false)
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	rec := authedJSON(t, router, token, "/v1/imports/remote-ygg/profiles/preview", map[string]any{"profiles": []any{map[string]any{"id": "p1", "name": "One"}}})
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"profiles\":[{\"id\":\"p1\",\"name\":\"One\"}]}\n" {
		t.Fatalf("get-profiles response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = authedJSON(t, router, token, "/v1/imports/remote-ygg/profiles/import", map[string]any{"profile_id": "", "profile_name": "Missing"})
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("profile_id and profile_name are required")) {
		t.Fatalf("import-profile should validate required fields: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = authedJSON(t, router, token, "/v1/imports/remote-ygg/profiles/import-batch", map[string]any{"profiles": []any{}})
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("profiles cannot be empty")) {
		t.Fatalf("import-profiles should reject empty list: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = authedJSON(t, router, token, "/v1/imports/remote-ygg/profiles/preview", map[string]any{})
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"profiles\":[]}\n" {
		t.Fatalf("missing profiles should normalize to empty list: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func authedJSON(t *testing.T, h http.Handler, token, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
