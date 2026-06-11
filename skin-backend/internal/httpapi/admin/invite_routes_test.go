package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
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

	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"max-total-invite","total_uses":2147483647}`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"code\":\"max-total-invite\",\"note\":\"\",\"total_uses\":2147483647}\n" {
		t.Fatalf("max total invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	maxInvite, err := db.Invites.Get(req.Context(), "max-total-invite")
	if err != nil || maxInvite == nil || maxInvite.TotalUses == nil || *maxInvite.TotalUses != 2147483647 {
		t.Fatalf("max total invite state mismatch: invite=%#v err=%v", maxInvite, err)
	}
}

func TestInviteRoutesGenerateCodeWithExactShapeAndDefaults(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"note":"Generated Invite"}`))
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("generated invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	match := regexp.MustCompile(`^\{"code":"([0-9a-f]{40})","note":"Generated Invite","total_uses":1\}\n$`).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("generated invite body has unexpected shape: %q", body)
	}
	invite, err := db.Invites.Get(req.Context(), match[1])
	if err != nil || invite == nil || invite.Code != match[1] || invite.Note != "Generated Invite" ||
		invite.TotalUses == nil || *invite.TotalUses != 1 || invite.UsedCount != 0 {
		t.Fatalf("generated invite state mismatch: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesListAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Invites.Create(context.Background(), "route-list-invite", 3, "List Invite"); err != nil {
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

func TestInviteRoutesRejectInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/invites?cursor=not-base64", nil)
	rec := httptest.NewRecorder()
	h.Invites(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("invite list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	for _, cursor := range []string{
		util.EncodeCursor(map[string]any{"last_created_at": 1}),
		util.EncodeCursor(map[string]any{"last_created_at": 1.5, "last_code": "invite"}),
	} {
		req = httptest.NewRequest(http.MethodGet, "/admin/invites?cursor="+cursor, nil)
		rec = httptest.NewRecorder()
		h.Invites(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
			t.Fatalf("invite list malformed cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("invite create bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"abc","total_uses":5,"note":"too short"}`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invite code too short\"}\n" {
		t.Fatalf("invite short code mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "abc"); err != nil || invite != nil {
		t.Fatalf("short invite code should not persist: invite=%#v err=%v", invite, err)
	}

	for _, body := range []string{
		`{"code":"invalid-zero","total_uses":0}`,
		`{"code":"invalid-negative","total_uses":-1}`,
		`{"code":"invalid-fraction","total_uses":1.5}`,
		`{"code":"invalid-string","total_uses":"2"}`,
		`{"code":"invalid-database-overflow","total_uses":2147483648}`,
		`{"code":"invalid-inexact","total_uses":9007199254740993}`,
		`{"code":"invalid-overflow","total_uses":9223372036854775808}`,
	} {
		req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(body))
		rec = httptest.NewRecorder()
		h.CreateInvite(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"total_uses must be a positive integer\"}\n" {
			t.Fatalf("invalid total_uses body=%s: status=%d response=%q", body, rec.Code, rec.Body.String())
		}
	}
	if list, err := db.Invites.List(req.Context(), 10, nil, ""); err != nil || len(list["items"].([]map[string]any)) != 0 {
		t.Fatalf("invalid invite requests must not persist rows: list=%#v err=%v", list, err)
	}

	if err := db.Invites.Create(context.Background(), "existing-invite", 1, "Existing"); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"existing-invite","total_uses":2}`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("duplicate invite should use generic internal error envelope: status=%d body=%q", rec.Code, rec.Body.String())
	}
	existing, err := db.Invites.Get(req.Context(), "existing-invite")
	if err != nil || existing == nil || existing.TotalUses == nil || *existing.TotalUses != 1 || existing.Note != "Existing" {
		t.Fatalf("duplicate invite should not mutate existing row: invite=%#v err=%v", existing, err)
	}
}

func TestInviteRoutesDeleteMissingInviteIsIdempotent(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodDelete, "/admin/invites/missing-invite", nil)
	req.SetPathValue("code", "missing-invite")
	rec := httptest.NewRecorder()
	h.DeleteInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("missing invite delete should be idempotent: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "missing-invite"); err != nil || invite != nil {
		t.Fatalf("idempotent delete must not create a row: invite=%#v err=%v", invite, err)
	}
}
