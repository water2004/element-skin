package admin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestUserRoutesReturnInternalServerErrorWhenDatabaseIsClosedExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
	db.Close()

	cases := []struct {
		name string
		req  *http.Request
		call func(http.ResponseWriter, *http.Request)
	}{
		{"users", withProtectedActor(httptest.NewRequest(http.MethodGet, "/v2/admin/users", nil), "admin-closed-db"), h.Users},
		{"user detail", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/user-closed-db", nil)
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.User},
		{"grant role", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/user-closed-db/roles/admin", nil)
			req.SetPathValue("user_id", "user-closed-db")
			req.SetPathValue("role_id", permission.RoleAdmin)
			return req
		}(), "admin-closed-db"), h.GrantUserRole},
		{"revoke role", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/user-closed-db/roles/admin", nil)
			req.SetPathValue("user_id", "user-closed-db")
			req.SetPathValue("role_id", permission.RoleAdmin)
			return req
		}(), "admin-closed-db"), h.RevokeUserRole},
		{"transfer protected subject", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/user-closed-db/protected-subject/transfer", nil)
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.TransferProtectedSubject},
		{"delete user", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/user-closed-db", nil)
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.DeleteUser},
		{"user profiles", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/user-closed-db/profiles", nil)
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.UserProfiles},
		{"ban user", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/user-closed-db/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`,"reason":"closed database"}`))
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.BanUser},
		{"unban user", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/user-closed-db/unban", nil)
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.UnbanUser},
		{"reset password", withProtectedActor(httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"user-closed-db","new_password":"ChangedPassword123"}`)), "admin-closed-db"), h.ResetUserPassword},
		{"user permissions", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/user-closed-db/permissions", nil)
			req.SetPathValue("user_id", "user-closed-db")
			return req
		}(), "admin-closed-db"), h.UserPermissions},
		{"set permission override", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/user-closed-db/permissions/notice.create.any", strings.NewReader(`{"effect":"allow"}`))
			req.SetPathValue("user_id", "user-closed-db")
			req.SetPathValue("permission_code", "notice.create.any")
			return req
		}(), "admin-closed-db"), h.SetUserPermissionOverride},
		{"clear permission override", withProtectedActor(func() *http.Request {
			req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/user-closed-db/permissions/notice.create.any", nil)
			req.SetPathValue("user_id", "user-closed-db")
			req.SetPathValue("permission_code", "notice.create.any")
			return req
		}(), "admin-closed-db"), h.ClearUserPermissionOverride},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.call(rec, tc.req)
			if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
				t.Fatalf("%s closed database response mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUserRoutesProtectProtectedSubjectFromPlainAdminExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	plainAdmin := testutil.CreateUser(t, db, "plain-protect@test.com", "Password123", "PlainProtect", true)
	protectedAdmin := testutil.CreateUser(t, db, "protected-subject-protect@test.com", "Password123", "ProtectedSubjectProtect", true, true)

	req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+protectedAdmin.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", protectedAdmin.ID)
	req = withAdminActor(req, plainAdmin.ID)
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"cannot modify protected subject"`) {
		t.Fatalf("plain admin deleting protected subject mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+protectedAdmin.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`,"reason":"protected target"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", protectedAdmin.ID)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"cannot modify protected subject"`) {
		t.Fatalf("plain admin banning protected subject mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+protectedAdmin.ID+"/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", protectedAdmin.ID)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"cannot modify protected subject\"}\n" {
		t.Fatalf("plain admin unbanning protected subject mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserRoutesRejectMissingTargetsAndMalformedResetWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	protectedAdmin := testutil.CreateUser(t, db, "admin-missing-protected@test.com", "Password123", "AdminMissingProtected", true, true)
	target := testutil.CreateUser(t, db, "admin-reset-unchanged@test.com", "Password123", "AdminResetUnchanged", false)

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/users?cursor=not-base64", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("user list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	incompleteCursor := util.EncodeCursor(map[string]any{"unexpected": "value"})
	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users?cursor="+incompleteCursor, nil)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("user list incomplete cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users/missing-user", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", "missing-user")
	rec = httptest.NewRecorder()
	h.User(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing user detail mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/missing-user/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", "missing-user")
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withProtectedActor(req, protectedAdmin.ID)
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing role grant target mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/missing-user/protected-subject/transfer", nil)
	req = withProtectedActor(req, protectedAdmin.ID)
	req.SetPathValue("user_id", "missing-user")
	rec = httptest.NewRecorder()
	h.TransferProtectedSubject(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing transfer target mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/", nil)
	req = withProtectedActor(req, protectedAdmin.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", "")
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"role_id required\"}\n" {
		t.Fatalf("blank role grant mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/", nil)
	req = withProtectedActor(req, protectedAdmin.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", "")
	rec = httptest.NewRecorder()
	h.RevokeUserRole(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"role_id required\"}\n" {
		t.Fatalf("blank role revoke mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req = withProtectedActor(req, protectedAdmin.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("malformed reset payload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	unchanged, err := db.Users.GetByID(t.Context(), target.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("rejected reset must preserve password: user=%#v err=%v", unchanged, err)
	}
}

func TestAdminResetPasswordPreservesCredentialsAndRefreshWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	baseCache := testutil.NewMemoryRedis()
	cache := &deleteYggFailRedis{Store: baseCache}
	h := admin.NewWithRedis(cfg, db, cache, nil)
	adminUser := testutil.CreateUser(t, db, "admin-reset-ygg-fail@test.com", "Password123", "AdminResetYggFail", true)
	target := testutil.CreateUser(t, db, "target-reset-ygg-fail@test.com", "Password123", "TargetResetYggFail", false)
	const refreshHash = "admin_reset_ygg_fail_refresh"
	if err := db.Tokens.AddRefresh(t.Context(), refreshHash, target.ID, time.Now().Add(time.Hour).UnixMilli(), time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminNewPassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("admin reset ygg failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	unchanged, err := db.Users.GetByID(t.Context(), target.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) || util.VerifyPassword("AdminNewPassword123", unchanged.Password) {
		t.Fatalf("failed admin reset must preserve old password: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(t.Context(), refreshHash); err != nil || refresh == nil || refresh["user_id"] != target.ID {
		t.Fatalf("failed admin reset must preserve refresh token: refresh=%#v err=%v", refresh, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("admin reset should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}

func TestUserRoutesReturnExactErrorsAfterPrimaryLookupSucceeds(t *testing.T) {
	t.Run("role attachment failure on list and detail", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		h := admin.New(testutil.TestConfig(), db, nil)
		adminUser := testutil.CreateUser(t, db, "admin-role-attach-fail@test.com", "Password123", "AdminRoleAttachFail", true)
		target := testutil.CreateUser(t, db, "target-role-attach-fail@test.com", "Password123", "TargetRoleAttachFail", false)
		if _, err := db.Pool.Exec(t.Context(), `DROP TABLE permission_subjects CASCADE`); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/v2/admin/users?q=TargetRoleAttachFail", nil)
		req = withAdminActor(req, adminUser.ID)
		rec := httptest.NewRecorder()
		h.Users(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("list role attachment failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/v2/admin/users/"+target.ID, nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		rec = httptest.NewRecorder()
		h.User(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("detail role lookup failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("grant and revoke persistence failure after target exists", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		h := admin.New(testutil.TestConfig(), db, nil)
		adminUser := testutil.CreateUser(t, db, "admin-role-write-fail@test.com", "Password123", "AdminRoleWriteFail", true)
		target := testutil.CreateUser(t, db, "target-role-write-fail@test.com", "Password123", "TargetRoleWriteFail", false)
		if _, err := db.Pool.Exec(t.Context(), `DROP TABLE subject_roles CASCADE`); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		req.SetPathValue("role_id", permission.RoleAdmin)
		rec := httptest.NewRecorder()
		h.GrantUserRole(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("grant role persistence failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		req.SetPathValue("role_id", permission.RoleAdmin)
		rec = httptest.NewRecorder()
		h.RevokeUserRole(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("revoke role persistence failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("revoke missing target", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		h := admin.New(testutil.TestConfig(), db, nil)
		adminUser := testutil.CreateUser(t, db, "admin-revoke-missing@test.com", "Password123", "AdminRevokeMissing", true)

		req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/missing-role-target/roles/admin", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", "missing-role-target")
		req.SetPathValue("role_id", permission.RoleAdmin)
		rec := httptest.NewRecorder()
		h.RevokeUserRole(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
			t.Fatalf("revoke missing target mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("protected subject check dependency failure", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		h := admin.New(testutil.TestConfig(), db, nil)
		adminUser := testutil.CreateUser(t, db, "admin-protected-check-fail@test.com", "Password123", "AdminProtectedCheckFail", true)
		target := testutil.CreateUser(t, db, "target-protected-check-fail@test.com", "Password123", "TargetProtectedCheckFail", false)
		if _, err := db.Pool.Exec(t.Context(), `DROP TABLE permission_subjects CASCADE`); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/unban", nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		rec := httptest.NewRecorder()
		h.UnbanUser(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("unban protected-subject check failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID, nil)
		req = withAdminActor(req, adminUser.ID)
		req.SetPathValue("user_id", target.ID)
		rec = httptest.NewRecorder()
		h.DeleteUser(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("delete protected-subject check failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"ChangedPassword123"}`))
		req = withAdminActor(req, adminUser.ID)
		rec = httptest.NewRecorder()
		h.ResetUserPassword(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("reset protected-subject check failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})
}

type deleteYggFailRedis struct {
	redisstore.Store
	deleteCalls int
}

func (r *deleteYggFailRedis) DeleteYggTokensByUser(context.Context, string) error {
	r.deleteCalls++
	return errors.New("ygg token revocation failed")
}
