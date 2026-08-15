package account_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAccountServicePropagatesClosedDatabaseErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-closed@test.com", "Password123", "AdminAccountClosed", true)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(
		adminUser.ID,
		"user.read.any",
		"account.read.any",
		"permission.grant.any",
		"permission.revoke.any",
		"permission_protected.manage.any",
		"account.delete.any",
		"account.update.any",
		"account.ban.any",
		"account.unban.any",
	)
	db.Close()

	if result, err := svc.ListUsers(ctx, actor, "", 10, ""); result != nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("ListUsers closed db = result=%#v err=%v; want nil closed pool", result, err)
	}
	if result, err := svc.UserDetail(ctx, actor, adminUser.ID); result != nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("UserDetail closed db = result=%#v err=%v; want nil closed pool", result, err)
	}
	if err := svc.GrantUserRole(ctx, actor, adminUser.ID, permission.RoleAdmin); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("GrantUserRole closed db error mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, adminUser.ID, permission.RoleAdmin); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("RevokeUserRole closed db error mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, "target-account-closed"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("TransferProtectedSubject closed db error mismatch: %#v", err)
	}
	if err := svc.DeleteUser(ctx, actor, "target-account-closed"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("DeleteUser closed db error mismatch: %#v", err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: "target-account-closed", NewPassword: "ChangedPassword123"}); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("ResetPassword closed db error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, adminUser.ID, accountsvc.BanUserInput{BannedUntil: database.NowMS() + int64(time.Hour/time.Millisecond), Reason: "closed"}); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("BanUser closed db error mismatch: %#v", err)
	}
	if err := svc.UnbanUser(ctx, actor, adminUser.ID); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("UnbanUser closed db error mismatch: %#v", err)
	}
}

func TestAccountServiceRoleMutationDependencyErrorsAndMissingUsers(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "admin-account-role-errors@test.com", "Password123", "AdminRoleErrors", true)
	target := testutil.CreateUser(t, db, "target-account-role-errors@test.com", "Password123", "TargetRoleErrors", false)
	cache := &accountFailStore{Store: redisstore.NewMemoryStore(), failInvalidate: true}
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(admin.ID, "permission.grant.any", "permission.revoke.any")

	if err := svc.GrantUserRole(ctx, actor, "missing-role-user", permission.RoleAdmin); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("grant missing user mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, "missing-role-user", permission.RoleAdmin); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("revoke missing user mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, target.ID, ""); !httpErrorIs(err, http.StatusBadRequest, "role_id required") {
		t.Fatalf("revoke empty role mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, permission.Actor{}, target.ID, permission.RoleAdmin); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("revoke without permission mismatch: %#v", err)
	}
	err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin)
	if err == nil || err.Error() != "auth cache invalidation failed" {
		t.Fatalf("grant cache failure mismatch: %#v", err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || !hasRole {
		t.Fatalf("grant should persist role before cache failure is returned: has=%v err=%v", hasRole, err)
	}
}

func TestAccountServiceDeleteResetBanUnbanDependencyFailuresKeepExactState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "admin-account-dependency@test.com", "Password123", "AdminDependency", true)
	target := testutil.CreateUser(t, db, "target-account-dependency@test.com", "Password123", "TargetDependency", false)
	actor := actorWithPermissions(admin.ID, "account.delete.any", "account.update.any", "account.ban.any", "account.unban.any")

	yggFail := &accountFailStore{Store: redisstore.NewMemoryStore(), failYggDelete: true}
	svc := accountsvc.AccountService{DB: db, Redis: yggFail}
	if err := svc.DeleteUser(ctx, actor, target.ID); err == nil || err.Error() != "ygg token deletion failed" {
		t.Fatalf("delete ygg failure mismatch: %#v", err)
	}
	if user, err := db.Users.GetByID(ctx, target.ID); err != nil || user == nil {
		t.Fatalf("delete ygg failure must keep target user: user=%#v err=%v", user, err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: target.ID, NewPassword: "ChangedPassword123"}); err == nil || err.Error() != "ygg token deletion failed" {
		t.Fatalf("reset ygg failure mismatch: %#v", err)
	}
	if unchanged, err := db.Users.GetByID(ctx, target.ID); err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("reset ygg failure must preserve password: user=%#v err=%v", unchanged, err)
	}

	provider := model.IdentityProvider{
		ID: "account-delete-failure-provider", Name: "Account Delete Failure", IssuerURL: "https://account-delete-failure.example",
		AuthorizationEndpoint: "https://account-delete-failure.example/authorize", TokenEndpoint: "https://account-delete-failure.example/token",
		JWKSURI: "https://account-delete-failure.example/jwks", ClientID: "account-delete-failure-client",
		Scopes: []string{"openid"}, Adapter: "generic_oidc", Enabled: true, LoginEnabled: true, LinkEnabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	externalIdentity := model.ExternalIdentity{
		ID: "account-delete-failure-identity", UserID: target.ID, ProviderID: provider.ID,
		Subject: "account-delete-failure-subject", CreatedAt: 2, UpdatedAt: 2,
	}
	if err := db.Identities.CreateIdentity(ctx, externalIdentity, model.ExternalIdentityCredential{IdentityID: externalIdentity.ID, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	externalFail := &accountFailStore{Store: redisstore.NewMemoryStore(), failExternalDelete: true}
	svc = accountsvc.AccountService{DB: db, Redis: externalFail}
	if err := svc.DeleteUser(ctx, actor, target.ID); err == nil || err.Error() != "external access token deletion failed" {
		t.Fatalf("delete external token failure mismatch: %#v", err)
	}
	if user, err := db.Users.GetByID(ctx, target.ID); err != nil || user == nil {
		t.Fatalf("delete external token failure must keep target user: user=%#v err=%v", user, err)
	}
	if identity, err := db.Identities.GetIdentity(ctx, externalIdentity.ID); err != nil || identity == nil || identity.UserID != target.ID {
		t.Fatalf("delete external token failure must keep identity: identity=%#v err=%v", identity, err)
	}

	cacheFail := &accountFailStore{Store: redisstore.NewMemoryStore(), failInvalidate: true}
	svc = accountsvc.AccountService{DB: db, Redis: cacheFail}
	bannedUntil := database.NowMS() + int64(time.Hour/time.Millisecond)
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: bannedUntil, Reason: "cache failure"}); err == nil || err.Error() != "auth cache invalidation failed" {
		t.Fatalf("ban cache failure mismatch: %#v", err)
	}
	if updated, err := db.Users.GetByID(ctx, target.ID); err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
		t.Fatalf("ban cache failure should preserve ban timestamp: user=%#v err=%v", updated, err)
	}
	if err := svc.UnbanUser(ctx, actor, target.ID); err == nil || err.Error() != "auth cache invalidation failed" {
		t.Fatalf("unban cache failure mismatch: %#v", err)
	}
	if updated, err := db.Users.GetByID(ctx, target.ID); err != nil || updated == nil || updated.BannedUntil != nil {
		t.Fatalf("unban cache failure should still clear ban timestamp: user=%#v err=%v", updated, err)
	}

	noticeFail := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE notices CASCADE`); err != nil {
		t.Fatal(err)
	}
	nextBan := database.NowMS() + int64(2*time.Hour/time.Millisecond)
	gotUntil, err := noticeFail.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: nextBan, Reason: "notice failure"})
	if gotUntil != 0 {
		t.Fatalf("ban notice failure returned banned_until=%d want 0", gotUntil)
	}
	assertAccountPgCode(t, err, "42P01")
	if updated, err := db.Users.GetByID(ctx, target.ID); err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != nextBan {
		t.Fatalf("ban notice failure should preserve ban timestamp: user=%#v err=%v", updated, err)
	}
}
