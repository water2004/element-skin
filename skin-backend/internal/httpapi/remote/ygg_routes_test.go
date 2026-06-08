package remote_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/remote"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/testutil"
)

func TestRemoteYggRoutesValidateAndReturnExactBodies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := remote.New(db, nil)
	user := testutil.CreateUser(t, db, "remote-direct@test.com", "Password123", "RemoteDirect", false)

	req := httptest.NewRequest(http.MethodPost, "/remote-ygg/get-profiles", strings.NewReader(`{"profiles":[{"id":"p1","name":"One"}]}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec := httptest.NewRecorder()
	h.GetProfiles(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"profiles\":[{\"id\":\"p1\",\"name\":\"One\"}]}\n" {
		t.Fatalf("get profiles exact body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/remote-ygg/import-profile", strings.NewReader(`{"profile_id":"","profile_name":"Missing"}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "profile_id and profile_name are required") {
		t.Fatalf("import profile validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/remote-ygg/import-profiles", strings.NewReader(`{"profiles":[]}`))
	req = req.WithContext(shared.WithUser(req.Context(), user.ID, false))
	rec = httptest.NewRecorder()
	h.ImportProfiles(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "profiles cannot be empty") {
		t.Fatalf("import profiles empty validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
