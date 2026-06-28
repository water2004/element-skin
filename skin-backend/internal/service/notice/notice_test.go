package notice_test

import (
	"context"
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

func httpError(err error, status int, detail string) bool {
	he, ok := err.(util.HTTPError)
	return ok && he.Status == status && he.Detail == detail
}

func ptrInt64(v int64) *int64 { return &v }

func ptrBool(v bool) *bool { return &v }

func ptrString(v string) *string { return &v }
