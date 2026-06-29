package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestHandlerAuthOnlyRequiresAuthenticatedUser(t *testing.T) {
	var required []permission.Definition
	h := admin.New(testutil.TestConfig(), nil, func(next http.HandlerFunc, defs ...permission.Definition) http.HandlerFunc {
		required = defs
		return next
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent || len(required) != 0 {
		t.Fatalf("admin Auth required permissions mismatch: status=%d required=%v", rec.Code, required)
	}
}
