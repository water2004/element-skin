package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestWhitelistRoutesAddOfficialWhitelistPersistsUser(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Fallbacks.SaveEndpoints(context.Background(), []fallback.Endpoint{{
		Priority:        1,
		SessionURL:      "https://session.example",
		AccountURL:      "https://account.example",
		ServicesURL:     "https://services.example",
		CacheTTL:        60,
		EnableProfile:   true,
		EnableHasJoined: true,
		EnableWhitelist: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/official-whitelist", strings.NewReader(`{"username":"Steve","endpoint_id":1}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.AddOfficialWhitelist(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("add whitelist response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	ok, err := db.Fallbacks.IsUserInWhitelist(req.Context(), "Steve", 1)
	if err != nil || !ok {
		t.Fatalf("whitelist row should exist exactly: ok=%v err=%v", ok, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/official-whitelist?endpoint_id=1", nil)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.OfficialWhitelist(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"Steve"`) || !strings.Contains(rec.Body.String(), `"created_at":`) {
		t.Fatalf("whitelist list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/official-whitelist/Steve?endpoint_id=1", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("username", "Steve")
	rec = httptest.NewRecorder()
	h.RemoveOfficialWhitelist(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("remove whitelist response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	ok, err = db.Fallbacks.IsUserInWhitelist(req.Context(), "Steve", 1)
	if err != nil || ok {
		t.Fatalf("whitelist row should be removed exactly: ok=%v err=%v", ok, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/official-whitelist?endpoint_id=1", nil)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.OfficialWhitelist(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("empty whitelist list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestWhitelistRoutesRejectInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/official-whitelist", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.OfficialWhitelist(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"endpoint_id is required"`) {
		t.Fatalf("missing endpoint id list mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/official-whitelist", strings.NewReader(`{"username":" ","endpoint_id":1}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.AddOfficialWhitelist(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"username is required"`) {
		t.Fatalf("missing username add mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/official-whitelist/Steve", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("username", "Steve")
	rec = httptest.NewRecorder()
	h.RemoveOfficialWhitelist(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"endpoint_id is required"`) {
		t.Fatalf("missing endpoint id remove mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/official-whitelist", strings.NewReader(`{"username":"Alex","endpoint_id":999}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.AddOfficialWhitelist(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"fallback endpoint not found\"}\n" {
		t.Fatalf("missing endpoint add mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if ok, err := db.Fallbacks.IsUserInWhitelist(req.Context(), "Alex", 999); err != nil || ok {
		t.Fatalf("failed whitelist add must not create row: ok=%v err=%v", ok, err)
	}
}
