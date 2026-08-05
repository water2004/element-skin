package imports_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/imports"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestRemoteYggPreviewProfilesAuthenticatesAndFiltersExactly(t *testing.T) {
	service := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		if req.Method != http.MethodPost || req.URL.String() != "https://93.184.216.34/api/authserver/authenticate" {
			t.Fatalf("authenticate request mismatch: method=%s url=%s", req.Method, req.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["username"] != "remote-user" || body["password"] != "remote-pass" || body["requestUser"] != true {
			t.Fatalf("credential payload mismatch: %#v", body)
		}
		agent := body["agent"].(map[string]any)
		if agent["name"] != "Minecraft" || agent["version"] != float64(1) {
			t.Fatalf("agent payload mismatch: %#v", agent)
		}
		return remoteYggJSONResponse(200, `{"availableProfiles":[{"id":"p1","name":"One"},{"id":"","name":"Broken"},{"id":"p2","name":"  Two  "}]}`)
	})}

	profiles, err := service.PreviewProfiles(context.Background(), remoteYggUserActor("preview-user"), "https://93.184.216.34/api/", " remote-user ", "remote-pass")
	if err != nil {
		t.Fatalf("PreviewProfiles returned error: %v", err)
	}
	if len(profiles) != 2 ||
		profiles[0] != (imports.RemoteYggProfile{ID: "p1", Name: "One"}) ||
		profiles[1] != (imports.RemoteYggProfile{ID: "p2", Name: "Two"}) {
		t.Fatalf("profiles mismatch: %#v", profiles)
	}
}

func TestRemoteYggPreviewProfilesRejectsInputAndRemoteErrorsExactly(t *testing.T) {
	service := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		return remoteYggJSONResponse(403, `{"errorMessage":"Invalid credentials"}`)
	})}

	actor := remoteYggUserActor("preview-user")
	if profiles, err := service.PreviewProfiles(context.Background(), actor, "", "u", "p"); profiles != nil || !remoteYggHTTPError(err, 400, "api_url, username and password are required") {
		t.Fatalf("missing api_url mismatch: profiles=%#v err=%#v", profiles, err)
	}
	if profiles, err := service.PreviewProfiles(context.Background(), actor, "not-a-url", "u", "p"); profiles != nil || !remoteYggHTTPError(err, 400, "invalid remote api url") {
		t.Fatalf("invalid api_url mismatch: profiles=%#v err=%#v", profiles, err)
	}
	if profiles, err := service.PreviewProfiles(context.Background(), actor, "https://93.184.216.34/api", "u", "p"); profiles != nil || !remoteYggHTTPError(err, 400, "远端认证失败: Invalid credentials") {
		t.Fatalf("remote auth error mismatch: profiles=%#v err=%#v", profiles, err)
	}
}

func TestRemoteYggServiceRejectsMissingPermissionsBeforeAnyOutboundRequest(t *testing.T) {
	requests := 0
	service := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		requests++
		return remoteYggJSONResponse(http.StatusOK, `{}`)
	})}
	ctx := context.Background()

	profiles, err := service.PreviewProfiles(ctx, remoteYggActor("denied-preview"), "https://93.184.216.34/api", "user", "password")
	if profiles != nil || !remoteYggHTTPError(err, http.StatusForbidden, "permission denied") || requests != 0 {
		t.Fatalf("denied preview result: profiles=%#v err=%#v requests=%d", profiles, err, requests)
	}

	result, err := service.ImportProfile(ctx, remoteYggActor("denied-import", "profile.create.owned"), "https://93.184.216.34/api", "profile-id", "ProfileName")
	if result != nil || !remoteYggHTTPError(err, http.StatusForbidden, "permission denied") || requests != 0 {
		t.Fatalf("denied single import result: result=%#v err=%#v requests=%d", result, err, requests)
	}

	result, err = service.ImportProfiles(ctx, remoteYggActor("denied-batch", "texture.create.owned"), "https://93.184.216.34/api", []map[string]string{{"profile_id": "profile-id", "profile_name": "ProfileName"}})
	if result != nil || !remoteYggHTTPError(err, http.StatusForbidden, "permission denied") || requests != 0 {
		t.Fatalf("denied batch import result: result=%#v err=%#v requests=%d", result, err, requests)
	}
}

func TestRemoteYggFetchTextureAssetsParsesTexturesExactly(t *testing.T) {
	service := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		if req.Method != http.MethodGet || req.URL.String() != "https://93.184.216.34/api/sessionserver/session/minecraft/profile/abc123" {
			t.Fatalf("profile request mismatch: method=%s url=%s", req.Method, req.URL.String())
		}
		texturePayload := base64.StdEncoding.EncodeToString([]byte(`{"textures":{"SKIN":{"url":"https://textures.example/skin.png","metadata":{"model":"slim"}},"CAPE":{"url":"https://textures.example/cape.png"}}}`))
		return remoteYggJSONResponse(200, `{"id":"abc123","name":"Remote","properties":[{"name":"textures","value":"`+texturePayload+`"}]}`)
	})}

	assets, err := service.FetchTextureAssets(context.Background(), "https://93.184.216.34/api", "abc-123")
	if err != nil {
		t.Fatalf("FetchTextureAssets returned error: %v", err)
	}
	want := []imports.TextureAsset{
		{URL: "https://textures.example/skin.png", Kind: "skin", Variant: "slim"},
		{URL: "https://textures.example/cape.png", Kind: "cape"},
	}
	if len(assets) != len(want) || assets[0] != want[0] || assets[1] != want[1] {
		t.Fatalf("texture assets mismatch: got=%#v want=%#v", assets, want)
	}
}

func TestRemoteYggFetchTextureAssetsHandlesMissingAndInvalidTexturesExactly(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "missing property", body: `{"id":"p","properties":[]}`, want: 0},
		{name: "invalid base64", body: `{"id":"p","properties":[{"name":"textures","value":"not-base64"}]}`, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
				return remoteYggJSONResponse(200, tc.body)
			})}
			assets, err := service.FetchTextureAssets(context.Background(), "https://93.184.216.34/api", "p")
			if err != nil || len(assets) != tc.want {
				t.Fatalf("assets mismatch: got=%#v err=%v want len=%d", assets, err, tc.want)
			}
		})
	}
}

func TestRemoteYggServiceRejectsBadProfileResponsesExactly(t *testing.T) {
	service := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		return remoteYggJSONResponse(200, `{`)
	})}
	if assets, err := service.FetchTextureAssets(context.Background(), "https://93.184.216.34/api", "p"); assets != nil || !remoteYggHTTPError(err, 400, "远端资料格式无效") {
		t.Fatalf("bad profile response mismatch: assets=%#v err=%#v", assets, err)
	}
}

func TestRemoteYggFetchTextureAssetsRejectsMissingInputsExactly(t *testing.T) {
	service := imports.RemoteYggService{}
	if assets, err := service.FetchTextureAssets(context.Background(), "", "profile-id"); assets != nil || !remoteYggHTTPError(err, 400, "api_url and profile_id are required") {
		t.Fatalf("missing api_url mismatch: assets=%#v err=%#v", assets, err)
	}
	if assets, err := service.FetchTextureAssets(context.Background(), "https://93.184.216.34/api", "  "); assets != nil || !remoteYggHTTPError(err, 400, "api_url and profile_id are required") {
		t.Fatalf("missing profile_id mismatch: assets=%#v err=%#v", assets, err)
	}
}

func TestRemoteYggImportProfileFetchesTexturesAndPersistsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "remote-ygg-import@test.com", "Password123", "RemoteYggImport", false)
	texturesDir := t.TempDir()
	texturePayload := remoteYggTexturesValue(t, `{"textures":{"SKIN":{"url":"https://93.184.216.34/import-skin.png","metadata":{"model":"slim"}},"CAPE":{"url":"https://93.184.216.34/import-cape.png"}}}`)
	service := imports.RemoteYggService{DB: db, TexturesDir: texturesDir, HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		switch req.URL.Path {
		case "/api/sessionserver/session/minecraft/profile/0123456789abcdef0123456789abcdef":
			if req.Method != http.MethodGet {
				t.Fatalf("import profile request method=%s want GET", req.Method)
			}
			return remoteYggJSONResponse(200, `{"id":"0123456789abcdef0123456789abcdef","name":"RemoteImport","properties":[{"name":"textures","value":"`+texturePayload+`"}]}`)
		case "/import-skin.png":
			return remoteYggPNGResponse(t, 64, 64, color.RGBA{R: 30, G: 160, B: 220, A: 255})
		case "/import-cape.png":
			return remoteYggPNGResponse(t, 64, 32, color.RGBA{R: 220, G: 160, B: 30, A: 255})
		default:
			t.Fatalf("unexpected import request: method=%s url=%s", req.Method, req.URL.String())
			return remoteYggJSONResponse(http.StatusNotFound, `{}`)
		}
	})}

	result, err := service.ImportProfile(ctx, remoteYggUserActor(user.ID), "https://93.184.216.34/api/", "0123456789abcdef0123456789abcdef", "RemoteImport")
	if err != nil {
		t.Fatalf("ImportProfile returned error: %v", err)
	}
	profile := result["profile"].(map[string]any)
	if len(result) != 1 ||
		profile["id"] != "0123456789abcdef0123456789abcdef" ||
		profile["name"] != "RemoteImport" ||
		profile["model"] != "slim" ||
		profile["skin_hash"] == nil ||
		profile["cape_hash"] == nil {
		t.Fatalf("import result mismatch: %#v", result)
	}
	stored, err := db.Profiles.GetByID(ctx, "0123456789abcdef0123456789abcdef")
	if err != nil || stored == nil || stored.UserID != user.ID || stored.Name != "RemoteImport" ||
		stored.TextureModel != "slim" || stored.SkinHash == nil || stored.CapeHash == nil ||
		*stored.SkinHash != *(profile["skin_hash"].(*string)) ||
		*stored.CapeHash != *(profile["cape_hash"].(*string)) {
		t.Fatalf("stored remote ygg profile mismatch: profile=%#v err=%v", stored, err)
	}
	assertRemoteYggTextureFileExists(t, texturesDir, stored.SkinHash)
	assertRemoteYggTextureFileExists(t, texturesDir, stored.CapeHash)
}

func TestRemoteYggImportProfilesReportsPartialFailuresExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "remote-ygg-batch@test.com", "Password123", "RemoteYggBatch", false)
	requests := map[string]int{}
	texturesDir := t.TempDir()
	service := imports.RemoteYggService{DB: db, TexturesDir: texturesDir, HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		if req.URL.Path == "/batch-skin.png" {
			return remoteYggPNGResponse(t, 64, 64, color.RGBA{R: 90, G: 40, B: 180, A: 255})
		}
		id := strings.TrimPrefix(req.URL.Path, "/api/sessionserver/session/minecraft/profile/")
		requests[id]++
		switch id {
		case "remote_batch_ok":
			texturePayload := remoteYggTexturesValue(t, `{"textures":{"SKIN":{"url":"https://93.184.216.34/batch-skin.png"}}}`)
			return remoteYggJSONResponse(200, `{"id":"remote_batch_ok","properties":[{"name":"textures","value":"`+texturePayload+`"}]}`)
		case "remote_batch_bad_name":
			return remoteYggJSONResponse(200, `{"id":"remote_batch_bad_name","properties":[]}`)
		case "remote_batch_fail":
			return remoteYggJSONResponse(503, `{}`)
		default:
			t.Fatalf("unexpected batch request path: %s", req.URL.Path)
			return remoteYggJSONResponse(500, `{}`)
		}
	})}

	result, err := service.ImportProfiles(ctx, remoteYggUserActor(user.ID), "https://93.184.216.34/api", []map[string]string{
		{"profile_id": "remote_batch_ok", "profile_name": "RemoteBatchOk"},
		{"profile_id": "", "profile_name": "RemoteMissingID"},
		{"profile_id": "remote_batch_fail", "profile_name": "RemoteBatchFail"},
		{"profile_id": "remote_batch_bad_name", "profile_name": "Bad Name!"},
	})
	if err != nil {
		t.Fatalf("ImportProfiles returned error: %v", err)
	}
	if result["success_count"] != 1 || result["failure_count"] != 3 {
		t.Fatalf("batch counts mismatch: %#v", result)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != "remote_batch_ok" || items[0]["name"] != "RemoteBatchOk" ||
		items[0]["model"] != "default" || items[0]["skin_hash"] == nil || items[0]["cape_hash"] != (*string)(nil) {
		t.Fatalf("batch success item mismatch: %#v", items)
	}
	failed := result["failed"].([]map[string]any)
	byID := map[string]string{}
	for _, item := range failed {
		byID[item["profile_id"].(string)] = item["detail"].(string)
	}
	if byID[""] != "profile_id and profile_name are required" ||
		byID["remote_batch_fail"] != "导入失败" ||
		byID["remote_batch_bad_name"] != "invalid profile name" {
		t.Fatalf("batch failed details mismatch: %#v", failed)
	}
	if requests["remote_batch_ok"] != 1 || requests["remote_batch_bad_name"] != 1 || requests["remote_batch_fail"] != 1 || len(requests) != 3 {
		t.Fatalf("remote fetch request counts mismatch: %#v", requests)
	}
	if stored, err := db.Profiles.GetByID(ctx, "remote_batch_ok"); err != nil || stored == nil || stored.Name != "RemoteBatchOk" {
		t.Fatalf("successful batch profile mismatch: profile=%#v err=%v", stored, err)
	} else {
		assertRemoteYggTextureFileExists(t, texturesDir, stored.SkinHash)
	}
	for _, id := range []string{"remote_batch_fail", "remote_batch_bad_name"} {
		if stored, err := db.Profiles.GetByID(ctx, id); err != nil || stored != nil {
			t.Fatalf("failed batch import persisted id=%s profile=%#v err=%v", id, stored, err)
		}
	}
}

func TestRemoteYggImportProfileRejectsMissingAPIURLWithoutPersisting(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "remote-ygg-missing-api@test.com", "Password123", "RemoteYggMissingAPI", false)
	service := imports.RemoteYggService{DB: db}

	result, err := service.ImportProfile(ctx, remoteYggUserActor(user.ID), "", "missing_remote_api_profile", "MissingRemoteAPI")
	if result != nil || !remoteYggHTTPError(err, 400, "api_url is required") {
		t.Fatalf("missing api_url import mismatch: result=%#v err=%#v", result, err)
	}
	if stored, getErr := db.Profiles.GetByID(ctx, "missing_remote_api_profile"); getErr != nil || stored != nil {
		t.Fatalf("missing api_url import persisted profile=%#v err=%v", stored, getErr)
	}
}

func TestRemoteYggImportProfileForwardsRemoteFailureWithoutPersisting(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "remote-ygg-fetch-fail@test.com", "Password123", "RemoteYggFetchFail", false)
	service := imports.RemoteYggService{DB: db, HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		return remoteYggJSONResponse(500, `{}`)
	})}

	result, err := service.ImportProfile(ctx, remoteYggUserActor(user.ID), "https://93.184.216.34/api", "remote_fetch_fail_profile", "RemoteFetchFail")
	if result != nil || !remoteYggHTTPError(err, 400, "远端认证失败: HTTP 500") {
		t.Fatalf("remote fetch failure import mismatch: result=%#v err=%#v", result, err)
	}
	if stored, getErr := db.Profiles.GetByID(ctx, "remote_fetch_fail_profile"); getErr != nil || stored != nil {
		t.Fatalf("remote fetch failure persisted profile=%#v err=%v", stored, getErr)
	}
}

func TestRemoteYggImportProfilesRejectsMissingAPIURLWithoutPersisting(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "remote-ygg-batch-missing-api@test.com", "Password123", "RemoteYggBatchMissingAPI", false)
	service := imports.RemoteYggService{DB: db}

	result, err := service.ImportProfiles(ctx, remoteYggUserActor(user.ID), " ", []map[string]string{
		{"profile_id": "remote_batch_missing_api", "profile_name": "RemoteBatchMissingAPI"},
	})
	if err != nil {
		t.Fatalf("ImportProfiles returned error: %v", err)
	}
	if result["success_count"] != 0 || result["failure_count"] != 1 {
		t.Fatalf("missing api_url batch counts mismatch: %#v", result)
	}
	failed := result["failed"].([]map[string]any)
	if len(failed) != 1 ||
		failed[0]["profile_id"] != "remote_batch_missing_api" ||
		failed[0]["profile_name"] != "RemoteBatchMissingAPI" ||
		failed[0]["detail"] != "导入失败" {
		t.Fatalf("missing api_url batch failure mismatch: %#v", failed)
	}
	if stored, getErr := db.Profiles.GetByID(ctx, "remote_batch_missing_api"); getErr != nil || stored != nil {
		t.Fatalf("missing api_url batch persisted profile=%#v err=%v", stored, getErr)
	}
}

func TestRemoteYggServiceHandlesNetworkAndFallbackErrorsExactly(t *testing.T) {
	networkService := imports.RemoteYggService{HTTPClient: remoteYggServiceErrorClient(errors.New("network down"))}
	actor := remoteYggUserActor("preview-user")
	if profiles, err := networkService.PreviewProfiles(context.Background(), actor, "https://93.184.216.34/api", "user", "pass"); profiles != nil || !remoteYggHTTPError(err, 400, "无法获取远端资料，请检查账号或稍后重试") {
		t.Fatalf("network auth error mismatch: profiles=%#v err=%#v", profiles, err)
	}

	errorFieldService := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		return remoteYggJSONResponse(401, `{"error":"Forbidden"}`)
	})}
	if profiles, err := errorFieldService.PreviewProfiles(context.Background(), actor, "https://93.184.216.34/api", "user", "pass"); profiles != nil || !remoteYggHTTPError(err, 400, "远端认证失败: Forbidden") {
		t.Fatalf("remote error field mismatch: profiles=%#v err=%#v", profiles, err)
	}

	statusFallbackService := imports.RemoteYggService{HTTPClient: remoteYggServiceClient(t, func(req *http.Request) *http.Response {
		return remoteYggJSONResponse(503, `{}`)
	})}
	if profiles, err := statusFallbackService.PreviewProfiles(context.Background(), actor, "https://93.184.216.34/api", "user", "pass"); profiles != nil || !remoteYggHTTPError(err, 400, "远端认证失败: HTTP 503") {
		t.Fatalf("remote status fallback mismatch: profiles=%#v err=%#v", profiles, err)
	}
}

func remoteYggServiceClient(t *testing.T, fn func(*http.Request) *http.Response) *http.Client {
	t.Helper()
	return &http.Client{Transport: remoteYggRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return fn(req), nil
	})}
}

func remoteYggServiceErrorClient(err error) *http.Client {
	return &http.Client{Transport: remoteYggRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, err
	})}
}

type remoteYggRoundTripFunc func(*http.Request) (*http.Response, error)

func (f remoteYggRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func remoteYggJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func remoteYggPNGResponse(t *testing.T, width, height int, c color.RGBA) *http.Response {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode remote ygg png: %v", err)
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{"image/png"}},
		ContentLength: int64(buf.Len()),
		Body:          io.NopCloser(bytes.NewReader(buf.Bytes())),
	}
}

func assertRemoteYggTextureFileExists(t *testing.T, dir string, hash *string) {
	t.Helper()
	if hash == nil {
		t.Fatal("texture hash should not be nil")
	}
	if _, err := os.Stat(filepath.Join(dir, *hash+".png")); err != nil {
		t.Fatalf("imported remote ygg texture file missing for hash %s: %v", *hash, err)
	}
}

func remoteYggHTTPError(err error, status int, detail string) bool {
	httpErr, ok := err.(util.HTTPError)
	return ok && httpErr.Status == status && httpErr.Detail == detail
}

func remoteYggTexturesValue(t *testing.T, payload string) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func remoteYggUserActor(userID string) permission.Actor {
	return remoteYggActor(userID, "profile.create.owned", "texture.create.owned")
}

func remoteYggActor(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}
