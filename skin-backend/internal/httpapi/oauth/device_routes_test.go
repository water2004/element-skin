package oauth_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthDeviceCodeFlowRoutesIssueDelegatedBearer(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	user := testutil.CreateUser(t, db, "oauth-device-route@test.com", "Password123", "OAuthDeviceRoute", false)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, user.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Device route app",
		"redirect_uri": "https://device.example/callback",
		"client_type":  "public",
		"permissions":  []string{"account.read.self"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	clientID := decodeMap(t, createRes.Body.Bytes())["client_id"].(string)
	activateOAuthClient(t, db, clientID)

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", "account.read.self")
	deviceRes := doForm(t, router, "/oauth/device/code", form, "", "")
	if deviceRes.Code != http.StatusOK {
		t.Fatalf("device code status=%d body=%s", deviceRes.Code, deviceRes.Body.String())
	}
	device := decodeMap(t, deviceRes.Body.Bytes())
	deviceCode := device["device_code"].(string)
	userCode := device["user_code"].(string)
	if deviceCode == "" || userCode == "" || device["verification_uri"] != "https://skin.example/oauth/device" ||
		device["verification_uri_complete"] != "https://skin.example/oauth/device?user_code="+userCode ||
		device["scope"] != "account.read.self" {
		t.Fatalf("device response mismatch: %#v", device)
	}

	infoReq := httptest.NewRequest(http.MethodGet, "/oauth/device?user_code="+userCode, nil)
	infoReq.AddCookie(session)
	infoRec := httptest.NewRecorder()
	router.ServeHTTP(infoRec, infoReq)
	if infoRec.Code != http.StatusOK {
		t.Fatalf("device info status=%d body=%s", infoRec.Code, infoRec.Body.String())
	}
	info := decodeMap(t, infoRec.Body.Bytes())
	if info["status"] != "pending" || len(info["scopes"].([]any)) != 1 {
		t.Fatalf("device info mismatch: %#v", info)
	}

	pendingForm := url.Values{}
	pendingForm.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	pendingForm.Set("client_id", clientID)
	pendingForm.Set("device_code", deviceCode)
	pendingRes := doForm(t, router, "/oauth/token", pendingForm, "", "")
	if pendingRes.Code != http.StatusBadRequest ||
		pendingRes.Body.String() != "{\"error\":\"authorization_pending\"}\n" {
		t.Fatalf("pending device token error mismatch: status=%d body=%s", pendingRes.Code, pendingRes.Body.String())
	}

	decisionRes := doJSON(t, router, http.MethodPost, "/oauth/device", map[string]any{
		"user_code": userCode,
		"approve":   true,
	}, session, "")
	if decisionRes.Code != http.StatusNoContent || decisionRes.Body.Len() != 0 {
		t.Fatalf("device decision mismatch: status=%d body=%s", decisionRes.Code, decisionRes.Body.String())
	}
	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	tokenForm.Set("client_id", clientID)
	tokenForm.Set("device_code", deviceCode)
	tokenRes := doForm(t, router, "/oauth/token", tokenForm, "", "")
	if tokenRes.Code != http.StatusOK {
		t.Fatalf("device token status=%d body=%s", tokenRes.Code, tokenRes.Body.String())
	}
	token := decodeMap(t, tokenRes.Body.Bytes())
	access := token["access_token"].(string)
	if access == "" || token["refresh_token"].(string) == "" || token["scope"] != "account.read.self" {
		t.Fatalf("device token mismatch: %#v", token)
	}
	meRes := doJSON(t, router, http.MethodGet, "/v2/users/me", nil, nil, access)
	if meRes.Code != http.StatusOK {
		t.Fatalf("device bearer me status=%d body=%s", meRes.Code, meRes.Body.String())
	}
	me := decodeMap(t, meRes.Body.Bytes())
	if me["id"] != user.ID {
		t.Fatalf("device bearer user mismatch: %#v", me)
	}
}
