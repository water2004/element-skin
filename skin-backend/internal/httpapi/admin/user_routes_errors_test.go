package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestAdminUserRoutesRejectExactMissingUserAndBadPayloadEdges(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-edge@test.com", "Password123", "AdminUserEdge", true)

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/missing/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`,"reason":"missing user edge"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", "missing-user")
	rec := httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("ban missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"missing-user","new_password":"ChangedPassword123"}`))
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("reset missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+adminUser.ID+"/permissions/account.read.self", strings.NewReader(`{`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", adminUser.ID)
	req.SetPathValue("permission_code", "account.read.self")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("set permission bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
