package notice_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeServiceSystemMaintenanceActorCanOnlyCreateSystemNotices(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	actor := permission.SystemMaintenanceActor()

	created, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "System update",
		Summary:         "System task delivered this notice",
		ContentMarkdown: "Maintenance detail",
		DisplayMode:     noticesvc.DisplayDetail,
		Audience:        noticesvc.AudienceAdmins,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Type != noticesvc.TypeSystem ||
		created.Title != "System update" ||
		created.CreatedBy != nil {
		t.Fatalf("system-created notice mismatch: %#v", created)
	}

	denied, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Type:        noticesvc.TypeAnnouncement,
		Title:       "Announcement",
		Summary:     "Not allowed",
		DisplayMode: noticesvc.DisplayInline,
	})
	if denied != nil || !httpError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("system actor announcement create=%#v err=%#v; want forbidden", denied, err)
	}
}
