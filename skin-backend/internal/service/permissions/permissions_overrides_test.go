package permissions_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	noticesvc "element-skin/backend/internal/service/notice"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/testutil"
)

func TestPermissionServiceSetAndClearOverrideInvalidatesAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-write@test.com", "Password123", "AdminPermsWrite", true)
	target := testutil.CreateUser(t, db, "target-perms-write@test.com", "Password123", "TargetPermsWrite", false)
	cache := redisstore.NewMemoryStore()
	svc := permissionssvc.PermissionService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, 0); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetUserPermissionOverride(ctx, actor, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("auth cache should be invalidated after set, err=%v", err)
	}
	overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 1 || overrides[0].PermissionID != permission.MustDefinitionByCode("notice.create.any").ID ||
		overrides[0].PermissionCode != "notice.create.any" || overrides[0].Effect != "allow" || overrides[0].CreatedAt <= 0 {
		t.Fatalf("stored override mismatch: %#v", overrides)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, actor, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("auth cache should be invalidated after clear, err=%v", err)
	}
	overrides, err = db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 0 {
		t.Fatalf("override should be cleared exactly: %#v", overrides)
	}
}

func TestPermissionServiceSetOverrideSendsExactSystemNoticeOnlyOnChange(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-set-notice@test.com", "Password123", "AdminPermsSetNotice", true)
	target := testutil.CreateUser(t, db, "target-perms-set-notice@test.com", "Password123", "TargetPermsSetNotice", false)
	cache := redisstore.NewMemoryStore()
	svc := permissionssvc.PermissionService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any")
	beforeSet := database.NowMS()

	if err := svc.SetUserPermissionOverride(ctx, actor, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	afterSet := database.NowMS()

	page, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	notices := page["items"].([]model.NoticeView)
	if page["page_size"] != 1 || page["has_next"] != false || len(notices) != 1 {
		t.Fatalf("set override notice page mismatch: page=%#v items=%#v", page, notices)
	}
	notice := notices[0]
	wantContent := "你的单项权限已被调整。\n\n权限：`notice.create.any`\n\n说明：发布通知\n\n结果：允许"
	if notice.Title != "权限已更新：单项权限已调整" ||
		notice.Summary != "你的单项权限已被管理员调整，详情请查看通知。" ||
		notice.ContentMarkdown != wantContent ||
		notice.Type != noticesvc.TypeSystem ||
		notice.DisplayMode != noticesvc.DisplayDetail ||
		notice.Level != noticesvc.LevelInfo ||
		notice.Audience != noticesvc.AudienceTargeted ||
		!notice.Enabled ||
		notice.Pinned ||
		!notice.Dismissible ||
		notice.Read ||
		notice.CreatedBy != nil ||
		notice.LinkText != "" ||
		notice.LinkURL != "" {
		t.Fatalf("set override notice mismatch: %#v", notice)
	}
	const permissionNoticeTTLMS = int64(30 * 24 * 60 * 60 * 1000)
	if notice.EndsAt == nil ||
		*notice.EndsAt < beforeSet+permissionNoticeTTLMS ||
		*notice.EndsAt > afterSet+permissionNoticeTTLMS {
		t.Fatalf("set override notice ends_at mismatch: ends_at=%v before=%d after=%d", notice.EndsAt, beforeSet, afterSet)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetUserPermissionOverride(ctx, actor, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	if cached, err := cache.GetAuthUser(ctx, target.ID); err != nil || cached.ID != target.ID {
		t.Fatalf("duplicate override must not invalidate cache: cached=%#v err=%v", cached, err)
	}
	duplicatePage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	duplicateNotices := duplicatePage["items"].([]model.NoticeView)
	if duplicatePage["page_size"] != 1 || len(duplicateNotices) != 1 || duplicateNotices[0].ID != notice.ID {
		t.Fatalf("duplicate override must not create notices: page=%#v items=%#v", duplicatePage, duplicateNotices)
	}
}

func TestPermissionServiceClearOverrideSendsExactSystemNotice(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-clear-notice@test.com", "Password123", "AdminPermsClearNotice", true)
	target := testutil.CreateUser(t, db, "target-perms-clear-notice@test.com", "Password123", "TargetPermsClearNotice", false)
	other := testutil.CreateUser(t, db, "other-perms-clear-notice@test.com", "Password123", "OtherPermsClearNotice", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")

	if err := svc.SetUserPermissionOverride(ctx, actor, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	beforeClear := database.NowMS()
	if err := svc.ClearUserPermissionOverride(ctx, actor, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
	afterClear := database.NowMS()

	page, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	notices := page["items"].([]model.NoticeView)
	if page["page_size"] != 2 || page["has_next"] != false || len(notices) != 2 {
		t.Fatalf("target permission notice page mismatch: page=%#v items=%#v", page, notices)
	}

	clearNotice := permissionNoticeByTitle(t, notices, "权限已更新：单项权限覆盖已移除")
	wantContent := "你的单项权限覆盖已被移除。\n\n权限：`notice.create.any`\n\n说明：发布通知\n\n原覆盖结果：允许\n\n当前结果将由你的角色和其他权限规则决定。"
	if clearNotice.Summary != "你的单项权限覆盖已被移除，详情请查看通知。" ||
		clearNotice.ContentMarkdown != wantContent ||
		clearNotice.Type != noticesvc.TypeSystem ||
		clearNotice.DisplayMode != noticesvc.DisplayDetail ||
		clearNotice.Level != noticesvc.LevelInfo ||
		clearNotice.Audience != noticesvc.AudienceTargeted ||
		!clearNotice.Enabled ||
		clearNotice.Pinned ||
		!clearNotice.Dismissible ||
		clearNotice.Read ||
		clearNotice.CreatedBy != nil ||
		clearNotice.LinkText != "" ||
		clearNotice.LinkURL != "" {
		t.Fatalf("clear override notice mismatch: %#v", clearNotice)
	}
	const permissionNoticeTTLMS = int64(30 * 24 * 60 * 60 * 1000)
	if clearNotice.EndsAt == nil ||
		*clearNotice.EndsAt < beforeClear+permissionNoticeTTLMS ||
		*clearNotice.EndsAt > afterClear+permissionNoticeTTLMS {
		t.Fatalf("clear override notice ends_at mismatch: ends_at=%v before=%d after=%d", clearNotice.EndsAt, beforeClear, afterClear)
	}

	otherPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(other.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if otherPage["page_size"] != 0 || len(otherPage["items"].([]model.NoticeView)) != 0 {
		t.Fatalf("permission notices must be targeted only: %#v", otherPage)
	}
}

func TestPermissionServiceClearRequiresOppositePermissionForDenyOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-clear-deny@test.com", "Password123", "AdminClearDeny", true)
	target := testutil.CreateUser(t, db, "target-clear-deny@test.com", "Password123", "TargetClearDeny", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	revoker := actorWithPermissions(adminUser.ID, "permission.revoke.any")
	granter := actorWithPermissions(adminUser.ID, "permission.grant.any")

	if err := svc.SetUserPermissionOverride(ctx, revoker, target.ID, "notice.create.any", "deny"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, revoker, target.ID, "notice.create.any"); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("clear deny with revoke permission should fail: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, granter, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionServiceClearRequiresRevokePermissionForAllowOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-clear-allow@test.com", "Password123", "AdminClearAllow", true)
	target := testutil.CreateUser(t, db, "target-clear-allow@test.com", "Password123", "TargetClearAllow", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	granter := actorWithPermissions(adminUser.ID, "permission.grant.any")
	revoker := actorWithPermissions(adminUser.ID, "permission.revoke.any")

	if err := svc.SetUserPermissionOverride(ctx, granter, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, granter, target.ID, "notice.create.any"); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("clear allow with grant permission should fail: %#v", err)
	}
	overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil || len(overrides) != 1 || overrides[0].PermissionCode != "notice.create.any" || overrides[0].Effect != "allow" {
		t.Fatalf("failed clear must preserve allow override: overrides=%#v err=%v", overrides, err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, revoker, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
	overrides, err = db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil || len(overrides) != 0 {
		t.Fatalf("revoker should clear allow override exactly: overrides=%#v err=%v", overrides, err)
	}
}
