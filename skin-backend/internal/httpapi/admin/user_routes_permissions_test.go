package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestAdminUserRoutesRejectMissingFineGrainedPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-permission-matrix@test.com", "Password123", "AdminPermissionMatrix", true)
	target := testutil.CreateUser(t, db, "target-permission-matrix@test.com", "Password123", "TargetPermissionMatrix", false)

	cases := []struct {
		name        string
		permission  string
		makeRequest func() *http.Request
		call        func(http.ResponseWriter, *http.Request)
	}{
		{"users list requires user read", "user.read.any", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v2/admin/users", nil)
		}, h.Users},
		{"user detail requires account read", "account.read.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/"+target.ID, nil)
			req.SetPathValue("user_id", target.ID)
			return req
		}, h.User},
		{"role grant requires permission grant", "permission.grant.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
			req.SetPathValue("user_id", target.ID)
			req.SetPathValue("role_id", permission.RoleAdmin)
			return req
		}, h.GrantUserRole},
		{"role revoke requires permission revoke", "permission.revoke.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
			req.SetPathValue("user_id", target.ID)
			req.SetPathValue("role_id", permission.RoleAdmin)
			return req
		}, h.RevokeUserRole},
		{"delete requires account delete", "account.delete.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID, nil)
			req.SetPathValue("user_id", target.ID)
			return req
		}, h.DeleteUser},
		{"profiles require profile read", "profile.read.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/"+target.ID+"/profiles", nil)
			req.SetPathValue("user_id", target.ID)
			return req
		}, h.UserProfiles},
		{"ban requires account ban", "account.ban.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`,"reason":"permission check"}`))
			req.SetPathValue("user_id", target.ID)
			return req
		}, h.BanUser},
		{"unban requires account unban", "account.unban.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/unban", nil)
			req.SetPathValue("user_id", target.ID)
			return req
		}, h.UnbanUser},
		{"reset password requires account update", "account.update.any", func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"ResetPassword123"}`))
		}, h.ResetUserPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withAdminActorWithoutPermission(tc.makeRequest(), adminUser.ID, tc.permission)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("permission denial mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRoleGrantAndRevokeControlsExactPermissions(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)
	protectedAdmin := testutil.CreateUser(t, db, "protected-role@test.com", "Password123", "ProtectedRole", true, true)
	plainAdmin := testutil.CreateUser(t, db, "plain-role@test.com", "Password123", "PlainRole", true)
	target := testutil.CreateUser(t, db, "target-role@test.com", "Password123", "TargetRole", false)

	req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withAdminActor(req, plainAdmin.ID)
	rec := httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("admin role grant response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if hasRole, err := db.Permissions.UserHasRole(req.Context(), target.ID, permission.RoleAdmin); err != nil || !hasRole {
		t.Fatalf("target admin role after grant = %v, %v; want true, nil", hasRole, err)
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/roles/system_maintenance", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleSystemMaintenance)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"protected_role\",\"operation\":\"manage\",\"reason\":\"required\"}}\n" {
		t.Fatalf("plain admin protected role generic grant mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := cache.SetAuthUser(t.Context(), redisstore.AuthUser{ID: protectedAdmin.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(t.Context(), redisstore.AuthUser{ID: target.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/protected-subject/transfer", nil)
	req = withProtectedActor(req, protectedAdmin.ID)
	req.SetPathValue("user_id", target.ID)
	rec = httptest.NewRecorder()
	h.TransferProtectedSubject(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("protected subject transfer response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if protected, err := db.Permissions.UserIsProtected(req.Context(), target.ID); err != nil || !protected {
		t.Fatalf("target protected flag after transfer = %v, %v; want true, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(req.Context(), protectedAdmin.ID); err != nil || protected {
		t.Fatalf("actor protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	for _, userID := range []string{protectedAdmin.ID, target.ID} {
		if _, err := cache.GetAuthUser(t.Context(), userID); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("transfer should invalidate auth cache for %s exactly, got %v", userID, err)
		}
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.RevokeUserRole(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"protected_subject\",\"operation\":\"update\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("plain admin protected subject role revoke mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
	req = withProtectedActor(req, target.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	rec = httptest.NewRecorder()
	h.RevokeUserRole(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("protected subject role revoke response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if hasRole, err := db.Permissions.UserHasRole(req.Context(), target.ID, permission.RoleAdmin); err != nil || hasRole {
		t.Fatalf("target admin role after revoke = %v, %v; want false, nil", hasRole, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/admin", nil)
	req = withProtectedActor(req, target.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	rec = httptest.NewRecorder()
	h.RevokeUserRole(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"role_assignment\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing role revoke response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/roles/system_maintenance", nil)
	req = withAdminActor(req, plainAdmin.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleSystemMaintenance)
	rec = httptest.NewRecorder()
	h.RevokeUserRole(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"protected_role\",\"operation\":\"manage\",\"reason\":\"required\"}}\n" {
		t.Fatalf("plain admin protected role generic revoke mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserPermissionRoutesExposeCatalogAndOverrideExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, redis, nil)
	adminUser := testutil.CreateUser(t, db, "admin-permission-route@test.com", "Password123", "AdminPermissionRoute", true)
	target := testutil.CreateUser(t, db, "target-permission-route@test.com", "Password123", "TargetPermissionRoute", false)

	cacheTarget := func(t *testing.T) {
		t.Helper()
		if err := redis.SetAuthUser(t.Context(), redisstore.AuthUser{ID: target.ID}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	assertTargetCacheMiss := func(t *testing.T, action string) {
		t.Helper()
		if _, err := redis.GetAuthUser(t.Context(), target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should invalidate target auth cache, got %v", action, err)
		}
	}

	cacheTarget(t)
	req := httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/permissions/texture.update_visibility.owned", strings.NewReader(`{"effect":"deny"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "texture.update_visibility.owned")
	rec := httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("deny override response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "deny permission override")

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/permissions/notice.create.any", strings.NewReader(`{"effect":"allow"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "notice.create.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("allow override response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users/"+target.ID+"/permissions", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	rec = httptest.NewRecorder()
	h.UserPermissions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("user permissions status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body struct {
		Roles                []string `json:"roles"`
		EffectivePermissions []string `json:"effective_permissions"`
		Overrides            []struct {
			PermissionCode string `json:"permission_code"`
			Effect         string `json:"effect"`
			CreatedAt      int64  `json:"created_at"`
		} `json:"overrides"`
		Catalog struct {
			Permissions []struct {
				ID                  int64  `json:"id"`
				Code                string `json:"code"`
				Description         string `json:"description"`
				BitIndex            int    `json:"bit_index"`
				Resource            string `json:"resource"`
				ResourceDescription string `json:"resource_description"`
				Action              string `json:"action"`
				ActionDescription   string `json:"action_description"`
				Scope               string `json:"scope"`
				ScopeDescription    string `json:"scope_description"`
			} `json:"permissions"`
			Roles []struct {
				ID          string   `json:"id"`
				Name        string   `json:"name"`
				Description string   `json:"description"`
				SystemRole  bool     `json:"system_role"`
				Protected   bool     `json:"protected"`
				Permissions []string `json:"permissions"`
			} `json:"roles"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Roles) != 1 || body.Roles[0] != permission.RoleUser {
		t.Fatalf("target roles mismatch: %#v", body.Roles)
	}
	if !containsString(body.EffectivePermissions, "notice.create.any") || containsString(body.EffectivePermissions, "texture.update_visibility.owned") || !containsString(body.EffectivePermissions, "texture.update_metadata.owned") {
		t.Fatalf("effective permissions mismatch after overrides: %#v", body.EffectivePermissions)
	}
	if len(body.Overrides) != 2 ||
		body.Overrides[0].PermissionCode != "notice.create.any" ||
		body.Overrides[0].Effect != "allow" ||
		body.Overrides[0].CreatedAt <= 0 ||
		body.Overrides[1].PermissionCode != "texture.update_visibility.owned" ||
		body.Overrides[1].Effect != "deny" ||
		body.Overrides[1].CreatedAt <= 0 {
		t.Fatalf("overrides response mismatch: %#v", body.Overrides)
	}
	if len(body.Catalog.Permissions) != len(permission.Definitions) || body.Catalog.Permissions[0].Code != "account.read.self" || body.Catalog.Permissions[0].Resource != "account" || body.Catalog.Permissions[0].Action != "read" || body.Catalog.Permissions[0].Scope != "self" {
		t.Fatalf("permission catalog mismatch: first=%#v len=%d", body.Catalog.Permissions[0], len(body.Catalog.Permissions))
	}
	if len(body.Catalog.Roles) != len(permission.Roles) || body.Catalog.Roles[0].ID != permission.RoleUser || body.Catalog.Roles[0].Name != "用户" || !containsString(body.Catalog.Roles[0].Permissions, "account.read.self") {
		t.Fatalf("role catalog mismatch: first=%#v len=%d", body.Catalog.Roles[0], len(body.Catalog.Roles))
	}

	cacheTarget(t)
	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/permissions/texture.update_visibility.owned", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "texture.update_visibility.owned")
	rec = httptest.NewRecorder()
	h.ClearUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("clear override response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "clear permission override")
	bits, err := db.Permissions.EffectivePermissionsForUser(t.Context(), target.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bits.Has(permission.MustDefinitionByCode("texture.update_visibility.owned").BitIndex) {
		t.Fatal("clearing deny override should restore texture.update_visibility.owned")
	}
}

func TestUserPermissionRoutesRejectInvalidAndProtectedOperationsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-permission-reject@test.com", "Password123", "AdminPermissionReject", true)
	protectedAdmin := testutil.CreateUser(t, db, "protected-permission-reject@test.com", "Password123", "ProtectedPermissionReject", true, true)
	target := testutil.CreateUser(t, db, "target-permission-reject@test.com", "Password123", "TargetPermissionReject", false)

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/missing-user/permissions", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", "missing-user")
	rec := httptest.NewRecorder()
	h.UserPermissions(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"user\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing user permissions mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/permissions/nope.nope.nope", strings.NewReader(`{"effect":"allow"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "nope.nope.nope")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("unknown permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/permissions/notice.create.any", strings.NewReader(`{"effect":"inherit"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "notice.create.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"permission_override\",\"operation\":\"configure\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid effect mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/permissions/permission_protected.manage.any", strings.NewReader(`{"effect":"allow"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "permission_protected.manage.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"protected_permission\",\"operation\":\"transfer\",\"reason\":\"required\"}}\n" {
		t.Fatalf("plain admin protected management permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+target.ID+"/permissions/permission_protected.manage.any", strings.NewReader(`{"effect":"allow"}`))
	req = withProtectedActor(req, protectedAdmin.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "permission_protected.manage.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"protected_permission\",\"operation\":\"transfer\",\"reason\":\"required\"}}\n" {
		t.Fatalf("protected actor grant protected management permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/admin/users/"+protectedAdmin.ID+"/permissions/permission_protected.manage.any", strings.NewReader(`{"effect":"deny"}`))
	req = withProtectedActor(req, protectedAdmin.ID)
	req.SetPathValue("user_id", protectedAdmin.ID)
	req.SetPathValue("permission_code", "permission_protected.manage.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"protected_permission\",\"operation\":\"transfer\",\"reason\":\"required\"}}\n" {
		t.Fatalf("self protected management override mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+target.ID+"/permissions/notice.create.any", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "notice.create.any")
	rec = httptest.NewRecorder()
	h.ClearUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"permission_override\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing clear override mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAdminAuthWrapperOnlyRequiresAuthenticatedUser(t *testing.T) {
	var required []permission.Definition
	h := admin.New(testutil.TestConfig(), nil, func(next http.HandlerFunc, defs ...permission.Definition) http.HandlerFunc {
		required = defs
		return next
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.Background()))
	if rec.Code != http.StatusNoContent || len(required) != 0 {
		t.Fatalf("admin Auth required permissions mismatch: status=%d required=%v", rec.Code, required)
	}
}
