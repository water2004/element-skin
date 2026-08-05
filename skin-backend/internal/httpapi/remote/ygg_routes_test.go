package remote_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/remote"
	"element-skin/backend/internal/testutil"
)

func TestRemoteYggRoutesValidateAndReturnExactBodies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := remote.NewWithHTTPClient(cfg, db, nil, remoteYggTestClient(t))
	user := testutil.CreateUser(t, db, "remote-direct@test.com", "Password123", "RemoteDirect", false)

	req := httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/preview", strings.NewReader(`{"api_url":"https://93.184.216.34/ygg","username":"remote-user","password":"remote-password"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.GetProfiles(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[{\"id\":\"remote_profile_one\",\"name\":\"RemoteOne\"},{\"id\":\"remote_profile_two\",\"name\":\"RemoteTwo\"}]}\n" {
		t.Fatalf("get profiles exact body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/import", strings.NewReader(`{"profile_id":"","profile_name":"Missing"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "profile_id and profile_name are required") {
		t.Fatalf("import profile validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/import-batch", strings.NewReader(`{"profiles":[]}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfiles(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "profiles cannot be empty") {
		t.Fatalf("import profiles empty validation mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/preview", strings.NewReader(`{`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.GetProfiles(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("preview bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/import", strings.NewReader(`{`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("single import bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/import-batch", strings.NewReader(`{`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ImportProfiles(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("batch import bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestRemoteYggRoutesRejectMissingFineGrainedPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := remote.New(cfg, db, nil)
	user := testutil.CreateUser(t, db, "remote-permission@test.com", "Password123", "RemotePermission", false)

	cases := []struct {
		name       string
		exclude    string
		target     string
		body       string
		call       func(http.ResponseWriter, *http.Request)
		wantStatus int
	}{
		{"preview requires profile create", "profile.create.owned", "/v2/imports/remote-ygg/profiles/preview", `{"api_url":"https://93.184.216.34/ygg","username":"u","password":"p"}`, h.GetProfiles, http.StatusForbidden},
		{"single import requires profile create", "profile.create.owned", "/v2/imports/remote-ygg/profiles/import", `{"profile_id":"p","profile_name":"P"}`, h.ImportProfile, http.StatusForbidden},
		{"single import requires texture create", "texture.create.owned", "/v2/imports/remote-ygg/profiles/import", `{"profile_id":"p","profile_name":"P"}`, h.ImportProfile, http.StatusForbidden},
		{"batch import requires profile create", "profile.create.owned", "/v2/imports/remote-ygg/profiles/import-batch", `{"profiles":[{"profile_id":"p","profile_name":"P"}]}`, h.ImportProfiles, http.StatusForbidden},
		{"batch import requires texture create", "texture.create.owned", "/v2/imports/remote-ygg/profiles/import-batch", `{"profiles":[{"profile_id":"p","profile_name":"P"}]}`, h.ImportProfiles, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.target, strings.NewReader(tc.body))
			req = withUserActorWithoutPermission(req, user.ID, tc.exclude)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != tc.wantStatus || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
				t.Fatalf("permission denial mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRemoteYggRoutesImportProfileAndBatchPersistExactProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	h := remote.NewWithHTTPClient(cfg, db, nil, remoteYggTestClient(t))
	user := testutil.CreateUser(t, db, "remote-import@test.com", "Password123", "RemoteImport", false)

	req := httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/import", strings.NewReader(`{"api_url":"https://93.184.216.34/ygg","profile_id":"remote_profile_one","profile_name":"RemoteOne"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.ImportProfile(rec, req)
	if rec.Code != http.StatusCreated || rec.Body.String() != "{\"id\":\"remote_profile_one\",\"name\":\"RemoteOne\"}\n" {
		t.Fatalf("import profile exact body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	profile, err := db.Profiles.GetByID(req.Context(), "remote_profile_one")
	if err != nil || profile == nil || profile.UserID != user.ID || profile.Name != "RemoteOne" ||
		profile.TextureModel != "slim" || profile.SkinHash == nil || profile.CapeHash == nil {
		t.Fatalf("single remote import should persist profile with slim skin and cape: profile=%#v err=%v", profile, err)
	}
	assertTextureFileExists(t, cfg.TexturesDir, profile.SkinHash)
	assertTextureFileExists(t, cfg.TexturesDir, profile.CapeHash)

	req = httptest.NewRequest(http.MethodPost, "/v2/imports/remote-ygg/profiles/import-batch", strings.NewReader(`{"api_url":"https://93.184.216.34/ygg","profiles":[{"profile_id":"remote_batch_one","profile_name":"BatchOne"},{"profile_id":"","profile_name":"Broken"}]}`))
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
	assertTextureFileExists(t, cfg.TexturesDir, profile.SkinHash)
}

func remoteYggTestClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/ygg/authserver/authenticate":
			if req.Method != http.MethodPost {
				t.Fatalf("authenticate method=%s want POST", req.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode authenticate body: %v", err)
			}
			if body["username"] != "remote-user" || body["password"] != "remote-password" || body["requestUser"] != true {
				t.Fatalf("authenticate body mismatch: %#v", body)
			}
			agent, ok := body["agent"].(map[string]any)
			if !ok || agent["name"] != "Minecraft" || agent["version"] != float64(1) {
				t.Fatalf("authenticate agent mismatch: %#v", body["agent"])
			}
			return jsonResponse(http.StatusOK, `{"availableProfiles":[{"id":"remote_profile_one","name":"RemoteOne"},{"id":"remote_profile_two","name":"RemoteTwo"},{"id":"","name":"Broken"}]}`), nil
		case "/ygg/sessionserver/session/minecraft/profile/remote_profile_one",
			"/ygg/sessionserver/session/minecraft/profile/remote_batch_one":
			return jsonResponse(http.StatusOK, remoteProfileJSON(req.URL.Path)), nil
		case "/skin.png":
			return pngResponse(t, 64, 64, color.RGBA{R: 20, G: 180, B: 120, A: 255}), nil
		case "/cape.png":
			return pngResponse(t, 64, 32, color.RGBA{R: 180, G: 120, B: 20, A: 255}), nil
		default:
			t.Fatalf("unexpected remote ygg request: %s %s", req.Method, req.URL.String())
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})}
}

func remoteProfileJSON(path string) string {
	profileID := path[strings.LastIndex(path, "/")+1:]
	payload := base64.StdEncoding.EncodeToString([]byte(`{"textures":{"SKIN":{"url":"https://93.184.216.34/skin.png","metadata":{"model":"slim"}},"CAPE":{"url":"https://93.184.216.34/cape.png"}}}`))
	return `{"id":"` + profileID + `","name":"Remote","properties":[{"name":"textures","value":"` + payload + `"}]}`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func pngResponse(t *testing.T, width, height int, c color.RGBA) *http.Response {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{"image/png"}},
		ContentLength: int64(buf.Len()),
		Body:          io.NopCloser(bytes.NewReader(buf.Bytes())),
	}
}

func assertTextureFileExists(t *testing.T, dir string, hash *string) {
	t.Helper()
	if hash == nil {
		t.Fatal("texture hash should not be nil")
	}
	if _, err := os.Stat(filepath.Join(dir, *hash+".png")); err != nil {
		t.Fatalf("imported texture file missing for hash %s: %v", *hash, err)
	}
}
