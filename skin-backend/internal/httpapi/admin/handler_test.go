package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestHandlerAuthRequestsAdminAccess(t *testing.T) {
	var requireAdmin bool
	h := admin.New(testutil.TestConfig(), nil, func(next http.HandlerFunc, require bool) http.HandlerFunc {
		requireAdmin = require
		return next
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent || !requireAdmin {
		t.Fatalf("admin Auth should request admin access: status=%d requireAdmin=%v", rec.Code, requireAdmin)
	}
}
