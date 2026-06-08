package remote_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/remote"
)

func TestHandlerAuthRequestsUserAccess(t *testing.T) {
	var requireAdmin bool
	h := remote.New(nil, func(next http.HandlerFunc, require bool) http.HandlerFunc {
		requireAdmin = require
		return next
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent || requireAdmin {
		t.Fatalf("remote Auth should request non-admin access: status=%d requireAdmin=%v", rec.Code, requireAdmin)
	}
}
