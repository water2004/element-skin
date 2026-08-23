package oauth_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServiceOAuthReviewFlowCreatesExactNotifications(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-notice-owner@test.com", "Password123", "OAuthNoticeOwner", false)
	admin := testutil.CreateUser(t, db, "oauth-notice-admin@test.com", "Password123", "OAuthNoticeAdmin", true, true)
	other := testutil.CreateUser(t, db, "oauth-notice-other@test.com", "Password123", "OAuthNoticeOther", false)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)

	beforeCreate := database.NowMS()
	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Notify app",
		RedirectURI:     "https://notify.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	assertNoticeRow(t, db, "第三方应用待审核：Notify app", noticeExpectation{
		Summary:  "开发者提交了第三方应用 Notify app，请前往管理面板审核。",
		Level:    noticesvc.LevelWarning,
		Audience: noticesvc.AudienceAdmins,
		LinkURL:  "/admin/oauth-apps",
		Target:   "",
		MinEnds:  beforeCreate + int64((30*24*60*60*1000)-1000),
	})

	ownerSystemPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, oauthNoticeReader(owner.ID), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if items := ownerSystemPage["items"].([]model.NoticeView); len(items) != 0 {
		t.Fatalf("owner must not see admin review request notice: %#v", items)
	}

	if _, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusRejected, ""); !isHTTPError(err, 400, "audit_reason.validate.required") {
		t.Fatalf("reject without reason mismatch: %#v", err)
	}
	stillPending, err := svc.GetClient(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if stillPending["status"] != oauth.StatusPending {
		t.Fatalf("failed review should keep status pending: %#v", stillPending)
	}

	if _, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusRejected, "Missing support contact"); err != nil {
		t.Fatal(err)
	}
	assertNoticeRow(t, db, "第三方应用审核驳回：Notify app", noticeExpectation{
		Summary:  "你的第三方应用 Notify app 未通过审核。",
		Content:  "你的第三方应用 `Notify app` 未通过审核。\n\n原因：\n\nMissing support contact",
		Level:    noticesvc.LevelDanger,
		Audience: noticesvc.AudienceTargeted,
		LinkURL:  "/dashboard/oauth",
		Target:   owner.ID,
		MinEnds:  beforeCreate + int64((30*24*60*60*1000)-1000),
	})

	otherPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, oauthNoticeReader(other.ID), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if items := otherPage["items"].([]model.NoticeView); len(items) != 0 {
		t.Fatalf("other user must not see owner targeted notices: %#v", items)
	}

	activeClient, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Active notify",
		RedirectURI:     "https://active-notify.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	activeID := activeClient["client_id"].(string)
	if _, err := svc.ReviewClient(ctx, adminActor, activeID, oauth.StatusActive, ""); err != nil {
		t.Fatal(err)
	}
	assertNoticeRow(t, db, "第三方应用审核通过：Active notify", noticeExpectation{
		Summary:  "你的第三方应用 Active notify 已通过审核。",
		Content:  "你的第三方应用 `Active notify` 已通过审核，可以开始使用 OAuth 授权能力。",
		Level:    noticesvc.LevelSuccess,
		Audience: noticesvc.AudienceTargeted,
		LinkURL:  "/dashboard/oauth",
		Target:   owner.ID,
		MinEnds:  beforeCreate + int64((30*24*60*60*1000)-1000),
	})
	if _, err := svc.ReviewClient(ctx, adminActor, activeID, oauth.StatusDisabled, "Security issue"); err != nil {
		t.Fatal(err)
	}
	assertNoticeRow(t, db, "第三方应用已停用：Active notify", noticeExpectation{
		Summary:  "你的第三方应用 Active notify 已被管理员停用。",
		Content:  "你的第三方应用 `Active notify` 已被管理员停用。\n\n原因：\n\nSecurity issue",
		Level:    noticesvc.LevelWarning,
		Audience: noticesvc.AudienceTargeted,
		LinkURL:  "/dashboard/oauth",
		Target:   owner.ID,
		MinEnds:  beforeCreate + int64((30*24*60*60*1000)-1000),
	})
}
