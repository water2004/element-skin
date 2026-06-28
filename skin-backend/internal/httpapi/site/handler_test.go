package site_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/permission"
	sitepkg "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestHandlerAuthRequestsUserAccess(t *testing.T) {
	h := site.New(testutil.TestConfig(), nil, sitepkg.Site{}, func(next http.HandlerFunc, definitions ...permission.Definition) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			actor := shared.CurrentActor(req)
			if actor.UserID == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			for _, def := range definitions {
				if !actor.Has(def) {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			next(w, req)
		}
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("site Auth should reject unauthenticated requests: status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := shared.WithActor(req.Context(), permission.Actor{
		UserID:      "test-user",
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: permission.NewBitSet(len(permission.Definitions)),
	})
	wrapped(rec, req.WithContext(ctx))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("site Auth should pass authenticated requests: status=%d", rec.Code)
	}
}
