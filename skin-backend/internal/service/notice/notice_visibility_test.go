package notice_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeServiceTargetedAudienceOnlyVisibleToTargets(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-target-admin@test.com", "Password123", "NoticeTargetAdmin", true)
	actor := noticeManagerActor(admin.ID)
	target := testutil.CreateUser(t, db, "notice-target-user@test.com", "Password123", "NoticeTargetUser", false)
	other := testutil.CreateUser(t, db, "notice-target-other@test.com", "Password123", "NoticeTargetOther", false)

	if created, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Invalid targeted",
		Summary:         "Missing targets",
		ContentMarkdown: "Body",
		DisplayMode:     noticesvc.DisplayDetail,
		Audience:        noticesvc.AudienceTargeted,
	}); created != nil || !httpError(err, 400, "notice_target.validate.required") {
		t.Fatalf("targeted without targets mismatch: created=%#v err=%#v", created, err)
	}
	if created, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:         "Invalid audience targets",
		Summary:       "Wrong audience",
		Audience:      noticesvc.AudienceUsers,
		TargetUserIDs: []string{target.ID},
	}); created != nil || !httpError(err, 400, "notice_target.validate.unsupported") {
		t.Fatalf("non-targeted with targets mismatch: created=%#v err=%#v", created, err)
	}

	created, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "Targeted notice",
		Summary:         "Only one user should see this",
		ContentMarkdown: "Targeted **body**",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelSuccess,
		Audience:        noticesvc.AudienceTargeted,
		TargetUserIDs:   []string{target.ID, target.ID, " "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Audience != noticesvc.AudienceTargeted || created.Type != noticesvc.TypeSystem || created.Level != noticesvc.LevelSuccess {
		t.Fatalf("created targeted notice mismatch: %#v", created)
	}
	var targetRows int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notice_targets WHERE notice_id=$1 AND user_id=$2`, created.ID, target.ID).Scan(&targetRows); err != nil {
		t.Fatal(err)
	}
	if targetRows != 1 {
		t.Fatalf("target rows=%d want 1", targetRows)
	}

	targetPage, err := svc.ListForUser(ctx, noticeActor(target.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	targetItems := targetPage["items"].([]model.NoticeView)
	if len(targetItems) != 1 || targetItems[0].ID != created.ID || targetItems[0].Read || targetItems[0].Audience != noticesvc.AudienceTargeted {
		t.Fatalf("target user list mismatch: %#v", targetItems)
	}
	otherPage, err := svc.ListForUser(ctx, noticeActor(other.ID, "notice.read.owned"), noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if items := otherPage["items"].([]model.NoticeView); len(items) != 0 {
		t.Fatalf("other user should not see targeted notice: %#v", items)
	}
	if _, err := svc.GetForUser(ctx, created.ID, noticeActor(other.ID, "notice.read.owned")); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("other user targeted detail mismatch: %#v", err)
	}
	got, err := svc.GetForUser(ctx, created.ID, noticeActor(target.ID, "notice.read.owned"))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != created.ID || !got.Read || got.ContentMarkdown != "Targeted **body**" {
		t.Fatalf("targeted detail mismatch: %#v", got)
	}

	replaced, err := svc.Patch(ctx, actor, created.ID, noticesvc.PatchInput{
		Title:           ptrString("Targeted notice replaced"),
		Summary:         ptrString("Replacement summary"),
		ContentMarkdown: ptrString("Replacement body"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ID == created.ID || replaced.Audience != noticesvc.AudienceTargeted || replaced.Title != "Targeted notice replaced" {
		t.Fatalf("replaced targeted notice mismatch: %#v", replaced)
	}
	var oldTargets int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notice_targets WHERE notice_id=$1`, created.ID).Scan(&oldTargets); err != nil {
		t.Fatal(err)
	}
	if oldTargets != 0 {
		t.Fatalf("old target rows=%d want 0", oldTargets)
	}
	var replacedTargets int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notice_targets WHERE notice_id=$1 AND user_id=$2`, replaced.ID, target.ID).Scan(&replacedTargets); err != nil {
		t.Fatal(err)
	}
	if replacedTargets != 1 {
		t.Fatalf("replaced target rows=%d want 1", replacedTargets)
	}
	if _, err := svc.GetForUser(ctx, created.ID, noticeActor(target.ID, "notice.read.owned")); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("old targeted detail mismatch after replace: %#v", err)
	}
	replacedView, err := svc.GetForUser(ctx, replaced.ID, noticeActor(target.ID, "notice.read.owned"))
	if err != nil {
		t.Fatal(err)
	}
	if replacedView.Title != "Targeted notice replaced" || replacedView.ContentMarkdown != "Replacement body" {
		t.Fatalf("replaced targeted detail mismatch: %#v", replacedView)
	}
	if _, err := svc.GetForUser(ctx, replaced.ID, noticeActor(other.ID, "notice.read.owned")); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("other user replaced targeted detail mismatch: %#v", err)
	}
}

func TestNoticeServiceAudienceAndLifecycleVisibilityExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-admin-only@test.com", "Password123", "NoticeServiceAdminOnly", true)
	actor := noticeManagerActor(admin.ID)
	user := testutil.CreateUser(t, db, "notice-service-hidden-user@test.com", "Password123", "NoticeServiceHiddenUser", false)
	now := database.NowMS()

	adminOnly, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "Admin Only", ContentMarkdown: "Body", Audience: noticesvc.AudienceAdmins})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetForUser(ctx, adminOnly.ID, noticeActor(user.ID, "notice.read.owned")); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("normal user should not see admin notice, got %#v", err)
	}
	if got, err := svc.GetForUser(ctx, adminOnly.ID, noticeActor(admin.ID, "notice.read.owned", "notice.read.any")); err != nil || got == nil || got.ID != adminOnly.ID {
		t.Fatalf("admin should see admin notice: got=%#v err=%v", got, err)
	}

	scheduled, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "Scheduled", ContentMarkdown: "Body", StartsAt: ptrInt64(now + 3_600_000)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetForUser(ctx, scheduled.ID, noticeActor(admin.ID, "notice.read.owned", "notice.read.any")); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("scheduled notice should be hidden, got %#v", err)
	}
	expired, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "Expired Soon", ContentMarkdown: "Body", EndsAt: ptrInt64(now + 1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteExpired(ctx, permission.SystemMaintenanceActor(), now+2); err != nil {
		t.Fatal(err)
	}
	if row, err := db.Notices.Get(ctx, expired.ID); err != nil || row != nil {
		t.Fatalf("expired cleanup should delete row: row=%#v err=%v", row, err)
	}
}

func TestNoticeServiceCursorPaginationUsesExactOrderAndDashboardDefaults(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-cursor-admin@test.com", "Password123", "NoticeCursorAdmin", true)
	actor := noticeManagerActor(admin.ID)
	user := testutil.CreateUser(t, db, "notice-service-cursor-user@test.com", "Password123", "NoticeCursorUser", false)

	first, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "First", Summary: "First summary", DisplayMode: noticesvc.DisplayInline, Pinned: ptrBool(true)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "Second", Summary: "Second summary", DisplayMode: noticesvc.DisplayInline})
	if err != nil {
		t.Fatal(err)
	}
	third, err := svc.Create(ctx, actor, noticesvc.CreateInput{Type: noticesvc.TypeSystem, Title: "System", Summary: "System summary", DisplayMode: noticesvc.DisplayInline})
	if err != nil {
		t.Fatal(err)
	}
	exactCreated := map[string]int64{first.ID: 3000, second.ID: 2000, third.ID: 1000}
	for id, created := range exactCreated {
		if _, err := db.Pool.Exec(ctx, `UPDATE notices SET created_at=$2, updated_at=$2 WHERE id=$1`, id, created); err != nil {
			t.Fatal(err)
		}
	}

	page1, err := svc.ListForUser(ctx, noticeActor(user.ID, "notice.read.owned"), noticesvc.ListParams{Limit: 1, Dashboard: true})
	if err != nil {
		t.Fatal(err)
	}
	items1 := page1["items"].([]model.NoticeView)
	if len(items1) != 1 || items1[0].ID != first.ID || items1[0].Title != "First" || !items1[0].Pinned {
		t.Fatalf("dashboard first page mismatch: %#v", page1)
	}
	if page1["has_next"] != true || page1["page_size"] != 1 {
		t.Fatalf("dashboard first page metadata mismatch: %#v", page1)
	}
	cursor, ok := page1["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("dashboard first page cursor mismatch: %#v", page1)
	}

	page2, err := svc.ListForUser(ctx, noticeActor(user.ID, "notice.read.owned"), noticesvc.ListParams{Limit: 1, Dashboard: true, Cursor: cursor})
	if err != nil {
		t.Fatal(err)
	}
	items2 := page2["items"].([]model.NoticeView)
	if len(items2) != 1 || items2[0].ID != second.ID || items2[0].Title != "Second" || items2[0].Type != noticesvc.TypeAnnouncement {
		t.Fatalf("dashboard second page mismatch: %#v", page2)
	}
	if page2["has_next"] != false || page2["next_cursor"] != "" || page2["page_size"] != 1 {
		t.Fatalf("dashboard second page metadata mismatch: %#v", page2)
	}

	allTypes, err := svc.ListForUser(ctx, noticeActor(user.ID, "notice.read.owned"), noticesvc.ListParams{Limit: 10, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	allItems := allTypes["items"].([]model.NoticeView)
	gotIDs := make([]string, 0, len(allItems))
	for _, item := range allItems {
		gotIDs = append(gotIDs, item.ID)
	}
	if !sameStringSet(gotIDs, []string{first.ID, second.ID, third.ID}) {
		t.Fatalf("non-dashboard list should include announcement and system notices: got=%v", gotIDs)
	}
}
