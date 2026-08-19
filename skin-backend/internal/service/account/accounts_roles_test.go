package account_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestAccountServiceGrantAndRevokeRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-role@test.com", "Password123", "AdminAccountRole", true)
	target := testutil.CreateUser(t, db, "target-account-role@test.com", "Password123", "TargetAccountRole", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || !hasRole {
		t.Fatalf("grant role should persist exact role: has=%v err=%v", hasRole, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("grant role should invalidate auth cache exactly, got %v", err)
	}
	grantPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	grantNotices := grantPage["items"].([]model.NoticeView)
	if grantPage["page_size"] != 1 || grantPage["has_next"] != false || len(grantNotices) != 1 {
		t.Fatalf("grant role notice page mismatch: page=%#v items=%#v", grantPage, grantNotices)
	}
	grantNotice := grantNotices[0]
	if grantNotice.Title != "权限已更新：角色已授予" ||
		grantNotice.Summary != "你的站点角色已更新，详情请查看通知。" ||
		grantNotice.ContentMarkdown != "你的站点角色已被授予：管理员（admin）。" ||
		grantNotice.Type != noticesvc.TypeSystem ||
		grantNotice.DisplayMode != noticesvc.DisplayDetail ||
		grantNotice.Level != noticesvc.LevelInfo ||
		grantNotice.Audience != noticesvc.AudienceTargeted ||
		grantNotice.CreatedBy != nil {
		t.Fatalf("grant role notice mismatch: %#v", grantNotice)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if cached, err := cache.GetAuthUser(ctx, target.ID); err != nil || cached.ID != target.ID {
		t.Fatalf("duplicate role grant must not invalidate cache: cached=%#v err=%v", cached, err)
	}
	duplicateGrantPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	duplicateGrantNotices := duplicateGrantPage["items"].([]model.NoticeView)
	if duplicateGrantPage["page_size"] != 1 || len(duplicateGrantNotices) != 1 || duplicateGrantNotices[0].ID != grantNotice.ID {
		t.Fatalf("duplicate role grant must not create notices: page=%#v items=%#v", duplicateGrantPage, duplicateGrantNotices)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || hasRole {
		t.Fatalf("revoke role should remove exact role: has=%v err=%v", hasRole, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("revoke role should invalidate auth cache exactly, got %v", err)
	}
	revokePage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	revokeNotices := revokePage["items"].([]model.NoticeView)
	if revokePage["page_size"] != 2 || revokePage["has_next"] != false || len(revokeNotices) != 2 {
		t.Fatalf("revoke role notice page mismatch: page=%#v items=%#v", revokePage, revokeNotices)
	}
	revokeNotice := accountNoticeByTitle(t, revokeNotices, "权限已更新：角色已撤销")
	if revokeNotice.Summary != "你的站点角色已更新，详情请查看通知。" ||
		revokeNotice.ContentMarkdown != "你的站点角色已被撤销：管理员（admin）。" ||
		revokeNotice.Type != noticesvc.TypeSystem ||
		revokeNotice.DisplayMode != noticesvc.DisplayDetail ||
		revokeNotice.Level != noticesvc.LevelInfo ||
		revokeNotice.Audience != noticesvc.AudienceTargeted ||
		revokeNotice.CreatedBy != nil {
		t.Fatalf("revoke role notice mismatch: %#v", revokeNotice)
	}

	if err := svc.GrantUserRole(ctx, permission.Actor{}, target.ID, permission.RoleAdmin); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("grant without permission mismatch: %#v", err)
	}
	if err := svc.GrantUserRole(ctx, actor, target.ID, ""); !httpErrorIs(err, http.StatusBadRequest, "role_id.validate.required") {
		t.Fatalf("grant empty role mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, target.ID, permission.RoleAdmin); !httpErrorIs(err, http.StatusNotFound, "role_assignment.resolve.not_found") {
		t.Fatalf("revoke missing role mismatch: %#v", err)
	}
}

func TestAccountServiceRevokeRoleReconcilesOAuthDependentsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-role-oauth@test.com", "Password123", "AdminRoleOAuth", true)
	target := testutil.CreateUser(t, db, "target-account-role-oauth@test.com", "Password123", "TargetRoleOAuth", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	client := createAccountOAuthClient(t, db, target.ID, "account-role-reconcile-client", "invite.create.any")
	grant := model.OAuthGrant{
		ID:        "account-role-reconcile-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 2100,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, accountOAuthPermissionIDs("invite.create.any")); err != nil {
		t.Fatal(err)
	}
	unaffectedClient := createAccountOAuthClient(t, db, target.ID, "account-role-unaffected-client", "account.read.self")
	unaffectedGrant := model.OAuthGrant{
		ID:        "account-role-unaffected-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  unaffectedClient.ID,
		Status:    "active",
		CreatedAt: 2200,
	}
	if err := db.OAuth.CreateGrant(ctx, unaffectedGrant, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	before := database.NowMS()

	if err := svc.RevokeUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || hasRole {
		t.Fatalf("role revoke should persist before OAuth assertions: has=%v err=%v", hasRole, err)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, target.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 {
		t.Fatalf("grant count after role revoke = %d want 2: %#v", len(grants), grants)
	}
	revokedGrant := accountOAuthGrantByID(grants, grant.ID)
	if revokedGrant == nil || revokedGrant.Status != "revoked" ||
		revokedGrant.RevokedAt == nil || *revokedGrant.RevokedAt < before {
		t.Fatalf("oauth grant should be revoked exactly after role revoke: %#v", grants)
	}
	keptGrant := accountOAuthGrantByID(grants, unaffectedGrant.ID)
	if keptGrant == nil || keptGrant.Status != "active" || keptGrant.RevokedAt != nil {
		t.Fatalf("unaffected oauth grant should remain active exactly: %#v", grants)
	}
	gotClient, err := db.OAuth.GetClient(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotClient == nil || gotClient.Status != "disabled" || gotClient.UpdatedAt < before {
		t.Fatalf("oauth client should be disabled exactly after owner role revoke: %#v", gotClient)
	}
	gotUnaffectedClient, err := db.OAuth.GetClient(ctx, unaffectedClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotUnaffectedClient == nil || gotUnaffectedClient.Status != "active" || gotUnaffectedClient.UpdatedAt != unaffectedClient.UpdatedAt {
		t.Fatalf("unaffected oauth client should remain active exactly: %#v", gotUnaffectedClient)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("role revoke should invalidate auth cache exactly, got %v", err)
	}

	page, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	notices := page["items"].([]model.NoticeView)
	if page["page_size"] != 4 || page["has_next"] != false || len(notices) != 4 {
		t.Fatalf("role revoke notice page mismatch: page=%#v items=%#v", page, notices)
	}
	grantNotice := accountNoticeByTitle(t, notices, "权限已更新：角色已授予")
	if grantNotice.ContentMarkdown != "你的站点角色已被授予：管理员（admin）。" ||
		grantNotice.Summary != "你的站点角色已更新，详情请查看通知。" ||
		grantNotice.Type != noticesvc.TypeSystem || grantNotice.Audience != noticesvc.AudienceTargeted ||
		grantNotice.Level != noticesvc.LevelInfo || grantNotice.CreatedBy != nil {
		t.Fatalf("role grant notice mismatch: %#v", grantNotice)
	}
	revokeNotice := accountNoticeByTitle(t, notices, "权限已更新：角色已撤销")
	if revokeNotice.ContentMarkdown != "你的站点角色已被撤销：管理员（admin）。" ||
		revokeNotice.Summary != "你的站点角色已更新，详情请查看通知。" ||
		revokeNotice.Type != noticesvc.TypeSystem || revokeNotice.Audience != noticesvc.AudienceTargeted ||
		revokeNotice.Level != noticesvc.LevelInfo || revokeNotice.CreatedBy != nil {
		t.Fatalf("role revoke notice mismatch: %#v", revokeNotice)
	}
	grantDependencyNotice := accountNoticeByTitle(t, notices, "第三方应用授权已自动撤销")
	wantGrantDependencyContent := "你的站点权限发生变化，以下第三方应用授权已自动撤销：\n\n" +
		"- account-role-reconcile-client（`account-role-reconcile-client`）\n\n" +
		"这些授权包含你当前已不再拥有的权限，后续访问会失败。需要继续使用时，请在权限恢复后重新授权。"
	if grantDependencyNotice.Summary != "你的权限发生变化，1 个第三方应用授权已自动撤销。" ||
		grantDependencyNotice.ContentMarkdown != wantGrantDependencyContent ||
		grantDependencyNotice.Level != noticesvc.LevelWarning ||
		grantDependencyNotice.LinkText != "查看授权" || grantDependencyNotice.LinkURL != "/dashboard/oauth" ||
		strings.Contains(grantDependencyNotice.ContentMarkdown, unaffectedClient.ID) ||
		grantDependencyNotice.CreatedBy != nil {
		t.Fatalf("oauth grant dependency notice mismatch: %#v", grantDependencyNotice)
	}
	clientDependencyNotice := accountNoticeByTitle(t, notices, "第三方应用已自动停用")
	wantClientDependencyContent := "你的站点权限发生变化，以下第三方应用已自动停用：\n\n" +
		"- account-role-reconcile-client（`account-role-reconcile-client`）\n\n" +
		"这些应用申请了你当前已不再拥有的权限。请调整应用权限后重新提交审核。"
	if clientDependencyNotice.Summary != "你的权限发生变化，1 个你创建的第三方应用已自动停用。" ||
		clientDependencyNotice.ContentMarkdown != wantClientDependencyContent ||
		clientDependencyNotice.Level != noticesvc.LevelWarning ||
		clientDependencyNotice.LinkText != "查看应用" || clientDependencyNotice.LinkURL != "/dashboard/oauth" ||
		strings.Contains(clientDependencyNotice.ContentMarkdown, unaffectedClient.ID) ||
		clientDependencyNotice.CreatedBy != nil {
		t.Fatalf("oauth client dependency notice mismatch: %#v", clientDependencyNotice)
	}
}

func TestAccountServiceProtectsProtectedRoleMutationsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-protected-role@test.com", "Password123", "AdminAccountProtectedRole", true)
	target := testutil.CreateUser(t, db, "target-account-protected-role@test.com", "Password123", "TargetAccountProtectedRole", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	plainActor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	managerActor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any", "permission_protected.manage.any")

	if err := svc.GrantUserRole(ctx, plainActor, target.ID, permission.RoleSystemMaintenance); !httpErrorIs(err, http.StatusForbidden, "protected_role.manage.required") {
		t.Fatalf("protected role generic grant mismatch: %#v", err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSystemMaintenance); err != nil || hasRole {
		t.Fatalf("rejected protected role generic grant must not mutate target: has=%v err=%v", hasRole, err)
	}
	if err := svc.GrantUserRole(ctx, managerActor, target.ID, permission.RoleSystemMaintenance); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSystemMaintenance); err != nil || !hasRole {
		t.Fatalf("manager protected role grant mismatch: has=%v err=%v", hasRole, err)
	}
	if err := svc.RevokeUserRole(ctx, plainActor, target.ID, permission.RoleSystemMaintenance); !httpErrorIs(err, http.StatusForbidden, "protected_role.manage.required") {
		t.Fatalf("protected role generic revoke mismatch: %#v", err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSystemMaintenance); err != nil || !hasRole {
		t.Fatalf("rejected protected role generic revoke must preserve target: has=%v err=%v", hasRole, err)
	}
}

func TestAccountServiceTransfersProtectedSubjectExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	actorUser := testutil.CreateUser(t, db, "actor-account-transfer@test.com", "Password123", "ActorAccountTransfer", true, true)
	target := testutil.CreateUser(t, db, "target-account-transfer@test.com", "Password123", "TargetAccountTransfer", false)
	stale := testutil.CreateUser(t, db, "stale-account-transfer@test.com", "Password123", "StaleAccountTransfer", true, true)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(actorUser.ID, "permission_protected.manage.any")
	dependentClient := createAccountOAuthClient(t, db, actorUser.ID, "account-transfer-dependent-client", "permission_protected.manage.any")
	dependentGrant := model.OAuthGrant{
		ID:        "account-transfer-dependent-grant",
		UserID:    actorUser.ID,
		SubjectID: permissiondb.SubjectIDForUser(actorUser.ID),
		ClientID:  dependentClient.ID,
		Status:    "active",
		CreatedAt: 4100,
	}
	if err := db.OAuth.CreateGrant(ctx, dependentGrant, accountOAuthPermissionIDs("permission_protected.manage.any")); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{actorUser.ID, target.ID, stale.ID} {
		if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: userID}, time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.TransferProtectedSubject(ctx, permission.Actor{}, target.ID); !httpErrorIs(err, http.StatusForbidden, "protected_subject.manage.required") {
		t.Fatalf("transfer without protected permission mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, actorUser.ID); !httpErrorIs(err, http.StatusForbidden, "protected_subject.transfer.denied") {
		t.Fatalf("self transfer mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, "missing-account-transfer"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("missing transfer target mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, target.ID); err != nil {
		t.Fatal(err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, actorUser.ID); err != nil || protected {
		t.Fatalf("actor protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, target.ID); err != nil || !protected {
		t.Fatalf("target protected flag after transfer = %v, %v; want true, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, stale.ID); err != nil || protected {
		t.Fatalf("stale protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, actorUser.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].ID != dependentGrant.ID || grants[0].Status != "revoked" || grants[0].RevokedAt == nil {
		t.Fatalf("protected transfer should revoke dependent grants exactly: %#v", grants)
	}
	gotDependentClient, err := db.OAuth.GetClient(ctx, dependentClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotDependentClient == nil || gotDependentClient.Status != "disabled" {
		t.Fatalf("protected transfer should disable dependent client exactly: %#v", gotDependentClient)
	}
	for _, userID := range []string{actorUser.ID, target.ID, stale.ID} {
		if _, err := cache.GetAuthUser(ctx, userID); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("transfer should invalidate auth cache for %s exactly, got %v", userID, err)
		}
	}
	actorPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(actorUser.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	actorNotices := actorPage["items"].([]model.NoticeView)
	if actorPage["page_size"] != 3 || actorPage["has_next"] != false || len(actorNotices) != 3 {
		t.Fatalf("actor transfer notice page mismatch: page=%#v items=%#v", actorPage, actorNotices)
	}
	lostNotice := accountNoticeByTitle(t, actorNotices, "权限已更新：受保护主体已转出")
	wantLostContent := "你的受保护权限主体状态已变更。\n\n权限：`permission_protected.manage.any`\n\n说明：管理受保护权限主体\n\n结果：已转移"
	if lostNotice.Summary != "你不再是受保护权限主体，详情请查看通知。" ||
		lostNotice.ContentMarkdown != wantLostContent ||
		lostNotice.Type != noticesvc.TypeSystem || lostNotice.Audience != noticesvc.AudienceTargeted ||
		lostNotice.Level != noticesvc.LevelInfo || lostNotice.CreatedBy != nil {
		t.Fatalf("protected subject lost notice mismatch: %#v", lostNotice)
	}
	grantDependencyNotice := accountNoticeByTitle(t, actorNotices, "第三方应用授权已自动撤销")
	wantGrantDependencyContent := "你的站点权限发生变化，以下第三方应用授权已自动撤销：\n\n" +
		"- account-transfer-dependent-client（`account-transfer-dependent-client`）\n\n" +
		"这些授权包含你当前已不再拥有的权限，后续访问会失败。需要继续使用时，请在权限恢复后重新授权。"
	if grantDependencyNotice.ContentMarkdown != wantGrantDependencyContent ||
		grantDependencyNotice.Summary != "你的权限发生变化，1 个第三方应用授权已自动撤销。" ||
		grantDependencyNotice.LinkText != "查看授权" || grantDependencyNotice.LinkURL != "/dashboard/oauth" ||
		grantDependencyNotice.CreatedBy != nil {
		t.Fatalf("protected transfer grant dependency notice mismatch: %#v", grantDependencyNotice)
	}
	clientDependencyNotice := accountNoticeByTitle(t, actorNotices, "第三方应用已自动停用")
	wantClientDependencyContent := "你的站点权限发生变化，以下第三方应用已自动停用：\n\n" +
		"- account-transfer-dependent-client（`account-transfer-dependent-client`）\n\n" +
		"这些应用申请了你当前已不再拥有的权限。请调整应用权限后重新提交审核。"
	if clientDependencyNotice.ContentMarkdown != wantClientDependencyContent ||
		clientDependencyNotice.Summary != "你的权限发生变化，1 个你创建的第三方应用已自动停用。" ||
		clientDependencyNotice.LinkText != "查看应用" || clientDependencyNotice.LinkURL != "/dashboard/oauth" ||
		clientDependencyNotice.CreatedBy != nil {
		t.Fatalf("protected transfer client dependency notice mismatch: %#v", clientDependencyNotice)
	}
	targetPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	targetNotices := targetPage["items"].([]model.NoticeView)
	if targetPage["page_size"] != 1 || targetPage["has_next"] != false || len(targetNotices) != 1 {
		t.Fatalf("target transfer notice page mismatch: page=%#v items=%#v", targetPage, targetNotices)
	}
	gainedNotice := targetNotices[0]
	wantGainedContent := "你的受保护权限主体状态已变更。\n\n权限：`permission_protected.manage.any`\n\n说明：管理受保护权限主体\n\n结果：允许"
	if gainedNotice.Title != "权限已更新：受保护主体已转入" ||
		gainedNotice.Summary != "你已成为受保护权限主体，详情请查看通知。" ||
		gainedNotice.ContentMarkdown != wantGainedContent ||
		gainedNotice.Type != noticesvc.TypeSystem || gainedNotice.Audience != noticesvc.AudienceTargeted ||
		gainedNotice.Level != noticesvc.LevelInfo || gainedNotice.CreatedBy != nil {
		t.Fatalf("protected subject gained notice mismatch: %#v", gainedNotice)
	}
	stalePage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(stale.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	staleNotices := stalePage["items"].([]model.NoticeView)
	if stalePage["page_size"] != 1 || stalePage["has_next"] != false || len(staleNotices) != 1 {
		t.Fatalf("stale transfer notice page mismatch: page=%#v items=%#v", stalePage, staleNotices)
	}
	if staleNotices[0].Title != "权限已更新：受保护主体已转出" || staleNotices[0].ContentMarkdown != wantLostContent {
		t.Fatalf("stale protected subject lost notice mismatch: %#v", staleNotices[0])
	}
}
