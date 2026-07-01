package notice_test

import (
	"context"
	"testing"

	noticedb "element-skin/backend/internal/database/notice"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestNoticeStoreFiltersReceiptsPaginationAndCleanupExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	creator := testutil.CreateUser(t, db, "notice-creator@test.com", "Password123", "NoticeCreator", true)
	user := testutil.CreateUser(t, db, "notice-user@test.com", "Password123", "NoticeUser", false)
	admin := testutil.CreateUser(t, db, "notice-admin@test.com", "Password123", "NoticeAdmin", true)
	now := int64(1_700_000_000_000)

	for _, item := range []model.Notice{
		testNotice("users-pinned", "Users Pinned", "users", true, true, now-50, nil, nil, creator.ID),
		testNotice("users-new", "Users New", "users", true, false, now-40, nil, nil, creator.ID),
		testNotice("users-old", "Users Old", "users", true, false, now-80, nil, nil, creator.ID),
		testNotice("admins-only", "Admins Only", "admins", true, true, now-30, nil, nil, creator.ID),
		testNotice("disabled", "Disabled", "users", false, false, now-20, nil, nil, creator.ID),
		testNotice("expired", "Expired", "users", true, false, now-10, nil, ptrInt64(now-1), creator.ID),
		testNotice("scheduled", "Scheduled", "users", true, false, now-5, ptrInt64(now+1), nil, creator.ID),
	} {
		if err := db.Notices.Create(ctx, item); err != nil {
			t.Fatalf("create %s: %v", item.ID, err)
		}
	}

	normalPage, err := db.Notices.ListForUser(ctx, noticedb.UserListOptions{UserID: user.ID, Type: "announcement", Limit: 10, Now: now, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	normalItems := normalPage["items"].([]model.NoticeView)
	if len(normalItems) != 3 ||
		normalItems[0].ID != "users-pinned" || !normalItems[0].Pinned ||
		normalItems[1].ID != "users-new" ||
		normalItems[2].ID != "users-old" ||
		normalPage["has_next"] != false ||
		normalPage["next_cursor"] != "" ||
		normalPage["page_size"] != 3 {
		t.Fatalf("normal user page mismatch: %#v", normalPage)
	}
	if normalItems[0].Read || normalItems[0].ReadAt != nil || normalItems[0].DismissedAt != nil {
		t.Fatalf("fresh notice receipt state mismatch: %#v", normalItems[0])
	}

	adminPage, err := db.Notices.ListForUser(ctx, noticedb.UserListOptions{UserID: admin.ID, CanReadAdminAudience: true, Type: "announcement", Limit: 10, Now: now, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	adminItems := adminPage["items"].([]model.NoticeView)
	if len(adminItems) != 4 ||
		adminItems[0].ID != "admins-only" ||
		adminItems[1].ID != "users-pinned" ||
		adminItems[2].ID != "users-new" ||
		adminItems[3].ID != "users-old" {
		t.Fatalf("admin user page should include admin notice first by pinned/time: %#v", adminItems)
	}

	firstPage, err := db.Notices.ListForUser(ctx, noticedb.UserListOptions{UserID: user.ID, Type: "announcement", Limit: 2, Now: now, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]model.NoticeView)
	if len(firstItems) != 2 || firstItems[0].ID != "users-pinned" || firstItems[1].ID != "users-new" || firstPage["has_next"] != true {
		t.Fatalf("first page mismatch: %#v", firstPage)
	}
	cursor, ok := firstPage["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("first page should include cursor: %#v", firstPage)
	}
	secondPage, err := db.Notices.ListForUser(ctx, noticedb.UserListOptions{
		UserID:      user.ID,
		Type:        "announcement",
		Limit:       2,
		Now:         now,
		IncludeRead: true,
		LastPinned:  ptrBool(false),
		LastCreated: ptrInt64(now - 40),
		LastID:      "users-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondItems := secondPage["items"].([]model.NoticeView)
	if len(secondItems) != 1 || secondItems[0].ID != "users-old" || secondPage["has_next"] != false {
		t.Fatalf("second page mismatch: %#v", secondPage)
	}

	if err := db.Notices.MarkRead(ctx, "users-new", user.ID, now+1); err != nil {
		t.Fatal(err)
	}
	unreadPage, err := db.Notices.ListForUser(ctx, noticedb.UserListOptions{UserID: user.ID, Type: "announcement", Limit: 10, Now: now, IncludeRead: false})
	if err != nil {
		t.Fatal(err)
	}
	unreadItems := unreadPage["items"].([]model.NoticeView)
	if len(unreadItems) != 2 || unreadItems[0].ID != "users-pinned" || unreadItems[1].ID != "users-old" {
		t.Fatalf("unread page should exclude read notice: %#v", unreadItems)
	}
	readView, err := db.Notices.GetForUser(ctx, "users-new", user.ID, false)
	if err != nil || readView == nil || !readView.Read || readView.ReadAt == nil || *readView.ReadAt != now+1 {
		t.Fatalf("read receipt mismatch: view=%#v err=%v", readView, err)
	}

	if err := db.Notices.Dismiss(ctx, "users-new", user.ID, now+2); err != nil {
		t.Fatal(err)
	}
	dismissedPage, err := db.Notices.ListForUser(ctx, noticedb.UserListOptions{UserID: user.ID, Type: "announcement", Limit: 10, Now: now, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	dismissedItems := dismissedPage["items"].([]model.NoticeView)
	if len(dismissedItems) != 2 || dismissedItems[0].ID != "users-pinned" || dismissedItems[1].ID != "users-old" {
		t.Fatalf("dismissed notice should be hidden: %#v", dismissedItems)
	}
	dismissedView, err := db.Notices.GetForUser(ctx, "users-new", user.ID, false)
	if err != nil || dismissedView == nil || dismissedView.DismissedAt == nil || *dismissedView.DismissedAt != now+2 {
		t.Fatalf("dismiss receipt mismatch: view=%#v err=%v", dismissedView, err)
	}

	if err := db.Notices.DeleteExpired(ctx, now); err != nil {
		t.Fatal(err)
	}
	if expired, err := db.Notices.Get(ctx, "expired"); err != nil || expired != nil {
		t.Fatalf("expired notice should be deleted: notice=%#v err=%v", expired, err)
	}
	if scheduled, err := db.Notices.Get(ctx, "scheduled"); err != nil || scheduled == nil {
		t.Fatalf("scheduled notice should remain: notice=%#v err=%v", scheduled, err)
	}

	deleted, err := db.Notices.Delete(ctx, "users-new")
	if err != nil || !deleted {
		t.Fatalf("delete users-new=%v err=%v", deleted, err)
	}
	var receipts int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1`, "users-new").Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatalf("delete should cascade receipts, count=%d", receipts)
	}
}

func TestNoticeStoreAdminListUpdateAndReplaceExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	creator := testutil.CreateUser(t, db, "notice-admin-store@test.com", "Password123", "NoticeAdminStore", true)
	reader := testutil.CreateUser(t, db, "notice-admin-store-reader@test.com", "Password123", "NoticeAdminStoreReader", false)
	now := int64(1_800_000_000_000)
	start := now + 60_000
	end := now - 60_000

	for _, item := range []model.Notice{
		testNotice("admin-enabled-pinned", "Admin Enabled Pinned", "users", true, true, now-10, nil, nil, creator.ID),
		testNotice("admin-enabled-new", "Admin Enabled New", "users", true, false, now-20, nil, nil, creator.ID),
		testNotice("admin-disabled", "Admin Disabled", "users", false, false, now-30, nil, nil, creator.ID),
		testNotice("admin-expired", "Admin Expired", "users", true, false, now-40, nil, &end, creator.ID),
		testNotice("admin-scheduled", "Admin Scheduled", "users", true, false, now-50, &start, nil, creator.ID),
	} {
		if err := db.Notices.Create(ctx, item); err != nil {
			t.Fatalf("create %s: %v", item.ID, err)
		}
	}

	firstPage, err := db.Notices.ListForAdmin(ctx, noticedb.AdminListOptions{Type: "announcement", Status: "enabled", Limit: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]model.Notice)
	cursor := firstPage["next_cursor"].(string)
	decodedCursor, cursorErr := util.DecodeCursor(cursor)
	if len(firstItems) != 1 ||
		firstItems[0].ID != "admin-enabled-pinned" ||
		firstPage["has_next"] != true ||
		firstPage["page_size"] != 1 ||
		cursor == "" ||
		cursorErr != nil ||
		decodedCursor["last_pinned"] != true ||
		decodedCursor["last_created_at"] != float64(now-10) ||
		decodedCursor["last_id"] != "admin-enabled-pinned" {
		t.Fatalf("admin first page mismatch: page=%#v cursor=%#v err=%v", firstPage, decodedCursor, cursorErr)
	}

	secondPage, err := db.Notices.ListForAdmin(ctx, noticedb.AdminListOptions{
		Type:        "announcement",
		Status:      "enabled",
		Limit:       2,
		Now:         now,
		LastPinned:  ptrBool(true),
		LastCreated: ptrInt64(now - 10),
		LastID:      "admin-enabled-pinned",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondItems := secondPage["items"].([]model.Notice)
	if len(secondItems) != 2 ||
		secondItems[0].ID != "admin-enabled-new" ||
		secondItems[1].ID != "admin-expired" ||
		secondPage["has_next"] != true ||
		secondPage["next_cursor"] == "" {
		t.Fatalf("admin second page mismatch: %#v", secondPage)
	}

	for _, tc := range []struct {
		status string
		want   string
	}{
		{status: "disabled", want: "admin-disabled"},
		{status: "expired", want: "admin-expired"},
		{status: "scheduled", want: "admin-scheduled"},
	} {
		page, err := db.Notices.ListForAdmin(ctx, noticedb.AdminListOptions{Type: "announcement", Status: tc.status, Limit: 5, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		items := page["items"].([]model.Notice)
		if len(items) != 1 || items[0].ID != tc.want || page["page_size"] != 1 || page["has_next"] != false {
			t.Fatalf("%s admin page mismatch: %#v", tc.status, page)
		}
	}

	updated := testNotice("admin-disabled", "Admin Disabled Updated", "admins", true, true, now+1, nil, nil, creator.ID)
	updated.Type = "system"
	updated.Level = "danger"
	got, err := db.Notices.Update(ctx, updated)
	if err != nil ||
		got == nil ||
		got.ID != "admin-disabled" ||
		got.Type != "system" ||
		got.Title != "Admin Disabled Updated" ||
		got.Audience != "admins" ||
		got.Level != "danger" ||
		!got.Enabled ||
		!got.Pinned ||
		got.UpdatedAt != now+1 {
		t.Fatalf("update mismatch: got=%#v err=%v", got, err)
	}
	if missing, err := db.Notices.Update(ctx, testNotice("missing-admin-update", "Missing", "users", true, false, now, nil, nil, creator.ID)); err != nil || missing != nil {
		t.Fatalf("missing update should return nil: got=%#v err=%v", missing, err)
	}

	if err := db.Notices.MarkRead(ctx, "admin-enabled-new", reader.ID, now+2); err != nil {
		t.Fatal(err)
	}
	replacement := testNotice("admin-replacement", "Admin Replacement", "users", true, false, now+3, nil, nil, creator.ID)
	replaced, err := db.Notices.Replace(ctx, "admin-enabled-new", replacement)
	if err != nil || !replaced {
		t.Fatalf("replace should succeed: replaced=%v err=%v", replaced, err)
	}
	if old, err := db.Notices.Get(ctx, "admin-enabled-new"); err != nil || old != nil {
		t.Fatalf("old replaced notice should be absent: old=%#v err=%v", old, err)
	}
	if newNotice, err := db.Notices.Get(ctx, "admin-replacement"); err != nil || newNotice == nil || newNotice.Title != "Admin Replacement" {
		t.Fatalf("new replacement notice mismatch: notice=%#v err=%v", newNotice, err)
	}
	var oldReceipts int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1`, "admin-enabled-new").Scan(&oldReceipts); err != nil {
		t.Fatal(err)
	}
	if oldReceipts != 0 {
		t.Fatalf("replace should cascade old receipts, got %d", oldReceipts)
	}
	if missing, err := db.Notices.Replace(ctx, "missing-admin-replace", testNotice("unused-replace", "Unused", "users", true, false, now, nil, nil, creator.ID)); err != nil || missing {
		t.Fatalf("missing replace should return false: replaced=%v err=%v", missing, err)
	}
}

func testNotice(id, title, audience string, enabled, pinned bool, createdAt int64, startsAt, endsAt *int64, createdBy string) model.Notice {
	return model.Notice{
		ID:              id,
		Type:            "announcement",
		Title:           title,
		Summary:         title + " summary",
		ContentMarkdown: title + " body",
		DisplayMode:     "inline",
		Level:           "info",
		Audience:        audience,
		Enabled:         enabled,
		Pinned:          pinned,
		Dismissible:     true,
		StartsAt:        startsAt,
		EndsAt:          endsAt,
		CreatedBy:       &createdBy,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
}

func ptrInt64(v int64) *int64 { return &v }

func ptrBool(v bool) *bool { return &v }
