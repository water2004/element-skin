package integration_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"errors"

	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestYggdrasilAuthenticateJoinAndProfile(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	user := testutil.CreateUser(t, db, "ygg@test.com", "YggPassword123", "YggUser", false)
	skin := "my_skin_hash"
	cape := "my_cape_hash"
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_profile_id", "YggPlayer")
	if err := db.Profiles.UpdateSkin(context.Background(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(context.Background(), profile.ID, &cape); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateModel(context.Background(), profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}

	auth := doJSON(t, h, "POST", "/authserver/authenticate", map[string]any{
		"username": user.Email, "password": "YggPassword123", "requestUser": true,
	})
	if auth.Code != 200 {
		t.Fatalf("auth status=%d body=%s", auth.Code, auth.Body.String())
	}
	authBody := parseJSON(t, auth)
	if authBody["selectedProfile"].(map[string]any)["id"] != profile.ID {
		t.Fatalf("unexpected auth body: %#v", authBody)
	}
	userBody := authBody["user"].(map[string]any)
	if userBody["id"] != user.ID {
		t.Fatalf("requestUser should include user id: %#v", authBody)
	}
	propsUser := userBody["properties"].([]any)
	if len(propsUser) != 1 || propsUser[0].(map[string]any)["name"] != "preferredLanguage" || propsUser[0].(map[string]any)["value"] != "zh_CN" {
		t.Fatalf("requestUser should include preferredLanguage property: %#v", propsUser)
	}
	accessToken := authBody["accessToken"].(string)

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "refresh", path: "/authserver/refresh"},
		{name: "validate", path: "/authserver/validate"},
		{name: "invalidate", path: "/authserver/invalidate"},
	} {
		resp := doRawJSON(t, h, "POST", tc.path, `{"accessToken":`, nil)
		if resp.Code != 400 {
			t.Fatalf("%s malformed JSON should be 400, got %d body=%s", tc.name, resp.Code, resp.Body.String())
		}
	}
	if token, err := redis.GetYggToken(context.Background(), accessToken); err != nil || token.UserID != user.ID {
		t.Fatalf("malformed invalidate request must not delete the valid redis token: %#v err=%v", token, err)
	}
	if token, err := db.Tokens.Get(context.Background(), accessToken); err != nil || token != nil {
		t.Fatalf("ygg authenticate must not persist token in database: %#v err=%v", token, err)
	}

	refresh := doJSON(t, h, "POST", "/authserver/refresh", map[string]any{
		"accessToken": accessToken, "clientToken": authBody["clientToken"], "requestUser": true,
	})
	if refresh.Code != 200 {
		t.Fatalf("ygg refresh status=%d body=%s", refresh.Code, refresh.Body.String())
	}
	refreshBody := parseJSON(t, refresh)
	newAccessToken := refreshBody["accessToken"].(string)
	if newAccessToken == "" || newAccessToken == accessToken {
		t.Fatalf("refresh should rotate access token: %#v", refreshBody)
	}
	if _, err := redis.GetYggToken(context.Background(), accessToken); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old ygg access token should be deleted from redis after refresh, got %v", err)
	}
	if newToken, err := redis.GetYggToken(context.Background(), newAccessToken); err != nil || newToken.UserID != user.ID {
		t.Fatalf("new ygg access token should be stored in redis: %#v err=%v", newToken, err)
	}
	if newToken, err := db.Tokens.Get(context.Background(), newAccessToken); err != nil || newToken != nil {
		t.Fatalf("new ygg access token must not be persisted in database: %#v err=%v", newToken, err)
	}
	if refreshBody["selectedProfile"].(map[string]any)["id"] != profile.ID || refreshBody["user"].(map[string]any)["id"] != user.ID {
		t.Fatalf("refresh should include selected profile and requestUser data: %#v", refreshBody)
	}
	accessToken = newAccessToken

	join := doJSON(t, h, "POST", "/sessionserver/session/minecraft/join", map[string]any{
		"accessToken": accessToken, "selectedProfile": profile.ID, "serverId": "server_1",
	})
	if join.Code != 204 {
		t.Fatalf("join status=%d body=%s", join.Code, join.Body.String())
	}
	if session, err := redis.GetYggSession(context.Background(), "server_1"); err != nil || session.AccessToken != accessToken {
		t.Fatalf("join should store session in redis: %#v err=%v", session, err)
	}
	if session, err := db.Tokens.GetSession(context.Background(), "server_1"); err != nil || session != nil {
		t.Fatalf("join must not persist session in database: %#v err=%v", session, err)
	}
	hasJoined := doJSON(t, h, "GET", "/sessionserver/session/minecraft/hasJoined?username=YggPlayer&serverId=server_1", nil)
	if hasJoined.Code != 200 {
		t.Fatalf("hasJoined status=%d body=%s", hasJoined.Code, hasJoined.Body.String())
	}
	if parseJSON(t, hasJoined)["id"] != profile.ID {
		t.Fatal("hasJoined returned wrong profile")
	}

	profileResp := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/"+profile.ID, nil)
	if profileResp.Code != 200 {
		t.Fatalf("profile status=%d body=%s", profileResp.Code, profileResp.Body.String())
	}
	pbody := parseJSON(t, profileResp)
	props := pbody["properties"].([]any)
	texturesValue := props[0].(map[string]any)["value"].(string)
	decoded, err := base64.StdEncoding.DecodeString(texturesValue)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(decoded, []byte("my_skin_hash.png")) {
		t.Fatalf("textures payload missing skin hash: %s", decoded)
	}
	var texturesPayload map[string]any
	if err := json.Unmarshal(decoded, &texturesPayload); err != nil {
		t.Fatal(err)
	}
	textures := texturesPayload["textures"].(map[string]any)
	skinPayload := textures["SKIN"].(map[string]any)
	if skinPayload["metadata"].(map[string]any)["model"] != "slim" {
		t.Fatalf("slim skin should include model metadata: %#v", skinPayload)
	}
	if !strings.Contains(textures["CAPE"].(map[string]any)["url"].(string), "my_cape_hash.png") {
		t.Fatalf("cape url missing from textures payload: %#v", textures)
	}
	if len(props) != 2 ||
		props[1].(map[string]any)["name"] != "uploadableTextures" ||
		props[1].(map[string]any)["value"] != "skin,cape" {
		t.Fatalf("profile properties should include exact uploadableTextures property: %#v", props)
	}

	defaultProfile := testutil.CreateProfile(t, db, user.ID, "default_profile_id", "DefaultModel")
	defaultSkin := "default_skin_hash"
	if err := db.Profiles.UpdateSkin(context.Background(), defaultProfile.ID, &defaultSkin); err != nil {
		t.Fatal(err)
	}
	defaultResp := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/"+defaultProfile.ID, nil)
	if defaultResp.Code != 200 {
		t.Fatalf("default profile status=%d body=%s", defaultResp.Code, defaultResp.Body.String())
	}
	defaultProps := parseJSON(t, defaultResp)["properties"].([]any)
	defaultDecoded, err := base64.StdEncoding.DecodeString(defaultProps[0].(map[string]any)["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var defaultPayload map[string]any
	if err := json.Unmarshal(defaultDecoded, &defaultPayload); err != nil {
		t.Fatal(err)
	}
	defaultTextures := defaultPayload["textures"].(map[string]any)
	if _, ok := defaultTextures["SKIN"].(map[string]any)["metadata"]; ok {
		t.Fatalf("default skin should not include metadata: %#v", defaultTextures["SKIN"])
	}
	if _, ok := defaultTextures["CAPE"]; ok {
		t.Fatalf("profile without cape should not include CAPE: %#v", defaultTextures)
	}
	signedResp := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/"+profile.ID+"?unsigned=false", nil)
	if signedResp.Code != 200 {
		t.Fatalf("signed profile status=%d body=%s", signedResp.Code, signedResp.Body.String())
	}
	signedProps := parseJSON(t, signedResp)["properties"].([]any)
	signedTexture := signedProps[0].(map[string]any)
	meta := doJSON(t, h, "GET", "/", nil)
	metaBody := parseJSON(t, meta)
	wantPublicKey, err := os.ReadFile(testutil.TestConfig().PublicKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if metaBody["signaturePublickey"] != string(wantPublicKey) {
		t.Fatalf("metadata public key mismatch: got=%q want=%q", metaBody["signaturePublickey"], string(wantPublicKey))
	}
	verifyYggPropertySignature(t, string(wantPublicKey), signedTexture["value"].(string), signedTexture["signature"].(string))
	skinDomains := metaBody["skinDomains"].([]any)
	if len(skinDomains) != 2 || skinDomains[0] != "textures.minecraft.net" || skinDomains[1] != "test" {
		t.Fatalf("metadata skinDomains mismatch: %#v", metaBody)
	}
	if metaBody["meta"].(map[string]any)["serverName"] != "皮肤站" {
		t.Fatalf("metadata should include site serverName: %#v", metaBody)
	}
}

func verifyYggPropertySignature(t *testing.T, publicKeyPEM, value, signature string) {
	t.Helper()
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		t.Fatal("metadata public key is not PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		t.Fatal("metadata public key is not RSA")
	}
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha1.Sum([]byte(value))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, digest[:], rawSignature); err != nil {
		t.Fatalf("signed textures property signature mismatch: %v", err)
	}
}

func TestAdminAccessUsesDatabaseState(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	admin := testutil.CreateUser(t, db, "admin@test.com", "Password123", "Admin", true)
	normal := testutil.CreateUser(t, db, "normal@test.com", "Password123", "Normal", false)
	token, err := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	adminCookie := &http.Cookie{Name: "access_token", Value: token}

	users := doJSON(t, h, "GET", "/admin/users", nil, adminCookie)
	if users.Code != 200 {
		t.Fatalf("admin users status=%d body=%s", users.Code, users.Body.String())
	}
	if _, err := db.Users.ToggleAdmin(context.Background(), admin.ID); err != nil {
		t.Fatal(err)
	}
	cached := doJSON(t, h, "GET", "/admin/users", nil, adminCookie)
	if cached.Code != 200 {
		t.Fatalf("short auth cache should allow until invalidated, got %d", cached.Code)
	}
	if err := redis.InvalidateAuthUser(context.Background(), admin.ID); err != nil {
		t.Fatal(err)
	}
	demoted := doJSON(t, h, "GET", "/admin/users", nil, adminCookie)
	if demoted.Code != 403 {
		t.Fatalf("demoted admin should be forbidden after auth cache invalidation, got %d", demoted.Code)
	}

	normalToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, normal.ID, false, time.Hour)
	forbidden := doJSON(t, h, "GET", "/admin/users", nil, &http.Cookie{Name: "access_token", Value: normalToken})
	if forbidden.Code != 403 {
		t.Fatalf("normal user should be forbidden, got %d", forbidden.Code)
	}
}

func TestYggdrasilProfileNameLoginAndLookupMiss(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "multi@test.com", "YggPassword123", "MultiUser", false)
	p1 := testutil.CreateProfile(t, db, user.ID, "profile_1", "PlayerOne")
	_ = testutil.CreateProfile(t, db, user.ID, "profile_2", "PlayerTwo")

	byName := doJSON(t, h, "POST", "/authserver/authenticate", map[string]any{"username": "PlayerOne", "password": "YggPassword123"})
	if byName.Code != 200 {
		t.Fatalf("profile-name auth status=%d body=%s", byName.Code, byName.Body.String())
	}
	body := parseJSON(t, byName)
	if body["selectedProfile"].(map[string]any)["id"] != p1.ID {
		t.Fatalf("wrong selected profile: %#v", body)
	}
	if len(body["availableProfiles"].([]any)) != 1 {
		t.Fatalf("profile-name login should return one profile: %#v", body)
	}

	byEmail := doJSON(t, h, "POST", "/authserver/authenticate", map[string]any{"username": user.Email, "password": "YggPassword123"})
	if byEmail.Code != 200 {
		t.Fatalf("email auth status=%d body=%s", byEmail.Code, byEmail.Body.String())
	}
	emailBody := parseJSON(t, byEmail)
	if _, ok := emailBody["selectedProfile"]; ok {
		t.Fatalf("email login with multiple profiles should not select one: %#v", emailBody)
	}
	if len(emailBody["availableProfiles"].([]any)) != 2 {
		t.Fatalf("email login should return two profiles: %#v", emailBody)
	}

	miss := doJSON(t, h, "GET", "/api/profiles/minecraft/NoSuchPlayer", nil)
	if miss.Code != 204 {
		t.Fatalf("lookup miss should be 204, got %d", miss.Code)
	}
}

func TestTextureUploadAndYggdrasilTextureRoutes(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	user := testutil.CreateUser(t, db, "upload@test.com", "Password123", "Uploader", false)
	profile := testutil.CreateProfile(t, db, user.ID, "upload_profile", "UploadPlayer")
	access, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	cookie := &http.Cookie{Name: "access_token", Value: access}

	upload := doMultipart(t, h, "POST", "/me/textures", map[string]string{
		"texture_type": "skin",
		"note":         "API Upload",
		"is_public":    "true",
		"model":        "default",
	}, "file", "skin.png", pngTexture(t, 64, 64), cookie)
	if upload.Code != 200 {
		t.Fatalf("site texture upload status=%d body=%s", upload.Code, upload.Body.String())
	}
	hash := parseJSON(t, upload)["hash"].(string)
	info, _ := db.Textures.GetInfo(context.Background(), user.ID, hash, "skin")
	if info == nil || info["note"] != "API Upload" || info["is_public"].(int) != 1 {
		t.Fatalf("uploaded texture not persisted: %#v", info)
	}

	oversized := doMultipart(t, h, "POST", "/me/textures", map[string]string{
		"texture_type": "skin",
	}, "file", "too-large.png", bytes.Repeat([]byte("x"), 16<<20+1), cookie)
	if oversized.Code != 400 || !strings.Contains(oversized.Body.String(), "File too large") {
		t.Fatalf("oversized texture upload should be rejected, got %d body=%s", oversized.Code, oversized.Body.String())
	}

	missingBearer := doMultipart(t, h, "PUT", "/api/user/profile/"+profile.ID+"/skin", nil, "file", "skin.png", pngTexture(t, 64, 64))
	if missingBearer.Code != 401 {
		t.Fatalf("missing bearer should be 401, got %d", missingBearer.Code)
	}

	token := model.Token{AccessToken: "texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}
	if stored, err := db.Tokens.Get(context.Background(), token.AccessToken); err != nil || stored != nil {
		t.Fatalf("ygg texture token seed must be redis-only: %#v err=%v", stored, err)
	}
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("model", "slim")
	part, _ := mw.CreateFormFile("file", "skin.png")
	_, _ = part.Write(pngTexture(t, 64, 64))
	_ = mw.Close()
	yggReq := httptest.NewRequest("PUT", "/api/user/profile/"+profile.ID+"/skin", &b)
	yggReq.Header.Set("Content-Type", mw.FormDataContentType())
	yggReq.Header.Set("Authorization", "Bearer texture_token")
	yggRR := httptest.NewRecorder()
	h.ServeHTTP(yggRR, yggReq)
	if yggRR.Code != 200 {
		t.Fatalf("ygg texture upload status=%d body=%s", yggRR.Code, yggRR.Body.String())
	}
	p, _ := db.Profiles.GetByID(context.Background(), profile.ID)
	if p.SkinHash == nil || p.TextureModel != "slim" {
		t.Fatalf("ygg upload did not apply skin/model: %#v", p)
	}

	delReq := httptest.NewRequest("DELETE", "/api/user/profile/"+profile.ID+"/skin", nil)
	delReq.Header.Set("Authorization", "Bearer texture_token")
	delRR := httptest.NewRecorder()
	h.ServeHTTP(delRR, delReq)
	if delRR.Code != 204 {
		t.Fatalf("ygg delete status=%d body=%s", delRR.Code, delRR.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profile.ID)
	if p.SkinHash != nil {
		t.Fatalf("skin should be cleared: %#v", p)
	}
}

func TestYggdrasilFallbackRoutes(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	ctx := context.Background()

	var seen []string
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/session/minecraft/hasJoined":
			if r.URL.Query().Get("username") != "RemotePlayer" || r.URL.Query().Get("serverId") != "remote-server" || r.URL.Query().Get("ip") != "127.0.0.1" {
				t.Fatalf("unexpected hasJoined query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"remoteid","name":"RemotePlayer"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/session/minecraft/profile/remoteid":
			if r.URL.Query().Get("unsigned") != "false" {
				t.Fatalf("expected unsigned=false, got %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"remoteid","name":"RemotePlayer","properties":[{"name":"textures","value":"v"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/profiles/minecraft/RemoteAccount":
			_, _ = w.Write([]byte(`{"id":"accountid","name":"RemoteAccount"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/profiles/minecraft":
			var names []string
			if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
				t.Fatalf("decode fallback bulk body: %v", err)
			}
			if len(names) != 1 || names[0] != "RemoteBulk" {
				t.Fatalf("unexpected fallback bulk names: %#v", names)
			}
			_, _ = w.Write([]byte(`[{"id":"bulkid","name":"RemoteBulk"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/minecraft/profile/lookup/name/RemoteServices":
			_, _ = w.Write([]byte(`{"id":"servicesid","name":"RemoteServices"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fallbackServer.Close()

	if err := db.Fallbacks.SaveEndpoints(ctx, []fallback.Endpoint{{
		Priority: 1, SessionURL: fallbackServer.URL, AccountURL: fallbackServer.URL, ServicesURL: fallbackServer.URL,
		CacheTTL: 60, EnableProfile: true, EnableHasJoined: true,
	}}); err != nil {
		t.Fatal(err)
	}

	hasJoined := doJSON(t, h, "GET", "/sessionserver/session/minecraft/hasJoined?username=RemotePlayer&serverId=remote-server&ip=127.0.0.1", nil)
	if hasJoined.Code != 200 || !strings.Contains(hasJoined.Body.String(), `"RemotePlayer"`) {
		t.Fatalf("hasJoined fallback failed: %d %s", hasJoined.Code, hasJoined.Body.String())
	}

	profile := doJSON(t, h, "GET", "/sessionserver/session/minecraft/profile/remoteid?unsigned=false", nil)
	if profile.Code != 200 || !strings.Contains(profile.Body.String(), `"properties"`) {
		t.Fatalf("profile fallback failed: %d %s", profile.Code, profile.Body.String())
	}

	account := doJSON(t, h, "GET", "/api/profiles/minecraft/RemoteAccount", nil)
	if account.Code != 200 || parseJSON(t, account)["id"] != "accountid" {
		t.Fatalf("account lookup fallback failed: %d %s", account.Code, account.Body.String())
	}

	userAccount := doJSON(t, h, "GET", "/users/profiles/minecraft/RemoteAccount", nil)
	if userAccount.Code != 200 || parseJSON(t, userAccount)["id"] != "accountid" {
		t.Fatalf("users profile fallback failed: %d %s", userAccount.Code, userAccount.Body.String())
	}

	bulk := doJSON(t, h, "POST", "/api/profiles/minecraft", []string{"RemoteBulk"})
	var bulkBody []map[string]any
	if err := json.Unmarshal(bulk.Body.Bytes(), &bulkBody); err != nil {
		t.Fatalf("decode bulk body: %v body=%s", err, bulk.Body.String())
	}
	if bulk.Code != 200 || len(bulkBody) != 1 || bulkBody[0]["id"] != "bulkid" {
		t.Fatalf("bulk fallback failed: %d %s", bulk.Code, bulk.Body.String())
	}

	services := doJSON(t, h, "GET", "/minecraft/profile/lookup/name/RemoteServices", nil)
	if services.Code != 200 || parseJSON(t, services)["id"] != "servicesid" {
		t.Fatalf("services lookup fallback failed: %d %s", services.Code, services.Body.String())
	}

	if len(seen) < 6 {
		t.Fatalf("expected fallback server to receive all lookup requests, saw %#v", seen)
	}
}
