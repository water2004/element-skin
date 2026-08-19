package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestListHomepageMedia(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	t.Run("empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v2/admin/homepage-media", nil)
		req = withAdminActor(req, "admin-test-user")
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("empty list status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "{\"items\":[]}\n" {
			t.Fatalf("empty list response mismatch: %q", rec.Body.String())
		}
	})

	t.Run("permission.check.denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v2/admin/homepage-media", nil)
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("permission denied status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
			t.Fatalf("permission denied body mismatch: %q", rec.Body.String())
		}
	})
}
