package account_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestAccountServiceBanUserPersistsInvalidatesCacheAndSendsExactNotice(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-ban@test.com", "Password123", "AdminAccountBan", true)
	target := testutil.CreateUser(t, db, "target-account-ban@test.com", "Password123", "TargetAccountBan", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "account.ban.any")
	bannedUntil := database.NowMS() + int64(6*time.Hour/time.Millisecond)
	reason := "server join abuse"
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	before := database.NowMS()

	gotUntil, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: bannedUntil, Reason: "  " + reason + "  "})
	if err != nil {
		t.Fatal(err)
	}
	after := database.NowMS()
	if gotUntil != bannedUntil {
		t.Fatalf("banned until mismatch: got=%d want=%d", gotUntil, bannedUntil)
	}
	updated, err := db.Users.GetByID(ctx, target.ID)
	if err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
		t.Fatalf("ban should persist exact timestamp: user=%#v err=%v", updated, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("ban should invalidate auth cache exactly, got %v", err)
	}

	page, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]model.NoticeView)
	if page["page_size"] != 1 || page["has_next"] != false || len(items) != 1 {
		t.Fatalf("target notice page mismatch: page=%#v items=%#v", page, items)
	}
	notice := items[0]
	wantContent := "你的账号已被管理员封禁。\n\n封禁截止时间：" + strconv.FormatInt(bannedUntil, 10) + "\n\n原因：\n\n" + reason
	if notice.Type != noticesvc.TypeSystem || notice.Title != "账号已被封禁" ||
		notice.Summary != "你的账号已被管理员封禁，详情请查看通知。" ||
		notice.ContentMarkdown != wantContent ||
		notice.DisplayMode != noticesvc.DisplayDetail || notice.Level != noticesvc.LevelDanger ||
		notice.Audience != noticesvc.AudienceTargeted || !notice.Enabled || notice.Pinned ||
		!notice.Dismissible || notice.Read || notice.CreatedBy != nil {
		t.Fatalf("ban notice content mismatch: %#v", notice)
	}
	if notice.EndsAt == nil || *notice.EndsAt < before+int64(30*24*time.Hour/time.Millisecond) ||
		*notice.EndsAt > after+int64(30*24*time.Hour/time.Millisecond) {
		t.Fatalf("ban notice end time mismatch: ends_at=%v before=%d after=%d", notice.EndsAt, before, after)
	}

	other := testutil.CreateUser(t, db, "other-account-ban@test.com", "Password123", "OtherAccountBan", false)
	otherPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, actorWithPermissions(other.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if otherPage["page_size"] != 0 || len(otherPage["items"].([]model.NoticeView)) != 0 {
		t.Fatalf("ban notice should be targeted only: %#v", otherPage)
	}
}

func TestAccountServiceBanUserRejectsInvalidInputsWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-ban-invalid@test.com", "Password123", "AdminAccountBanInvalid", true)
	target := testutil.CreateUser(t, db, "target-account-ban-invalid@test.com", "Password123", "TargetAccountBanInvalid", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "account.ban.any")
	future := database.NowMS() + int64(time.Hour/time.Millisecond)

	if _, err := svc.BanUser(ctx, permission.Actor{}, target.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: "reason"}); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ban without permission error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: 1, Reason: "reason"}); !httpErrorIs(err, http.StatusBadRequest, "user_ban.configure.required") {
		t.Fatalf("expired ban timestamp error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: " \t\n "}); !httpErrorIs(err, http.StatusBadRequest, "audit_reason.validate.required") {
		t.Fatalf("missing ban reason error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: strings.Repeat("中", 501)}); !httpErrorIs(err, http.StatusBadRequest, "audit_reason.validate.too_long") {
		t.Fatalf("long ban reason error mismatch: %#v", err)
	}
	if banned, err := db.Users.IsBanned(ctx, target.ID); err != nil || banned {
		t.Fatalf("invalid ban inputs must not change target state: banned=%v err=%v", banned, err)
	}
	page, err := noticesvc.Service{DB: db}.ListForManagement(ctx, actorWithPermissions(adminUser.ID, "notice.read.any"), noticesvc.ListParams{Type: noticesvc.TypeSystem, Status: noticesvc.StatusAll, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page["page_size"] != 0 || len(page["items"].([]model.Notice)) != 0 {
		t.Fatalf("invalid ban inputs must not create notices: %#v", page)
	}
}

func TestAccountServiceProtectsProtectedSubjectAndUnbansExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	plainAdmin := testutil.CreateUser(t, db, "plain-account-protect@test.com", "Password123", "PlainAccountProtect", true)
	protectedAdmin := testutil.CreateUser(t, db, "protected-account-protect@test.com", "Password123", "ProtectedAccountProtect", true, true)
	target := testutil.CreateUser(t, db, "target-account-unban@test.com", "Password123", "TargetAccountUnban", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	plainActor := actorWithPermissions(plainAdmin.ID, "account.ban.any")
	future := database.NowMS() + int64(time.Hour/time.Millisecond)
	if _, err := svc.BanUser(ctx, plainActor, protectedAdmin.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: "protected"}); !httpErrorIs(err, http.StatusForbidden, "protected_subject.update.denied") {
		t.Fatalf("protected admin ban error mismatch: %#v", err)
	}
	if banned, err := db.Users.IsBanned(ctx, protectedAdmin.ID); err != nil || banned {
		t.Fatalf("protected admin ban must not mutate user: banned=%v err=%v", banned, err)
	}

	unbanActor := actorWithPermissions(plainAdmin.ID, "account.unban.any")
	if err := db.Users.Ban(ctx, target.ID, future); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnbanUser(ctx, unbanActor, target.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, target.ID)
	if err != nil || updated == nil || updated.BannedUntil != nil {
		t.Fatalf("unban should clear banned_until exactly: user=%#v err=%v", updated, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("unban should invalidate auth cache exactly, got %v", err)
	}

	if err := svc.UnbanUser(ctx, permission.Actor{}, target.ID); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("unban without permission error mismatch: %#v", err)
	}
	if err := svc.UnbanUser(ctx, unbanActor, "missing-account-unban"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("missing unban user error mismatch: %#v", err)
	}
}

func TestAccountServiceProtectedManagerCanBanProtectedUser(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	manager := testutil.CreateUser(t, db, "manager-account-protected@test.com", "Password123", "ManagerAccountProtected", true, true)
	protected := testutil.CreateUser(t, db, "protected-account-ban@test.com", "Password123", "ProtectedAccountBan", true, true)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(manager.ID, "account.ban.any", "permission_protected.manage.any")
	bannedUntil := database.NowMS() + int64(time.Hour/time.Millisecond)

	if _, err := svc.BanUser(ctx, actor, protected.ID, accountsvc.BanUserInput{BannedUntil: bannedUntil, Reason: "protected managed"}); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, protected.ID)
	if err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
		t.Fatalf("protected manager ban mismatch: user=%#v err=%v", updated, err)
	}
}
