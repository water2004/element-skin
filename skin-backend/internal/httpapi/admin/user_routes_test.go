package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

var (
	testProtectedPermission = permission.MustDefinitionByCode("permission_protected.manage.any")
)

func withAdminActor(req *http.Request, userID string) *http.Request {
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, rolePermissions(permission.RoleAdmin)...))
}

func withProtectedActor(req *http.Request, userID string) *http.Request {
	defs := append([]permission.Definition{}, rolePermissions(permission.RoleAdmin)...)
	defs = append(defs, testProtectedPermission)
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, defs...))
}

func rolePermissions(roleID string) []permission.Definition {
	for _, role := range permission.Roles {
		if role.ID == roleID {
			return role.Permissions
		}
	}
	panic("missing role: " + roleID)
}

func TestUserRoutesListAndProtectCurrentUserExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-users@test.com", "Password123", "AdminUsers", true)
	other := testutil.CreateUser(t, db, "listed-users@test.com", "Password123", "ListedUsers", false)

	req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=1&q=Listed", nil)
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+other.ID+`"`) ||
		!strings.Contains(rec.Body.String(), `"email":"listed-users@test.com"`) ||
		!strings.Contains(rec.Body.String(), `"roles":["user"]`) ||
		!strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("admin user list mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+adminUser.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", adminUser.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot delete yourself") {
		t.Fatalf("self delete should be forbidden exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+adminUser.ID+"/roles/super_admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", adminUser.ID)
	req.SetPathValue("role_id", permission.RoleSuperAdmin)
	req = withProtectedActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot grant protected role to yourself") {
		t.Fatalf("self protected role grant should be forbidden exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestRoleGrantAndRevokeControlsExactPermissions(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	protectedAdmin := testutil.CreateUser(t, db, "protected-role@test.com", "Password123", "ProtectedRole", true, true)
	plainAdmin := testutil.CreateUser(t, db, "plain-role@test.com", "Password123", "PlainRole", true)
	target := testutil.CreateUser(t, db, "target-role@test.com", "Password123", "TargetRole", false)

	req := httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withAdminActor(req, plainAdmin.ID)
	rec := httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true,\"role_id\":\"admin\"}\n" {
		t.Fatalf("admin role grant response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if hasRole, err := db.Permissions.UserHasRole(req.Context(), target.ID, permission.RoleAdmin); err != nil || !hasRole {
		t.Fatalf("target admin role after grant = %v, %v; want true, nil", hasRole, err)
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/roles/super_admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleSuperAdmin)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"protected role management required\"}\n" {
		t.Fatalf("plain admin protected role grant mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/roles/super_admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleSuperAdmin)
	req = withProtectedActor(req, protectedAdmin.ID)
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true,\"role_id\":\"super_admin\"}\n" {
		t.Fatalf("protected role grant response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if hasRole, err := db.Permissions.UserHasRole(req.Context(), target.ID, permission.RoleSuperAdmin); err != nil || !hasRole {
		t.Fatalf("target super admin role after grant = %v, %v; want true, nil", hasRole, err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID+"/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.RevokeUserRole(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true,\"role_id\":\"admin\"}\n" {
		t.Fatalf("role revoke response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if hasRole, err := db.Permissions.UserHasRole(req.Context(), target.ID, permission.RoleAdmin); err != nil || hasRole {
		t.Fatalf("target admin role after revoke = %v, %v; want false, nil", hasRole, err)
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
	req := httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/permissions/texture.update_visibility.owned", strings.NewReader(`{"effect":"deny"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "texture.update_visibility.owned")
	rec := httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"effect\":\"deny\",\"ok\":true,\"permission_code\":\"texture.update_visibility.owned\"}\n" {
		t.Fatalf("deny override response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "deny permission override")

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/permissions/notice.create.any", strings.NewReader(`{"effect":"allow"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "notice.create.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"effect\":\"allow\",\"ok\":true,\"permission_code\":\"notice.create.any\"}\n" {
		t.Fatalf("allow override response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users/"+target.ID+"/permissions", nil)
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
	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID+"/permissions/texture.update_visibility.owned", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "texture.update_visibility.owned")
	rec = httptest.NewRecorder()
	h.ClearUserPermissionOverride(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true,\"permission_code\":\"texture.update_visibility.owned\"}\n" {
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
	superAdmin := testutil.CreateUser(t, db, "super-permission-reject@test.com", "Password123", "SuperPermissionReject", true, true)
	target := testutil.CreateUser(t, db, "target-permission-reject@test.com", "Password123", "TargetPermissionReject", false)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/missing-user/permissions", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", "missing-user")
	rec := httptest.NewRecorder()
	h.UserPermissions(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing user permissions mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/permissions/nope.nope.nope", strings.NewReader(`{"effect":"allow"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "nope.nope.nope")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"permission not found\"}\n" {
		t.Fatalf("unknown permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/permissions/notice.create.any", strings.NewReader(`{"effect":"inherit"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "notice.create.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"effect must be allow or deny\"}\n" {
		t.Fatalf("invalid effect mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/permissions/permission_protected.manage.any", strings.NewReader(`{"effect":"allow"}`))
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "permission_protected.manage.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"protected permission management required\"}\n" {
		t.Fatalf("plain admin protected permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/permissions/permission_protected.manage.any", strings.NewReader(`{"effect":"allow"}`))
	req = withProtectedActor(req, superAdmin.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "permission_protected.manage.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"effect\":\"allow\",\"ok\":true,\"permission_code\":\"permission_protected.manage.any\"}\n" {
		t.Fatalf("protected actor grant protected permission mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/"+superAdmin.ID+"/permissions/permission_protected.manage.any", strings.NewReader(`{"effect":"deny"}`))
	req = withProtectedActor(req, superAdmin.ID)
	req.SetPathValue("user_id", superAdmin.ID)
	req.SetPathValue("permission_code", "permission_protected.manage.any")
	rec = httptest.NewRecorder()
	h.SetUserPermissionOverride(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"cannot modify protected permission on yourself\"}\n" {
		t.Fatalf("self protected override mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID+"/permissions/notice.create.any", nil)
	req = withAdminActor(req, adminUser.ID)
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("permission_code", "notice.create.any")
	rec = httptest.NewRecorder()
	h.ClearUserPermissionOverride(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"permission override not found\"}\n" {
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

func TestUserRoutesDetailProfilesBanUnbanAndResetPassword(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-actions@test.com", "Password123", "AdminUserActions", true)
	target := testutil.CreateUser(t, db, "target-user-actions@test.com", "Password123", "TargetUserActions", false)
	profile := testutil.CreateProfile(t, db, target.ID, "target_user_profile", "TargetUserProfile")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+target.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.User(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+target.ID+`"`) || !strings.Contains(rec.Body.String(), `"email":"target-user-actions@test.com"`) {
		t.Fatalf("user detail response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users/"+target.ID+"/profiles", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UserProfiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"TargetUserProfile"`) {
		t.Fatalf("user profiles response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	banUntil := time.Now().Add(time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(banUntil)+`}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"banned_until":`+strconvI64(banUntil)) {
		t.Fatalf("ban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || !banned {
		t.Fatalf("target should be banned: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("unban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("target should be unbanned: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminNewPassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("reset password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), target.ID)
	if err != nil || updated == nil || !util.VerifyPassword("AdminNewPassword123", updated.Password) {
		t.Fatalf("reset password should persist new hash: user=%#v err=%v", updated, err)
	}
}

func TestUserProfilesPaginatesEncodedCursorWithoutRepeatingRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-profile-page@test.com", "Password123", "AdminUserProfilePage", true)
	target := testutil.CreateUser(t, db, "target-user-profile-page@test.com", "Password123", "TargetUserProfilePage", false)
	firstProfile := testutil.CreateProfile(t, db, target.ID, "admin_user_profile_page_a", "ProfilePageA")
	secondProfile := testutil.CreateProfile(t, db, target.ID, "admin_user_profile_page_b", "ProfilePageB")

	requestPage := func(cursor string) *httptest.ResponseRecorder {
		targetURL := "/admin/users/" + target.ID + "/profiles?limit=1"
		if cursor != "" {
			targetURL += "&cursor=" + cursor
		}
		req := httptest.NewRequest(http.MethodGet, targetURL, nil)
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("user_id", target.ID)
		req = withAdminActor(req, adminUser.ID)
		rec := httptest.NewRecorder()
		h.UserProfiles(rec, req)
		return rec
	}
	decodePage := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var page map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}

	firstRec := requestPage("")
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first user profile page status=%d body=%q", firstRec.Code, firstRec.Body.String())
	}
	first := decodePage(firstRec)
	firstItems := first["items"].([]any)
	cursor, _ := first["next_cursor"].(string)
	if len(firstItems) != 1 || firstItems[0].(map[string]any)["id"] != firstProfile.ID || first["has_next"] != true || cursor == "" {
		t.Fatalf("first user profile page mismatch: %#v", first)
	}

	secondRec := requestPage(cursor)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second user profile page status=%d body=%q", secondRec.Code, secondRec.Body.String())
	}
	second := decodePage(secondRec)
	secondItems := second["items"].([]any)
	if len(secondItems) != 1 || secondItems[0].(map[string]any)["id"] != secondProfile.ID ||
		second["has_next"] != false || second["next_cursor"] != "" {
		t.Fatalf("second user profile page mismatch: %#v", second)
	}

	for _, malformed := range []string{
		"not-base64",
		util.EncodeCursor(map[string]any{"unexpected": "value"}),
	} {
		rec := requestPage(malformed)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
			t.Fatalf("malformed user profile cursor status=%d body=%q", rec.Code, rec.Body.String())
		}
	}
}

func TestUserRoutesMutationsInvalidateAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, redis, nil)
	superAdmin := testutil.CreateUser(t, db, "admin-cache-super@test.com", "Password123", "AdminCacheSuper", true, true)
	adminUser := testutil.CreateUser(t, db, "admin-cache-admin@test.com", "Password123", "AdminCacheAdmin", true)
	target := testutil.CreateUser(t, db, "admin-cache-target@test.com", "Password123", "AdminCacheTarget", false)

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
	req := httptest.NewRequest(http.MethodPut, "/admin/users/"+target.ID+"/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withProtectedActor(req, superAdmin.ID)
	rec := httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("grant admin role response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "grant admin role")

	cacheTarget(t)
	banUntil := time.Now().Add(time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(banUntil)+`}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "ban user")

	cacheTarget(t)
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "unban user")

	cacheTarget(t)
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "admin_reset_ygg", UserID: target.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminCachePassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset user password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertTargetCacheMiss(t, "reset user password")
	if _, err := redis.GetYggToken(t.Context(), "admin_reset_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("admin reset password should revoke target ygg tokens, got %v", err)
	}
}

func TestUserRoutesRejectInvalidBanUnbanAndResetPayloadsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-errors@test.com", "Password123", "AdminUserErrors", true)
	target := testutil.CreateUser(t, db, "target-user-errors@test.com", "Password123", "TargetUserErrors", false)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("ban bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":1}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"banned_until is required\"}\n" {
		t.Fatalf("ban expired timestamp mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("invalid ban should not change user state: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/missing-user/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", "missing-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("unban missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"user_id and new_password required\"}\n" {
		t.Fatalf("reset missing password mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"missing-user","new_password":"AdminNewPassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("reset missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUnbanReturnsNotFoundWhenUserIsDeletedAfterAuthorizationCheck(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-unban-delete-race@test.com", "Password123", "AdminUnbanRace", true)
	target := testutil.CreateUser(t, db, "target-unban-delete-race@test.com", "Password123", "TargetUnbanRace", false)

	tx, err := db.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	var one, lockHolderPID int
	if err := tx.QueryRow(t.Context(), `SELECT 1, pg_backend_pid() FROM users WHERE id=$1 FOR UPDATE`, target.ID).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/admin/users/"+target.ID+"/unban", nil)
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("user_id", target.ID)
		req = withAdminActor(req, adminUser.ID)
		rec := httptest.NewRecorder()
		h.UnbanUser(rec, req)
		result <- rec
	}()
	waitForBlockedAdminMutation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(t.Context(), `DELETE FROM users WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	rec := <-result
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("user deleted before unban should return exact not found: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func waitForBlockedAdminMutation(
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	lockHolderPID int,
	result <-chan *httptest.ResponseRecorder,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case rec := <-result:
			t.Fatalf("admin mutation completed before row-lock release: status=%d body=%q", rec.Code, rec.Body.String())
		default:
		}
		var waiting bool
		if err := db.QueryRow(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("admin mutation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUserRoutesDeleteUserAndInvalidateAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, redis, nil)
	adminUser := testutil.CreateUser(t, db, "admin-delete@test.com", "Password123", "AdminDelete", true)
	target := testutil.CreateUser(t, db, "target-delete@test.com", "Password123", "TargetDelete", false)
	profile := testutil.CreateProfile(t, db, target.ID, "delete_user_profile", "DeleteUserProfile")
	if err := redis.SetAuthUser(context.Background(), redisstore.AuthUser{ID: target.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "admin_delete_ygg", UserID: target.ID, CreatedAt: time.Now().UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if user, err := db.Users.GetByID(req.Context(), target.ID); err != nil || user != nil {
		t.Fatalf("delete user should remove user row: user=%#v err=%v", user, err)
	}
	if p, err := db.Profiles.GetByID(req.Context(), profile.ID); err != nil || p != nil {
		t.Fatalf("delete user should cascade profile row: profile=%#v err=%v", p, err)
	}
	if _, err := redis.GetAuthUser(context.Background(), target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete user should invalidate auth cache, got %v", err)
	}
	if _, err := redis.GetYggToken(t.Context(), "admin_delete_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete user should revoke existing ygg tokens, got %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/users/"+target.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"detail":"user not found"`) {
		t.Fatalf("delete missing user mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserRoutesProtectSuperAdminFromPlainAdminExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	plainAdmin := testutil.CreateUser(t, db, "plain-protect@test.com", "Password123", "PlainProtect", true)
	superAdmin := testutil.CreateUser(t, db, "super-protect@test.com", "Password123", "SuperProtect", true, true)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+superAdmin.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", superAdmin.ID)
	req = withAdminActor(req, plainAdmin.ID)
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"cannot modify super admin"`) {
		t.Fatalf("plain admin deleting super admin mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+superAdmin.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(time.Now().Add(time.Hour).UnixMilli())+`}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", superAdmin.ID)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"detail":"cannot modify super admin"`) {
		t.Fatalf("plain admin banning super admin mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+superAdmin.ID+"/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", superAdmin.ID)
	req = withAdminActor(req, plainAdmin.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"cannot modify super admin\"}\n" {
		t.Fatalf("plain admin unbanning super admin mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserRoutesRejectMissingTargetsAndMalformedResetWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	superAdmin := testutil.CreateUser(t, db, "admin-missing-super@test.com", "Password123", "AdminMissingSuper", true, true)
	target := testutil.CreateUser(t, db, "admin-reset-unchanged@test.com", "Password123", "AdminResetUnchanged", false)

	req := httptest.NewRequest(http.MethodGet, "/admin/users?cursor=not-base64", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("user list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	incompleteCursor := util.EncodeCursor(map[string]any{"unexpected": "value"})
	req = httptest.NewRequest(http.MethodGet, "/admin/users?cursor="+incompleteCursor, nil)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("user list incomplete cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/users/missing-user", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", "missing-user")
	rec = httptest.NewRecorder()
	h.User(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing user detail mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/missing-user/roles/admin", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", "missing-user")
	req.SetPathValue("role_id", permission.RoleAdmin)
	req = withProtectedActor(req, superAdmin.ID)
	rec = httptest.NewRecorder()
	h.GrantUserRole(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"user not found\"}\n" {
		t.Fatalf("missing role grant target mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	req = withProtectedActor(req, superAdmin.ID)
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

	req := httptest.NewRequest(http.MethodPost, "/admin/users/reset-password", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminNewPassword123"}`))
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

type deleteYggFailRedis struct {
	redisstore.Store
	deleteCalls int
}

func (r *deleteYggFailRedis) DeleteYggTokensByUser(context.Context, string) error {
	r.deleteCalls++
	return errors.New("ygg token revocation failed")
}

type authInvalidateFailRedis struct {
	redisstore.Store
	failAt  int
	userIDs []string
}

type repopulateDuringTransferRedis struct {
	redisstore.Store
	oldUsers map[string]redisstore.AuthUser
	userIDs  []string
}

func (r *repopulateDuringTransferRedis) InvalidateAuthUser(ctx context.Context, userID string) error {
	r.userIDs = append(r.userIDs, userID)
	if err := r.Store.InvalidateAuthUser(ctx, userID); err != nil {
		return err
	}
	if len(r.userIDs) == 2 {
		for _, oldUser := range r.oldUsers {
			if err := r.Store.SetAuthUser(ctx, oldUser, time.Minute); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *authInvalidateFailRedis) InvalidateAuthUser(ctx context.Context, userID string) error {
	r.userIDs = append(r.userIDs, userID)
	if len(r.userIDs) == r.failAt {
		return errors.New("auth cache invalidation failed")
	}
	return r.Store.InvalidateAuthUser(ctx, userID)
}

func strconvI64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
