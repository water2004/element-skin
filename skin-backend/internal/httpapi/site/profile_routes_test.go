package site_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/testutil"
)

func TestProfileRoutesCreateAndListExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, service.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-profile@test.com", "Password123", "SiteProfile", false)

	req := httptest.NewRequest(http.MethodPost, "/me/profiles", strings.NewReader(`{"name":"RouteRole","model":"slim"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.CreateProfile(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"RouteRole"`) || !strings.Contains(rec.Body.String(), `"model":"slim"`) {
		t.Fatalf("create profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/me/profiles?limit=1", nil)
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ListMyProfiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"RouteRole"`) || !strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("list profiles response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
