package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestInviteRoutesCreateInvitePersistsExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"route-invite","total_uses":2,"note":"Route Invite"}`))
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"code\":\"route-invite\",\"note\":\"Route Invite\",\"total_uses\":2}\n" {
		t.Fatalf("create invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	invite, err := db.Invites.Get(req.Context(), "route-invite")
	if err != nil || invite == nil || invite.Code != "route-invite" || invite.Note != "Route Invite" || invite.TotalUses == nil || *invite.TotalUses != 2 {
		t.Fatalf("created invite state mismatch: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesListAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Invites.Create(t.Context(), "route-list-invite", 3, "List Invite"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/invites?limit=1", nil)
	rec := httptest.NewRecorder()
	h.Invites(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"route-list-invite"`) || !strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("invite list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/invites/route-list-invite", nil)
	req.SetPathValue("code", "route-list-invite")
	rec = httptest.NewRecorder()
	h.DeleteInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "route-list-invite"); err != nil || invite != nil {
		t.Fatalf("invite should be deleted: invite=%#v err=%v", invite, err)
	}
}
