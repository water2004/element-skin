package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestUserRoutesListAndProtectCurrentUserExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-users@test.com", "Password123", "AdminUsers", true)
	other := testutil.CreateUser(t, db, "listed-users@test.com", "Password123", "ListedUsers", false)

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/users?limit=1&q=Listed", nil)
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.Users(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+other.ID+`"`) ||
		!strings.Contains(rec.Body.String(), `"email":"listed-users@test.com"`) ||
		!strings.Contains(rec.Body.String(), `"roles":["user"]`) ||
		!strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("admin user list mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/users/"+adminUser.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", adminUser.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.DeleteUser(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot delete yourself") {
		t.Fatalf("self delete should be forbidden exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+adminUser.ID+"/protected-subject/transfer", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", adminUser.ID)
	req = withProtectedActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.TransferProtectedSubject(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"cannot transfer protected subject to yourself\"}\n" {
		t.Fatalf("protected subject self transfer should be rejected exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestUserRoutesDetailProfilesBanUnbanAndResetPassword(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := admin.New(cfg, db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-actions@test.com", "Password123", "AdminUserActions", true)
	target := testutil.CreateUser(t, db, "target-user-actions@test.com", "Password123", "TargetUserActions", false)
	profile := testutil.CreateProfile(t, db, target.ID, "target_user_profile", "TargetUserProfile")

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/users/"+target.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec := httptest.NewRecorder()
	h.User(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+target.ID+`"`) || !strings.Contains(rec.Body.String(), `"email":"target-user-actions@test.com"`) {
		t.Fatalf("user detail response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/admin/users/"+target.ID+"/profiles", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UserProfiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"TargetUserProfile"`) {
		t.Fatalf("user profiles response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	banUntil := time.Now().Add(time.Hour).UnixMilli()
	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/ban", strings.NewReader(`{"banned_until":`+strconvI64(banUntil)+`,"reason":"route test ban"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.BanUser(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"banned_until":`+strconvI64(banUntil)) {
		t.Fatalf("ban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || !banned {
		t.Fatalf("target should be banned: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/"+target.ID+"/unban", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("user_id", target.ID)
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.UnbanUser(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("unban user response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if banned, err := db.Users.IsBanned(req.Context(), target.ID); err != nil || banned {
		t.Fatalf("target should be unbanned: banned=%v err=%v", banned, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/admin/users/password/reset", strings.NewReader(`{"user_id":"`+target.ID+`","new_password":"AdminNewPassword123"}`))
	req = withAdminActor(req, "admin-test-user")
	req = withAdminActor(req, adminUser.ID)
	rec = httptest.NewRecorder()
	h.ResetUserPassword(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("reset password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), target.ID)
	if err != nil || updated == nil || !util.VerifyPassword("AdminNewPassword123", updated.Password) {
		t.Fatalf("reset password should persist new hash: user=%#v err=%v", updated, err)
	}
}

func TestUserProfilesPaginatesEncodedCursorWithoutRepeatingRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	adminUser := testutil.CreateUser(t, db, "admin-user-profile-page@test.com", "Password123", "AdminUserProfilePage", true)
	target := testutil.CreateUser(t, db, "target-user-profile-page@test.com", "Password123", "TargetUserProfilePage", false)
	firstProfile := testutil.CreateProfile(t, db, target.ID, "admin_user_profile_page_a", "ProfilePageA")
	secondProfile := testutil.CreateProfile(t, db, target.ID, "admin_user_profile_page_b", "ProfilePageB")

	requestPage := func(cursor string) *httptest.ResponseRecorder {
		targetURL := "/v2/admin/users/" + target.ID + "/profiles?limit=1"
		if cursor != "" {
			targetURL += "&cursor=" + cursor
		}
		req := httptest.NewRequest(http.MethodGet, targetURL, nil)
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("user_id", target.ID)
		req = withAdminActor(req, adminUser.ID)
		rec := httptest.NewRecorder()
		h.UserProfiles(rec, req)
		return rec
	}
	decodePage := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var page map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}

	firstRec := requestPage("")
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first user profile page status=%d body=%q", firstRec.Code, firstRec.Body.String())
	}
	first := decodePage(firstRec)
	firstItems := first["items"].([]any)
	cursor, _ := first["next_cursor"].(string)
	if len(firstItems) != 1 || firstItems[0].(map[string]any)["id"] != firstProfile.ID || first["has_next"] != true || cursor == "" {
		t.Fatalf("first user profile page mismatch: %#v", first)
	}

	secondRec := requestPage(cursor)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second user profile page status=%d body=%q", secondRec.Code, secondRec.Body.String())
	}
	second := decodePage(secondRec)
	secondItems := second["items"].([]any)
	if len(secondItems) != 1 || secondItems[0].(map[string]any)["id"] != secondProfile.ID ||
		second["has_next"] != false || second["next_cursor"] != "" {
		t.Fatalf("second user profile page mismatch: %#v", second)
	}

	for _, malformed := range []string{
		"not-base64",
		util.EncodeCursor(map[string]any{"unexpected": "value"}),
	} {
		rec := requestPage(malformed)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
			t.Fatalf("malformed user profile cursor status=%d body=%q", rec.Code, rec.Body.String())
		}
	}
}
