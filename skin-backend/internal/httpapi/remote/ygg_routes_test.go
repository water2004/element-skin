package remote_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/remote"
	"element-skin/backend/internal/testutil"
)

func TestRemoteYggRoutesValidateAndReturnExactBodies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := remote.New(db, nil)
	user := testutil.CreateUser(t, db, "remote-direct@test.com", "Password123", "RemoteDirect", false)

	req := httptest.NewRequest(http.MethodPost, "/v1/imports/remote-ygg/profiles/preview", strings.NewReader(`{"profiles":[{"id":"p1","name":"One"}]}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.GetProfiles(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"profiles\":[{\"id\":\"p1\",\"name\":\"One\"}]}\n" {
		t.Fatalf("get profiles exact body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/imports/remote-ygg/profiles/import", strings.NewReader(`{"profile_id":"","profile_name":"Missing"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "profile_id and profile_name are required") {
		t.Fatalf("import profile validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/imports/remote-ygg/profiles/import-batch", strings.NewReader(`{"profiles":[]}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfiles(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "profiles cannot be empty") {
		t.Fatalf("import profiles empty validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestRemoteYggRoutesImportProfileAndBatchPersistExactProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := remote.New(db, nil)
	user := testutil.CreateUser(t, db, "remote-import@test.com", "Password123", "RemoteImport", false)

	req := httptest.NewRequest(http.MethodPost, "/v1/imports/remote-ygg/profiles/import", strings.NewReader(`{"profile_id":"remote_profile_one","profile_name":"RemoteOne"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"id\":\"remote_profile_one\",\"name\":\"RemoteOne\"}\n" {
		t.Fatalf("import profile exact body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	profile, err := db.Profiles.GetByID(req.Context(), "remote_profile_one")
	if err != nil || profile == nil || profile.UserID != user.ID || profile.Name != "RemoteOne" ||
		profile.TextureModel != "default" || profile.SkinHash == nil || profile.CapeHash != nil {
		t.Fatalf("single remote import should persist profile with skin only: profile=%#v err=%v", profile, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/imports/remote-ygg/profiles/import-batch", strings.NewReader(`{"profiles":[{"profile_id":"remote_batch_one","profile_name":"BatchOne"},{"profile_id":"","profile_name":"Broken"}]}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfiles(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success_count":1`) ||
		!strings.Contains(rec.Body.String(), `"failure_count":1`) ||
		!strings.Contains(rec.Body.String(), `"id":"remote_batch_one"`) ||
		!strings.Contains(rec.Body.String(), `"detail":"profile_id and profile_name are required"`) {
		t.Fatalf("batch remote import response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	profile, err = db.Profiles.GetByID(req.Context(), "remote_batch_one")
	if err != nil || profile == nil || profile.UserID != user.ID || profile.Name != "BatchOne" || profile.SkinHash == nil {
		t.Fatalf("batch remote import should persist successful profile: profile=%#v err=%v", profile, err)
	}
}
