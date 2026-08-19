package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestNoticeAdminRoutesCreateReplaceListDeleteExactFlow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-notice-route@test.com", "Password123", "AdminNoticeRoute", true)
	reader := testutil.CreateUser(t, db, "admin-notice-reader@test.com", "Password123", "AdminNoticeReader", false)

	req := adminNoticeRequest(http.MethodPost, "/v2/admin/notifications", `{
		"title":"Route notice",
		"summary":"Route summary",
		"content_markdown":"Route **body**",
		"display_mode":"detail",
		"level":"warning",
		"link_text":"Read more",
		"link_url":"/notifications/route",
		"audience":"users",
		"enabled":true,
		"pinned":true,
		"dismissible":true
	}`, adminUser.ID)
	rec := httptest.NewRecorder()
	h.CreateNotice(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", rec.Code, rec.Body.String())
	}
	created := decodeAdminNoticeBody(t, rec)
	oldID := created["id"].(string)
	if oldID == "" ||
		created["title"] != "Route notice" ||
		created["summary"] != "Route summary" ||
		created["content_markdown"] != "Route **body**" ||
		created["display_mode"] != "detail" ||
		created["level"] != "warning" ||
		created["link_text"] != "Read more" ||
		created["link_url"] != "/notifications/route" ||
		created["audience"] != "users" ||
		created["enabled"] != true ||
		created["pinned"] != true ||
		created["dismissible"] != true ||
		created["created_by"] != adminUser.ID {
		t.Fatalf("created notice body mismatch: %#v", created)
	}

	listReq := adminNoticeRequest(http.MethodGet, "/v2/admin/notifications?type=announcement&status=enabled&limit=10", "", adminUser.ID)
	rec = httptest.NewRecorder()
	h.Notices(rec, listReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%q", rec.Code, rec.Body.String())
	}
	list := decodeAdminNoticeBody(t, rec)
	items := list["items"].([]any)
	if list["page_size"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["id"] != oldID {
		t.Fatalf("enabled list mismatch: %#v", list)
	}

	if err := db.Notices.MarkRead(context.Background(), oldID, reader.ID, 1700000000000); err != nil {
		t.Fatal(err)
	}
	patchReq := adminNoticeRequest(http.MethodPatch, "/v2/admin/notifications/"+oldID, `{
		"title":"Route replacement",
		"summary":"Route replacement summary",
		"content_markdown":"",
		"display_mode":"inline",
		"enabled":false,
		"pinned":false,
		"starts_at":null,
		"ends_at":null
	}`, adminUser.ID)
	patchReq.SetPathValue("id", oldID)
	rec = httptest.NewRecorder()
	h.PatchNotice(rec, patchReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace status=%d body=%q", rec.Code, rec.Body.String())
	}
	replaced := decodeAdminNoticeBody(t, rec)
	newID := replaced["id"].(string)
	if newID == "" ||
		newID == oldID ||
		replaced["title"] != "Route replacement" ||
		replaced["summary"] != "Route replacement summary" ||
		replaced["content_markdown"] != "" ||
		replaced["display_mode"] != "inline" ||
		replaced["enabled"] != false ||
		replaced["pinned"] != false ||
		replaced["starts_at"] != nil ||
		replaced["ends_at"] != nil ||
		replaced["created_by"] != adminUser.ID {
		t.Fatalf("replacement body mismatch: %#v", replaced)
	}
	if old, err := db.Notices.Get(context.Background(), oldID); err != nil || old != nil {
		t.Fatalf("old notice should be deleted: old=%#v err=%v", old, err)
	}
	if countAdminNoticeReceipts(t, db, oldID) != 0 {
		t.Fatalf("old notice receipts should be cascaded")
	}

	disabledReq := adminNoticeRequest(http.MethodGet, "/v2/admin/notifications?type=announcement&status=disabled&limit=10", "", adminUser.ID)
	rec = httptest.NewRecorder()
	h.Notices(rec, disabledReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("disabled list status=%d body=%q", rec.Code, rec.Body.String())
	}
	disabled := decodeAdminNoticeBody(t, rec)
	disabledItems := disabled["items"].([]any)
	if len(disabledItems) != 1 || disabledItems[0].(map[string]any)["id"] != newID {
		t.Fatalf("disabled list mismatch: %#v", disabled)
	}

	deleteReq := adminNoticeRequest(http.MethodDelete, "/v2/admin/notifications/"+newID, "", adminUser.ID)
	deleteReq.SetPathValue("id", newID)
	rec = httptest.NewRecorder()
	h.DeleteNotice(rec, deleteReq)
	if rec.Code != http.StatusNoContent || rec.Body.String() != "" {
		t.Fatalf("delete mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if row, err := db.Notices.Get(context.Background(), newID); err != nil || row != nil {
		t.Fatalf("deleted notice should be absent: row=%#v err=%v", row, err)
	}
}

func TestNoticeAdminRoutesRejectInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-notice-errors@test.com", "Password123", "AdminNoticeErrors", true)

	rec := httptest.NewRecorder()
	h.CreateNotice(rec, adminNoticeRequest(http.MethodPost, "/v2/admin/notifications", `{`, adminUser.ID))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("bad create json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.CreateNotice(rec, adminNoticeRequest(http.MethodPost, "/v2/admin/notifications", `{"title":"Broken","summary":"Summary","display_mode":"detail"}`, adminUser.ID))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"notice_content\",\"operation\":\"validate\",\"reason\":\"required\"}}\n" {
		t.Fatalf("bad create validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	createReq := adminNoticeRequest(http.MethodPost, "/v2/admin/notifications", `{"title":"Valid","summary":"Valid summary","display_mode":"inline"}`, adminUser.ID)
	rec = httptest.NewRecorder()
	h.CreateNotice(rec, createReq)
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid create status=%d body=%q", rec.Code, rec.Body.String())
	}
	created := decodeAdminNoticeBody(t, rec)
	id := created["id"].(string)

	patchReq := adminNoticeRequest(http.MethodPatch, "/v2/admin/notifications/"+id, `{"enabled":"yes"}`, adminUser.ID)
	patchReq.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.PatchNotice(rec, patchReq)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"patch_value\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("bad patch bool mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	patchReq = adminNoticeRequest(http.MethodPatch, "/v2/admin/notifications/"+id, `{"link_text":"Open","link_url":"javascript:alert(1)"}`, adminUser.ID)
	patchReq.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	h.PatchNotice(rec, patchReq)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"error\":{\"object\":\"notice_link\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("bad patch link mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	row, err := db.Notices.Get(context.Background(), id)
	if err != nil || row == nil || row.Title != "Valid" || row.LinkText != "" || row.LinkURL != "" {
		t.Fatalf("invalid patch should not mutate or replace notice: row=%#v err=%v", row, err)
	}

	missingReq := adminNoticeRequest(http.MethodPatch, "/v2/admin/notifications/missing", `{"title":"Missing"}`, adminUser.ID)
	missingReq.SetPathValue("id", "missing")
	rec = httptest.NewRecorder()
	h.PatchNotice(rec, missingReq)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"notice\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
		t.Fatalf("missing patch mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestNoticeAdminRoutesRejectMissingFineGrainedPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-notice-permissions@test.com", "Password123", "AdminNoticePermissions", true)

	cases := []struct {
		name        string
		permission  string
		makeRequest func() *http.Request
		call        func(http.ResponseWriter, *http.Request)
	}{
		{"list requires notice read", "notice.read.any", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/v2/admin/notifications", nil)
		}, h.Notices},
		{"create requires notice create", "notice.create.any", func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/v2/admin/notifications", strings.NewReader(`{"title":"Blocked","summary":"Blocked"}`))
		}, h.CreateNotice},
		{"patch requires notice update", "notice.update.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodPatch, "/v2/admin/notifications/blocked", strings.NewReader(`{"title":"Blocked"}`))
			req.SetPathValue("id", "blocked")
			return req
		}, h.PatchNotice},
		{"delete requires notice delete", "notice.delete.any", func() *http.Request {
			req := httptest.NewRequest(http.MethodDelete, "/v2/admin/notifications/blocked", nil)
			req.SetPathValue("id", "blocked")
			return req
		}, h.DeleteNotice},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withAdminActorWithoutPermission(tc.makeRequest(), adminUser.ID, tc.permission)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
				t.Fatalf("permission denial mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func adminNoticeRequest(method, target, body, userID string) *http.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	return withAdminActor(req, userID)
}

func decodeAdminNoticeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return body
}

func countAdminNoticeReceipts(t *testing.T, db *database.DB, noticeID string) int {
	t.Helper()
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1`, noticeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
