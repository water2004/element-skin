package notice_test

import (
	"context"
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeServiceAdminListMarkReadDeleteAndCursorErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-admin-list@test.com", "Password123", "NoticeServiceAdminList", true)
	actor := noticeManagerActor(admin.ID)
	user := testutil.CreateUser(t, db, "notice-service-user-list@test.com", "Password123", "NoticeServiceUserList", false)
	now := database.NowMS()

	enabled, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Enabled Notice",
		Summary:         "Enabled summary",
		ContentMarkdown: "Enabled body",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelSuccess,
		Pinned:          ptrBool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	disabledFlag := false
	disabled, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Disabled Notice",
		ContentMarkdown: "Disabled body",
		Enabled:         &disabledFlag,
	})
	if err != nil {
		t.Fatal(err)
	}
	expired, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Expired Notice",
		ContentMarkdown: "Expired body",
		EndsAt:          ptrInt64(now - 1000),
	})
	if err != nil {
		t.Fatal(err)
	}
	scheduled, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Scheduled Notice",
		ContentMarkdown: "Scheduled body",
		StartsAt:        ptrInt64(now + 3_600_000),
	})
	if err != nil {
		t.Fatal(err)
	}

	adminAll, err := svc.ListForManagement(ctx, actor, noticesvc.ListParams{Status: noticesvc.StatusAll, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	allItems := adminAll["items"].([]model.Notice)
	if len(allItems) != 4 || allItems[0].ID != scheduled.ID && allItems[0].ID != expired.ID && allItems[0].ID != disabled.ID && allItems[0].ID != enabled.ID {
		t.Fatalf("admin all list should contain four notices: %#v", allItems)
	}
	statusCases := []struct {
		status  string
		wantIDs []string
	}{
		{noticesvc.StatusEnabled, []string{enabled.ID, expired.ID, scheduled.ID}},
		{noticesvc.StatusDisabled, []string{disabled.ID}},
		{noticesvc.StatusExpired, []string{expired.ID}},
		{noticesvc.StatusScheduled, []string{scheduled.ID}},
	}
	for _, tc := range statusCases {
		got, err := svc.ListForManagement(ctx, actor, noticesvc.ListParams{Status: tc.status, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		items := got["items"].([]model.Notice)
		gotIDs := make([]string, 0, len(items))
		for _, item := range items {
			gotIDs = append(gotIDs, item.ID)
		}
		if !sameStringSet(gotIDs, tc.wantIDs) {
			t.Fatalf("admin status %s mismatch: got=%v want=%v", tc.status, gotIDs, tc.wantIDs)
		}
	}
	if _, err := svc.ListForManagement(ctx, actor, noticesvc.ListParams{Status: "archived"}); !httpError(err, 400, "status.validate.invalid") {
		t.Fatalf("invalid admin status error mismatch: %#v", err)
	}
	if _, err := svc.ListForManagement(ctx, actor, noticesvc.ListParams{Type: "other"}); !httpError(err, 400, "type.validate.invalid") {
		t.Fatalf("invalid admin type error mismatch: %#v", err)
	}
	if _, err := svc.ListForManagement(ctx, actor, noticesvc.ListParams{Cursor: "not-a-cursor"}); !httpError(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("invalid admin cursor error mismatch: %#v", err)
	}
	reader := noticeActor(user.ID, "notice.read.owned")
	if _, err := svc.ListForUser(ctx, reader, noticesvc.ListParams{Type: "other"}); !httpError(err, 400, "type.validate.invalid") {
		t.Fatalf("invalid user type error mismatch: %#v", err)
	}
	if _, err := svc.ListForUser(ctx, reader, noticesvc.ListParams{Cursor: "not-a-cursor"}); !httpError(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("invalid user cursor error mismatch: %#v", err)
	}

	if err := svc.MarkRead(ctx, enabled.ID, reader); err != nil {
		t.Fatal(err)
	}
	view, err := db.Notices.GetForUser(ctx, enabled.ID, user.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || !view.Read || view.ReadAt == nil {
		t.Fatalf("MarkRead should persist exact read state: %#v", view)
	}
	if err := svc.MarkRead(ctx, scheduled.ID, reader); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("MarkRead hidden notice error mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, actor, "missing-notice"); !httpError(err, 404, "notice.resolve.not_found") {
		t.Fatalf("delete missing error mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, actor, enabled.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Notices.Get(ctx, enabled.ID); err != nil || got != nil {
		t.Fatalf("Delete should remove row exactly: got=%#v err=%v", got, err)
	}
}

func TestNoticeServicePatchReplacesAllEditableFieldsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-patch-all@test.com", "Password123", "NoticePatchAll", true)
	actor := noticeManagerActor(admin.ID)

	notice, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "Old", Summary: "Old summary", DisplayMode: noticesvc.DisplayInline})
	if err != nil {
		t.Fatal(err)
	}
	startsAt := int64(10_000)
	endsAt := int64(20_000)
	enabled := false
	pinned := true
	dismissible := false
	updated, err := svc.Patch(ctx, actor, notice.ID, noticesvc.PatchInput{
		Type:            ptrString(noticesvc.TypeSystem),
		Title:           ptrString("New title"),
		Summary:         ptrString("New summary"),
		ContentMarkdown: ptrString("New body"),
		DisplayMode:     ptrString(noticesvc.DisplayDetail),
		Level:           ptrString(noticesvc.LevelDanger),
		LinkText:        ptrString("Read"),
		LinkURL:         ptrString("https://example.com/notice"),
		Audience:        ptrString(noticesvc.AudienceAdmins),
		Enabled:         &enabled,
		Pinned:          &pinned,
		Dismissible:     &dismissible,
		StartsAt:        &startsAt,
		EndsAt:          &endsAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID == notice.ID ||
		updated.Type != noticesvc.TypeSystem ||
		updated.Title != "New title" ||
		updated.Summary != "New summary" ||
		updated.ContentMarkdown != "New body" ||
		updated.DisplayMode != noticesvc.DisplayDetail ||
		updated.Level != noticesvc.LevelDanger ||
		updated.LinkText != "Read" ||
		updated.LinkURL != "https://example.com/notice" ||
		updated.Audience != noticesvc.AudienceAdmins ||
		updated.Enabled ||
		!updated.Pinned ||
		updated.Dismissible ||
		updated.StartsAt == nil || *updated.StartsAt != startsAt ||
		updated.EndsAt == nil || *updated.EndsAt != endsAt ||
		updated.CreatedBy == nil || *updated.CreatedBy != admin.ID {
		t.Fatalf("patched notice mismatch: %#v", updated)
	}
}

func TestNoticeServiceSingleFieldPatchPreservesEveryUnsubmittedField(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-patch-single@test.com", "Password123", "NoticePatchSingle", true)
	actor := noticeManagerActor(admin.ID)
	startsAt := int64(10_000)
	endsAt := int64(20_000)
	pinned := true
	dismissible := false
	notice, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Type:            noticesvc.TypeAnnouncement,
		Title:           "Preserved title",
		Summary:         "Preserved summary",
		ContentMarkdown: "Preserved body",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "Open",
		LinkURL:         "/notifications/preserved",
		Audience:        noticesvc.AudienceAdmins,
		Pinned:          &pinned,
		Dismissible:     &dismissible,
		StartsAt:        &startsAt,
		EndsAt:          &endsAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	enabled := false
	updated, err := svc.Patch(ctx, actor, notice.ID, noticesvc.PatchInput{Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}

	expected := *notice
	expected.ID = updated.ID
	expected.Enabled = false
	expected.CreatedAt = updated.CreatedAt
	expected.UpdatedAt = updated.UpdatedAt
	if !reflect.DeepEqual(*updated, expected) {
		t.Fatalf("single-field notice patch changed unsubmitted fields: got=%#v want=%#v", updated, expected)
	}
	if old, err := db.Notices.Get(ctx, notice.ID); err != nil || old != nil {
		t.Fatalf("single-field patch should remove only the replaced notice: old=%#v err=%v", old, err)
	}
}
