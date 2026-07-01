package notice_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestNoticeServiceValidatesInputsWithoutPersistingInvalidRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	admin := testutil.CreateUser(t, db, "notice-service-admin@test.com", "Password123", "NoticeServiceAdmin", true)
	ctx := context.Background()

	cases := []struct {
		name  string
		input noticesvc.CreateInput
		want  string
	}{
		{
			name:  "detail requires summary",
			input: noticesvc.CreateInput{Title: "Detail", ContentMarkdown: "Body", DisplayMode: noticesvc.DisplayDetail},
			want:  "summary is required for detail notices",
		},
		{
			name:  "detail requires content",
			input: noticesvc.CreateInput{Title: "Detail", Summary: "Summary", DisplayMode: noticesvc.DisplayDetail},
			want:  "content_markdown is required for detail notices",
		},
		{
			name:  "invalid link protocol",
			input: noticesvc.CreateInput{Title: "Bad Link", ContentMarkdown: "Body", LinkText: "Open", LinkURL: "javascript:alert(1)"},
			want:  "invalid link_url",
		},
		{
			name:  "link text pair required",
			input: noticesvc.CreateInput{Title: "Half Link", ContentMarkdown: "Body", LinkURL: "/notifications/abc"},
			want:  "link_text and link_url must be provided together",
		},
		{
			name:  "ends after starts",
			input: noticesvc.CreateInput{Title: "Bad Time", ContentMarkdown: "Body", StartsAt: ptrInt64(20), EndsAt: ptrInt64(10)},
			want:  "ends_at must be greater than starts_at",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.Create(ctx, tc.input, admin.ID)
			if created != nil || !httpError(err, 400, tc.want) {
				t.Fatalf("Create()=%#v err=%#v; want nil and %q", created, err, tc.want)
			}
		})
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notices`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid notice creates persisted %d rows; want 0", count)
	}

	inline, err := svc.Create(ctx, noticesvc.CreateInput{Title: "Inline", Summary: "Short text", DisplayMode: noticesvc.DisplayInline}, admin.ID)
	if err != nil {
		t.Fatalf("inline notice without content should be valid: %v", err)
	}
	if inline.Title != "Inline" || inline.Summary != "Short text" || inline.ContentMarkdown != "" || inline.DisplayMode != noticesvc.DisplayInline {
		t.Fatalf("inline notice without content mismatch: %#v", inline)
	}

	system, err := svc.Create(ctx, noticesvc.CreateInput{Type: noticesvc.TypeSystem, Title: "System", Summary: "System text", DisplayMode: noticesvc.DisplayInline}, admin.ID)
	if err != nil {
		t.Fatalf("system notice should be valid: %v", err)
	}
	if system.Type != noticesvc.TypeSystem || system.Title != "System" || system.Summary != "System text" || system.ContentMarkdown != "" {
		t.Fatalf("system notice mismatch: %#v", system)
	}
}

func TestNoticeServiceTargetedAudienceOnlyVisibleToTargets(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-target-admin@test.com", "Password123", "NoticeTargetAdmin", true)
	target := testutil.CreateUser(t, db, "notice-target-user@test.com", "Password123", "NoticeTargetUser", false)
	other := testutil.CreateUser(t, db, "notice-target-other@test.com", "Password123", "NoticeTargetOther", false)

	if created, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:           "Invalid targeted",
		Summary:         "Missing targets",
		ContentMarkdown: "Body",
		DisplayMode:     noticesvc.DisplayDetail,
		Audience:        noticesvc.AudienceTargeted,
	}, admin.ID); created != nil || !httpError(err, 400, "target_user_ids are required for targeted notices") {
		t.Fatalf("targeted without targets mismatch: created=%#v err=%#v", created, err)
	}
	if created, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:         "Invalid audience targets",
		Summary:       "Wrong audience",
		Audience:      noticesvc.AudienceUsers,
		TargetUserIDs: []string{target.ID},
	}, admin.ID); created != nil || !httpError(err, 400, "target_user_ids require targeted audience") {
		t.Fatalf("non-targeted with targets mismatch: created=%#v err=%#v", created, err)
	}

	created, err := svc.Create(ctx, noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "Targeted notice",
		Summary:         "Only one user should see this",
		ContentMarkdown: "Targeted **body**",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelSuccess,
		Audience:        noticesvc.AudienceTargeted,
		TargetUserIDs:   []string{target.ID, target.ID, " "},
	}, admin.ID)
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

	targetPage, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: target.ID}, noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	targetItems := targetPage["items"].([]model.NoticeView)
	if len(targetItems) != 1 || targetItems[0].ID != created.ID || targetItems[0].Read || targetItems[0].Audience != noticesvc.AudienceTargeted {
		t.Fatalf("target user list mismatch: %#v", targetItems)
	}
	otherPage, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: other.ID}, noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if items := otherPage["items"].([]model.NoticeView); len(items) != 0 {
		t.Fatalf("other user should not see targeted notice: %#v", items)
	}
	if _, err := svc.GetForUser(ctx, created.ID, noticesvc.CurrentUser{ID: other.ID}); !httpError(err, 404, "notice not found") {
		t.Fatalf("other user targeted detail mismatch: %#v", err)
	}
	got, err := svc.GetForUser(ctx, created.ID, noticesvc.CurrentUser{ID: target.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != created.ID || !got.Read || got.ContentMarkdown != "Targeted **body**" {
		t.Fatalf("targeted detail mismatch: %#v", got)
	}

	replaced, err := svc.Patch(ctx, created.ID, noticesvc.PatchInput{
		Title:           ptrString("Targeted notice replaced"),
		Summary:         ptrString("Replacement summary"),
		ContentMarkdown: ptrString("Replacement body"),
	}, admin.ID)
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
	if _, err := svc.GetForUser(ctx, created.ID, noticesvc.CurrentUser{ID: target.ID}); !httpError(err, 404, "notice not found") {
		t.Fatalf("old targeted detail mismatch after replace: %#v", err)
	}
	replacedView, err := svc.GetForUser(ctx, replaced.ID, noticesvc.CurrentUser{ID: target.ID})
	if err != nil {
		t.Fatal(err)
	}
	if replacedView.Title != "Targeted notice replaced" || replacedView.ContentMarkdown != "Replacement body" {
		t.Fatalf("replaced targeted detail mismatch: %#v", replacedView)
	}
	if _, err := svc.GetForUser(ctx, replaced.ID, noticesvc.CurrentUser{ID: other.ID}); !httpError(err, 404, "notice not found") {
		t.Fatalf("other user replaced targeted detail mismatch: %#v", err)
	}
}

func TestNoticeServiceUserVisibilityReadDismissAndPatchExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-root@test.com", "Password123", "NoticeServiceRoot", true)
	user := testutil.CreateUser(t, db, "notice-service-user@test.com", "Password123", "NoticeServiceUser", false)
	now := database.NowMS()

	detail, err := svc.Create(ctx, noticesvc.CreateInput{
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
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Type != noticesvc.TypeAnnouncement || detail.Audience != noticesvc.AudienceUsers || !detail.Enabled ||
		detail.Title != "Developer Notice" || detail.Level != noticesvc.LevelWarning || detail.CreatedBy == nil || *detail.CreatedBy != admin.ID {
		t.Fatalf("created detail notice mismatch: %#v", detail)
	}

	got, err := svc.GetForUser(ctx, detail.ID, noticesvc.CurrentUser{ID: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != detail.ID || !got.Read || got.ReadAt == nil || got.ContentMarkdown != "Full **markdown** body" {
		t.Fatalf("GetForUser should mark read and return exact notice: %#v", got)
	}
	readAgain, err := svc.GetForUser(ctx, detail.ID, noticesvc.CurrentUser{ID: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if readAgain.ReadAt == nil || *readAgain.ReadAt != *got.ReadAt {
		t.Fatalf("read timestamp should remain idempotent: first=%#v second=%#v", got, readAgain)
	}
	if err := svc.Dismiss(ctx, detail.ID, noticesvc.CurrentUser{ID: user.ID}); !httpError(err, 403, "notice is not dismissible") {
		t.Fatalf("non-dismissible notice should reject exactly, got %#v", err)
	}

	updated, err := svc.Patch(ctx, detail.ID, noticesvc.PatchInput{
		Summary:         ptrString("Updated summary"),
		EndsAt:          nil,
		ClearEndsAt:     true,
		Dismissible:     ptrBool(true),
		DisplayMode:     ptrString(noticesvc.DisplayDetail),
		ContentMarkdown: ptrString("Updated body"),
	}, admin.ID)
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
	if err := svc.Dismiss(ctx, updated.ID, noticesvc.CurrentUser{ID: user.ID}); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: user.ID}, noticesvc.ListParams{Type: noticesvc.TypeAnnouncement, Limit: 10, IncludeRead: true})
	if err != nil {
		t.Fatal(err)
	}
	if items := list["items"].([]model.NoticeView); len(items) != 0 {
		t.Fatalf("dismissed notice should disappear from list: %#v", list)
	}
}

func TestNoticeServiceAudienceAndLifecycleVisibilityExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-admin-only@test.com", "Password123", "NoticeServiceAdminOnly", true)
	user := testutil.CreateUser(t, db, "notice-service-hidden-user@test.com", "Password123", "NoticeServiceHiddenUser", false)
	now := database.NowMS()

	adminOnly, err := svc.Create(ctx, noticesvc.CreateInput{Title: "Admin Only", ContentMarkdown: "Body", Audience: noticesvc.AudienceAdmins}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetForUser(ctx, adminOnly.ID, noticesvc.CurrentUser{ID: user.ID}); !httpError(err, 404, "notice not found") {
		t.Fatalf("normal user should not see admin notice, got %#v", err)
	}
	if got, err := svc.GetForUser(ctx, adminOnly.ID, noticesvc.CurrentUser{ID: admin.ID, CanReadAdminAudience: true}); err != nil || got == nil || got.ID != adminOnly.ID {
		t.Fatalf("admin should see admin notice: got=%#v err=%v", got, err)
	}

	scheduled, err := svc.Create(ctx, noticesvc.CreateInput{Title: "Scheduled", ContentMarkdown: "Body", StartsAt: ptrInt64(now + 3_600_000)}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetForUser(ctx, scheduled.ID, noticesvc.CurrentUser{ID: admin.ID, CanReadAdminAudience: true}); !httpError(err, 404, "notice not found") {
		t.Fatalf("scheduled notice should be hidden, got %#v", err)
	}
	expired, err := svc.Create(ctx, noticesvc.CreateInput{Title: "Expired Soon", ContentMarkdown: "Body", EndsAt: ptrInt64(now + 1)}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteExpired(ctx, now+2); err != nil {
		t.Fatal(err)
	}
	if row, err := db.Notices.Get(ctx, expired.ID); err != nil || row != nil {
		t.Fatalf("expired cleanup should delete row: row=%#v err=%v", row, err)
	}
}

func TestNoticeServiceAdminListMarkReadDeleteAndCursorErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-admin-list@test.com", "Password123", "NoticeServiceAdminList", true)
	user := testutil.CreateUser(t, db, "notice-service-user-list@test.com", "Password123", "NoticeServiceUserList", false)
	now := database.NowMS()

	enabled, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:           "Enabled Notice",
		Summary:         "Enabled summary",
		ContentMarkdown: "Enabled body",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelSuccess,
		Pinned:          ptrBool(true),
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	disabledFlag := false
	disabled, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:           "Disabled Notice",
		ContentMarkdown: "Disabled body",
		Enabled:         &disabledFlag,
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:           "Expired Notice",
		ContentMarkdown: "Expired body",
		EndsAt:          ptrInt64(now - 1000),
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	scheduled, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:           "Scheduled Notice",
		ContentMarkdown: "Scheduled body",
		StartsAt:        ptrInt64(now + 3_600_000),
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	adminAll, err := svc.ListForAdmin(ctx, noticesvc.ListParams{Status: noticesvc.StatusAll, Limit: 10})
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
		got, err := svc.ListForAdmin(ctx, noticesvc.ListParams{Status: tc.status, Limit: 10})
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
	if _, err := svc.ListForAdmin(ctx, noticesvc.ListParams{Status: "archived"}); !httpError(err, 400, "invalid status") {
		t.Fatalf("invalid admin status error mismatch: %#v", err)
	}
	if _, err := svc.ListForAdmin(ctx, noticesvc.ListParams{Type: "other"}); !httpError(err, 400, "invalid type") {
		t.Fatalf("invalid admin type error mismatch: %#v", err)
	}
	if _, err := svc.ListForAdmin(ctx, noticesvc.ListParams{Cursor: "not-a-cursor"}); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid admin cursor error mismatch: %#v", err)
	}
	if _, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: user.ID}, noticesvc.ListParams{Type: "other"}); !httpError(err, 400, "invalid type") {
		t.Fatalf("invalid user type error mismatch: %#v", err)
	}
	if _, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: user.ID}, noticesvc.ListParams{Cursor: "not-a-cursor"}); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid user cursor error mismatch: %#v", err)
	}

	if err := svc.MarkRead(ctx, enabled.ID, noticesvc.CurrentUser{ID: user.ID}); err != nil {
		t.Fatal(err)
	}
	view, err := db.Notices.GetForUser(ctx, enabled.ID, user.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || !view.Read || view.ReadAt == nil {
		t.Fatalf("MarkRead should persist exact read state: %#v", view)
	}
	if err := svc.MarkRead(ctx, scheduled.ID, noticesvc.CurrentUser{ID: user.ID}); !httpError(err, 404, "notice not found") {
		t.Fatalf("MarkRead hidden notice error mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, "missing-notice"); !httpError(err, 404, "notice not found") {
		t.Fatalf("delete missing error mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, enabled.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Notices.Get(ctx, enabled.ID); err != nil || got != nil {
		t.Fatalf("Delete should remove row exactly: got=%#v err=%v", got, err)
	}
}

func TestNoticeServiceValidationCoversLengthsLevelsAudienceLinksAndPatchClearsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-validation2@test.com", "Password123", "NoticeServiceValidation2", true)
	base := noticesvc.CreateInput{Title: "Valid", ContentMarkdown: "Body"}
	cases := []struct {
		name   string
		mutate func(*noticesvc.CreateInput)
		detail string
	}{
		{"invalid type", func(in *noticesvc.CreateInput) { in.Type = "other" }, "invalid type"},
		{"title required", func(in *noticesvc.CreateInput) { in.Title = " " }, "title is required"},
		{"title too long", func(in *noticesvc.CreateInput) { in.Title = strings.Repeat("测", noticesvc.MaxTitleLen+1) }, "title too long"},
		{"summary too long", func(in *noticesvc.CreateInput) { in.Summary = strings.Repeat("测", noticesvc.MaxSummaryLen+1) }, "summary too long"},
		{"content too long", func(in *noticesvc.CreateInput) { in.ContentMarkdown = strings.Repeat("a", noticesvc.MaxContentLen+1) }, "content_markdown too long"},
		{"invalid display", func(in *noticesvc.CreateInput) { in.DisplayMode = "popup" }, "invalid display_mode"},
		{"invalid level", func(in *noticesvc.CreateInput) { in.Level = "loud" }, "invalid level"},
		{"invalid audience", func(in *noticesvc.CreateInput) { in.Audience = "guests" }, "invalid audience"},
		{"unsafe protocol-relative link", func(in *noticesvc.CreateInput) { in.LinkText = "Open"; in.LinkURL = "//evil.example" }, "invalid link_url"},
		{"unsafe control link", func(in *noticesvc.CreateInput) { in.LinkText = "Open"; in.LinkURL = "/ok\nbad" }, "invalid link_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.mutate(&input)
			got, err := svc.Create(ctx, input, admin.ID)
			if got != nil || !httpError(err, 400, tc.detail) {
				t.Fatalf("Create invalid case got=%#v err=%#v want %q", got, err, tc.detail)
			}
		})
	}
	start := database.NowMS() - 1000
	end := database.NowMS() + 1000
	notice, err := svc.Create(ctx, noticesvc.CreateInput{
		Title:           "Patch clear",
		Summary:         "Patch summary",
		ContentMarkdown: "Patch body",
		DisplayMode:     noticesvc.DisplayDetail,
		StartsAt:        &start,
		EndsAt:          &end,
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Patch(ctx, notice.ID, noticesvc.PatchInput{
		DisplayMode:     ptrString(noticesvc.DisplayInline),
		ContentMarkdown: ptrString(""),
		ClearStartsAt:   true,
		ClearEndsAt:     true,
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.StartsAt != nil || updated.EndsAt != nil || updated.DisplayMode != noticesvc.DisplayInline || updated.ContentMarkdown != "" {
		t.Fatalf("patch clear fields mismatch: %#v", updated)
	}
}

func TestNoticeServiceCursorPaginationUsesExactOrderAndDashboardDefaults(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-cursor-admin@test.com", "Password123", "NoticeCursorAdmin", true)
	user := testutil.CreateUser(t, db, "notice-service-cursor-user@test.com", "Password123", "NoticeCursorUser", false)

	first, err := svc.Create(ctx, noticesvc.CreateInput{Title: "First", Summary: "First summary", DisplayMode: noticesvc.DisplayInline, Pinned: ptrBool(true)}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create(ctx, noticesvc.CreateInput{Title: "Second", Summary: "Second summary", DisplayMode: noticesvc.DisplayInline}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	third, err := svc.Create(ctx, noticesvc.CreateInput{Type: noticesvc.TypeSystem, Title: "System", Summary: "System summary", DisplayMode: noticesvc.DisplayInline}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	exactCreated := map[string]int64{first.ID: 3000, second.ID: 2000, third.ID: 1000}
	for id, created := range exactCreated {
		if _, err := db.Pool.Exec(ctx, `UPDATE notices SET created_at=$2, updated_at=$2 WHERE id=$1`, id, created); err != nil {
			t.Fatal(err)
		}
	}

	page1, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: user.ID}, noticesvc.ListParams{Limit: 1, Dashboard: true})
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

	page2, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: user.ID}, noticesvc.ListParams{Limit: 1, Dashboard: true, Cursor: cursor})
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

	allTypes, err := svc.ListForUser(ctx, noticesvc.CurrentUser{ID: user.ID}, noticesvc.ListParams{Limit: 10, IncludeRead: true})
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

func TestNoticeServicePatchReplacesAllEditableFieldsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-patch-all@test.com", "Password123", "NoticePatchAll", true)

	notice, err := svc.Create(ctx, noticesvc.CreateInput{Title: "Old", Summary: "Old summary", DisplayMode: noticesvc.DisplayInline}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	startsAt := int64(10_000)
	endsAt := int64(20_000)
	enabled := false
	pinned := true
	dismissible := false
	updated, err := svc.Patch(ctx, notice.ID, noticesvc.PatchInput{
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
	}, admin.ID)
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

func TestNoticeServicePropagatesDatabaseErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	db.Close()
	user := noticesvc.CurrentUser{ID: "notice-db-error-user"}

	if _, err := svc.GetForUser(ctx, "notice-id", user); err == nil || err.Error() != "closed pool" {
		t.Fatalf("GetForUser database error=%v; want closed pool", err)
	}
	if err := svc.MarkRead(ctx, "notice-id", user); err == nil || err.Error() != "closed pool" {
		t.Fatalf("MarkRead database error=%v; want closed pool", err)
	}
	if err := svc.Dismiss(ctx, "notice-id", user); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Dismiss database error=%v; want closed pool", err)
	}
	if _, err := svc.Create(ctx, noticesvc.CreateInput{Title: "DB Error", Summary: "summary"}, "admin-id"); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Create database error=%v; want closed pool", err)
	}
	if _, err := svc.Patch(ctx, "notice-id", noticesvc.PatchInput{Title: ptrString("DB Error")}, "admin-id"); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Patch database error=%v; want closed pool", err)
	}
	if err := svc.Delete(ctx, "notice-id"); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Delete database error=%v; want closed pool", err)
	}
	if err := svc.DeleteExpired(ctx, database.NowMS()); err == nil || err.Error() != "closed pool" {
		t.Fatalf("DeleteExpired database error=%v; want closed pool", err)
	}
}

func httpError(err error, status int, detail string) bool {
	he, ok := err.(util.HTTPError)
	return ok && he.Status == status && he.Detail == detail
}

func ptrInt64(v int64) *int64 { return &v }

func ptrBool(v bool) *bool { return &v }

func ptrString(v string) *string { return &v }

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
