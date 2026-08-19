package notice_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeUserActionsRejectMissingPermissionsBeforeDatabaseAccess(t *testing.T) {
	svc := noticesvc.Service{}
	ctx := context.Background()
	actor := noticeActor("notice-denied-user")

	if result, err := svc.ListForUser(ctx, actor, noticesvc.ListParams{}); result != nil || !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("denied list result=%#v err=%#v", result, err)
	}
	if result, err := svc.GetForUser(ctx, "notice-id", actor); result != nil || !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("denied detail result=%#v err=%#v", result, err)
	}
	if err := svc.MarkRead(ctx, "notice-id", actor); !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("denied mark read err=%#v", err)
	}
	if err := svc.Dismiss(ctx, "notice-id", actor); !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("denied dismiss err=%#v", err)
	}
}

func TestNoticeServiceUserVisibilityReadDismissAndPatchExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-root@test.com", "Password123", "NoticeServiceRoot", true)
	actor := noticeManagerActor(admin.ID)
	user := testutil.CreateUser(t, db, "notice-service-user@test.com", "Password123", "NoticeServiceUser", false)
	now := database.NowMS()

	detail, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Developer Notice",
		Summary:         "OAuth applications are coming",
		ContentMarkdown: "Full **markdown** body",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "Open",
		LinkURL:         "/notifications/dev",
		StartsAt:        ptrInt64(now - 1000),
		EndsAt:          ptrInt64(now + 1000),
		Dismissible:     ptrBool(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Type != noticesvc.TypeAnnouncement || detail.Audience != noticesvc.AudienceUsers || !detail.Enabled ||
		detail.Title != "Developer Notice" || detail.Level != noticesvc.LevelWarning || detail.CreatedBy == nil || *detail.CreatedBy != admin.ID {
		t.Fatalf("created detail notice mismatch: %#v", detail)
	}

	reader := noticeActor(user.ID, "notice.read.owned", "notice.dismiss.owned")
	got, err := svc.GetForUser(ctx, detail.ID, reader)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != detail.ID || !got.Read || got.ReadAt == nil || got.ContentMarkdown != "Full **markdown** body" {
		t.Fatalf("GetForUser should mark read and return exact notice: %#v", got)
	}
	readAgain, err := svc.GetForUser(ctx, detail.ID, reader)
	if err != nil {
		t.Fatal(err)
	}
	if readAgain.ReadAt == nil || *readAgain.ReadAt != *got.ReadAt {
		t.Fatalf("read timestamp should remain idempotent: first=%#v second=%#v", got, readAgain)
	}
	if err := svc.Dismiss(ctx, detail.ID, reader); !httpError(err, 403, "notice.dismiss.denied") {
		t.Fatalf("non-dismissible notice should reject exactly, got %#v", err)
	}

	updated, err := svc.Patch(ctx, actor, detail.ID, noticesvc.PatchInput{
		Summary:         ptrString("Updated summary"),
		EndsAt:          nil,
		ClearEndsAt:     true,
		Dismissible:     ptrBool(true),
		DisplayMode:     ptrString(noticesvc.DisplayDetail),
		ContentMarkdown: ptrString("Updated body"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID == detail.ID ||
		updated.Summary != "Updated summary" ||
		updated.EndsAt != nil ||
		!updated.Dismissible ||
		updated.ContentMarkdown != "Updated body" ||
		updated.CreatedBy == nil ||
		*updated.CreatedBy != admin.ID {
		t.Fatalf("patch should replace with a new exact notice: %#v", updated)
	}
	if old, err := db.Notices.Get(ctx, detail.ID); err != nil || old != nil {
		t.Fatalf("patch should delete old notice: old=%#v err=%v", old, err)
	}
	var oldReceipts int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1`, detail.ID).Scan(&oldReceipts); err != nil {
		t.Fatal(err)
	}
	if oldReceipts != 0 {
		t.Fatalf("patch should cascade old receipts, got %d", oldReceipts)
	}
	if err := svc.Dismiss(ctx, updated.ID, reader); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListForUser(ctx, reader, noticesvc.ListParams{Type: noticesvc.TypeAnnouncement, Limit: 10, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	if items := list["items"].([]model.NoticeView); len(items) != 0 {
		t.Fatalf("dismissed notice should disappear from list: %#v", list)
	}
}
