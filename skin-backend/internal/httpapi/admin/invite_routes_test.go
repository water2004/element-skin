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

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{"code_base64":"5qyi6L-OLyJc","total_uses":2,"note":"Route Invite"}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusCreated || rec.Body.String() != "{\"code\":\"欢迎/\\\"\\\\\",\"note\":\"Route Invite\",\"total_uses\":2}\n" {
		t.Fatalf("create invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	invite, err := db.Invites.Get(req.Context(), "欢迎/\"\\")
	if err != nil || invite == nil || invite.Code != "欢迎/\"\\" || invite.Note != "Route Invite" || invite.TotalUses == nil || *invite.TotalUses != 2 {
		t.Fatalf("created invite state mismatch: invite=%#v err=%v", invite, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{"code_base64":"bWF4LXRvdGFsLWludml0ZQ","total_uses":2147483647}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusCreated || rec.Body.String() != "{\"code\":\"max-total-invite\",\"note\":\"\",\"total_uses\":2147483647}\n" {
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

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{"note":"Generated Invite"}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusCreated {
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

func TestInviteRoutesCreateUnlimitedInvitePersistsNullExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{"code_base64":"cm91dGUtdW5saW1pdGVk","total_uses":null,"note":"No Limit"}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusCreated || rec.Body.String() != "{\"code\":\"route-unlimited\",\"note\":\"No Limit\",\"total_uses\":null}\n" {
		t.Fatalf("unlimited invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	invite, err := db.Invites.Get(req.Context(), "route-unlimited")
	if err != nil || invite == nil || invite.Code != "route-unlimited" || invite.TotalUses != nil || invite.UsedCount != 0 || invite.Note != "No Limit" {
		t.Fatalf("unlimited invite state mismatch: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesListAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Invites.Create(context.Background(), "route-list-invite", testutil.Pointer(3), "List Invite"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/invites?limit=1", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Invites(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"route-list-invite"`) || !strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("invite list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/invites/cm91dGUtbGlzdC1pbnZpdGU", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("code_base64", "cm91dGUtbGlzdC1pbnZpdGU")
	rec = httptest.NewRecorder()
	h.DeleteInvite(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "route-list-invite"); err != nil || invite != nil {
		t.Fatalf("invite should be deleted: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesRejectInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/invites?cursor=not-base64", nil)
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.Invites(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"pagination_cursor\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invite list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	for _, cursor := range []string{
		util.EncodeCursor(map[string]any{"last_created_at": 1}),
		util.EncodeCursor(map[string]any{"last_created_at": 1.5, "last_code": "invite"}),
	} {
		req = httptest.NewRequest(http.MethodGet, "/v2/admin/invites?cursor="+cursor, nil)
		req = withAdminActor(req, "admin-test-user")
		rec = httptest.NewRecorder()
		h.Invites(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"pagination_cursor\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
			t.Fatalf("invite list malformed cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invite create bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, body := range []string{
		`{"code_base64":""}`,
		`{"code_base64":"%%%"}`,
		`{"code_base64":"YQ=="}`,
		`{"code_base64":"YR"}`,
		`{"code_base64":7}`,
		`{"code_base64":"_w"}`,
		`{"code_base64":"AA"}`,
		`{"code":"legacy-plaintext"}`,
	} {
		req = httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(body))
		req = withAdminActor(req, "admin-test-user")
		rec = httptest.NewRecorder()
		h.CreateInvite(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"invite_code\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
			t.Fatalf("invalid invite encoding body=%s: status=%d response=%q", body, rec.Code, rec.Body.String())
		}
	}

	for _, body := range []string{
		`{"code_base64":"aW52YWxpZC16ZXJv","total_uses":0}`,
		`{"code_base64":"aW52YWxpZC1uZWdhdGl2ZQ","total_uses":-1}`,
		`{"code_base64":"aW52YWxpZC1mcmFjdGlvbg","total_uses":1.5}`,
		`{"code_base64":"aW52YWxpZC1zdHJpbmc","total_uses":"2"}`,
		`{"code_base64":"aW52YWxpZC1kYXRhYmFzZS1vdmVyZmxvdw","total_uses":2147483648}`,
		`{"code_base64":"aW52YWxpZC1pbmV4YWN0","total_uses":9007199254740993}`,
		`{"code_base64":"aW52YWxpZC1vdmVyZmxvdw","total_uses":9223372036854775808}`,
	} {
		req = httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(body))
		req = withAdminActor(req, "admin-test-user")
		rec = httptest.NewRecorder()
		h.CreateInvite(rec, req)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"invite_usage_limit\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
			t.Fatalf("invalid total_uses body=%s: status=%d response=%q", body, rec.Code, rec.Body.String())
		}
	}
	if list, err := db.Invites.List(req.Context(), 10, nil, ""); err != nil || len(list["items"].([]map[string]any)) != 0 {
		t.Fatalf("invalid invite requests must not persist rows: list=%#v err=%v", list, err)
	}

	if err := db.Invites.Create(context.Background(), "existing-invite", testutil.Pointer(1), "Existing"); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{"code_base64":"ZXhpc3RpbmctaW52aXRl","total_uses":2}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
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

	req := httptest.NewRequest(http.MethodDelete, "/v2/admin/invites/bWlzc2luZy1pbnZpdGU", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("code_base64", "bWlzc2luZy1pbnZpdGU")
	rec := httptest.NewRecorder()
	h.DeleteInvite(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("missing invite delete should be idempotent: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "missing-invite"); err != nil || invite != nil {
		t.Fatalf("idempotent delete must not create a row: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesRejectInvalidDeleteEncodingWithoutChangingState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Invites.Create(context.Background(), "kept-invite", testutil.Pointer(1), "Kept"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v2/admin/invites/%25%25%25", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("code_base64", "%%%")
	rec := httptest.NewRecorder()
	h.DeleteInvite(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"invite_code\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid delete encoding response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	invite, err := db.Invites.Get(req.Context(), "kept-invite")
	if err != nil || invite == nil || invite.Code != "kept-invite" || invite.Note != "Kept" {
		t.Fatalf("invalid delete must preserve existing invite: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesRejectMissingPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	tests := []struct {
		name       string
		permission string
		request    *http.Request
		call       func(http.ResponseWriter, *http.Request)
	}{
		{
			name:       "list",
			permission: "invite.read.any",
			request:    httptest.NewRequest(http.MethodGet, "/v2/admin/invites", nil),
			call:       h.Invites,
		},
		{
			name:       "create",
			permission: "invite.create.any",
			request:    httptest.NewRequest(http.MethodPost, "/v2/admin/invites", strings.NewReader(`{"code_base64":"YmxvY2tlZC1pbnZpdGU"}`)),
			call:       h.CreateInvite,
		},
		{
			name:       "delete",
			permission: "invite.delete.any",
			request:    httptest.NewRequest(http.MethodDelete, "/v2/admin/invites/YmxvY2tlZC1pbnZpdGU", nil),
			call:       h.DeleteInvite,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := withAdminActorWithoutPermission(tc.request, "admin-test-user", tc.permission)
			req.SetPathValue("code_base64", "YmxvY2tlZC1pbnZpdGU")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("%s missing permission response mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
	if invite, err := db.Invites.Get(context.Background(), "blocked-invite"); err != nil || invite != nil {
		t.Fatalf("permission-denied invite requests must not persist rows: invite=%#v err=%v", invite, err)
	}
}
