package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestSettingsRoutesSaveSiteSettingsPersistsValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings/site", strings.NewReader(`{"site_name":"Route Site"}`))
	rec := httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("save settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	got, err := db.Settings.Get(req.Context(), "site_name", "")
	if err != nil || got != "Route Site" {
		t.Fatalf("site setting should persist exactly: got=%q err=%v", got, err)
	}
}
