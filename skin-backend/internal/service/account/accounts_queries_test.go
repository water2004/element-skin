package account_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAccountServiceListUsersAndUserDetailAttachRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-list@test.com", "Password123", "AdminAccountList", true)
	target := testutil.CreateUser(t, db, "target-account-list@test.com", "Password123", "TargetAccountList", false)
	other := testutil.CreateUser(t, db, "other-account-list@test.com", "Password123", "OtherAccountList", true)
	if err := db.Permissions.GrantRole(ctx, target.ID, permission.RoleAdmin, ""); err != nil {
		t.Fatal(err)
	}
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "user.read.any", "account.read.any")

	page, err := svc.ListUsers(ctx, actor, "", 10, "target-account-list")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if page["page_size"] != 1 || page["has_next"] != false || page["next_cursor"] != "" || len(items) != 1 {
		t.Fatalf("list users page mismatch: %#v", page)
	}
	roles := items[0]["roles"].([]string)
	if items[0]["id"] != target.ID || items[0]["email"] != target.Email || items[0]["display_name"] != target.DisplayName ||
		items[0]["protected"] != false ||
		!stringSliceSetEquals(roles, []string{permission.RoleUser, permission.RoleAdmin}) {
		t.Fatalf("list users item mismatch: %#v", items[0])
	}

	detail, err := svc.UserDetail(ctx, actor, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	detailRoles := detail["roles"].([]string)
	if detail["id"] != other.ID || detail["email"] != other.Email || detail["display_name"] != other.DisplayName ||
		detail["protected"] != false ||
		!stringSliceSetEquals(detailRoles, []string{permission.RoleUser, permission.RoleAdmin}) {
		t.Fatalf("user detail mismatch: %#v", detail)
	}
}

func TestAccountServiceListAndDetailRejectInvalidAccessExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-list-invalid@test.com", "Password123", "AdminAccountListInvalid", true)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "user.read.any", "account.read.any")

	if _, err := svc.ListUsers(ctx, permission.Actor{}, "", 10, ""); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ListUsers without permission mismatch: %#v", err)
	}
	if _, err := svc.ListUsers(ctx, actor, "bad-cursor", 10, ""); !httpErrorIs(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListUsers bad cursor mismatch: %#v", err)
	}
	if _, err := svc.ListUsers(ctx, actor, util.EncodeCursor(map[string]any{"wrong": "field"}), 10, ""); !httpErrorIs(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListUsers cursor missing last_id mismatch: %#v", err)
	}
	if _, err := svc.UserDetail(ctx, permission.Actor{}, adminUser.ID); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("UserDetail without permission mismatch: %#v", err)
	}
	if _, err := svc.UserDetail(ctx, actor, "missing-account-detail"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("UserDetail missing user mismatch: %#v", err)
	}
}
