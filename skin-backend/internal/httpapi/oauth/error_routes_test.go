package oauth_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"element-skin/backend/internal/httpapi"
	oauthapi "element-skin/backend/internal/httpapi/oauth"
	"element-skin/backend/internal/redisstore"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthRoutesRejectMalformedInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	adminUser := testutil.CreateUser(t, db, "oauth-route-errors-admin@test.com", "Password123", "OAuthRouteErrorsAdmin", true, true)
	user := testutil.CreateUser(t, db, "oauth-route-errors-user@test.com", "Password123", "OAuthRouteErrorsUser", false)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	adminSession := webCookie(t, cfg.JWTSecret, adminUser.ID)
	userSession := webCookie(t, cfg.JWTSecret, user.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Malformed route app",
		"redirect_uri": "https://malformed.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self"},
	}, userSession, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create route error app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	clientID := decodeMap(t, createRes.Body.Bytes())["client_id"].(string)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		cookie *http.Cookie
	}{
		{name: "create app", method: http.MethodPost, path: "/v2/oauth/apps", cookie: userSession},
		{name: "update app", method: http.MethodPatch, path: "/v2/oauth/apps/" + clientID, cookie: userSession},
		{name: "review app", method: http.MethodPatch, path: "/v2/admin/oauth/apps/" + clientID + "/review", cookie: adminSession},
		{name: "permission override", method: http.MethodPut, path: "/v2/oauth/apps/" + clientID + "/permissions/account.read.self", cookie: adminSession},
		{name: "authorize", method: http.MethodPost, path: "/oauth/authorize", cookie: userSession},
		{name: "device decision", method: http.MethodPost, path: "/oauth/device", cookie: userSession},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRaw(t, router, tc.method, tc.path, "{bad", "application/json", tc.cookie, "")
			if res.Code != http.StatusBadRequest || res.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n" {
				t.Fatalf("%s invalid json mismatch: status=%d body=%s", tc.name, res.Code, res.Body.String())
			}
		})
	}

	reviewRes := doJSON(t, router, http.MethodPatch, "/v2/admin/oauth/apps/"+clientID+"/review", map[string]any{"status": "pending"}, adminSession, "")
	if reviewRes.Code != http.StatusBadRequest || reviewRes.Body.String() != "{\"error\":{\"object\":\"status\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid review status mismatch: status=%d body=%s", reviewRes.Code, reviewRes.Body.String())
	}
	rejectWithoutReason := doJSON(t, router, http.MethodPatch, "/v2/admin/oauth/apps/"+clientID+"/review", map[string]any{"status": "rejected"}, adminSession, "")
	if rejectWithoutReason.Code != http.StatusBadRequest || rejectWithoutReason.Body.String() != "{\"error\":{\"object\":\"audit_reason\",\"operation\":\"validate\",\"reason\":\"required\"}}\n" {
		t.Fatalf("reject without reason mismatch: status=%d body=%s", rejectWithoutReason.Code, rejectWithoutReason.Body.String())
	}
	grantRes := doJSON(t, router, http.MethodPut, "/v2/oauth/apps/"+clientID+"/permissions/nope.nope.nope", map[string]any{"effect": "allow"}, adminSession, "")
	if grantRes.Code != http.StatusBadRequest || grantRes.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("invalid client permission mismatch: status=%d body=%s", grantRes.Code, grantRes.Body.String())
	}
	clearRes := doJSON(t, router, http.MethodDelete, "/v2/oauth/apps/"+clientID+"/permissions/nope.nope.nope", nil, adminSession, "")
	if clearRes.Code != http.StatusBadRequest || clearRes.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("clear invalid client permission mismatch: status=%d body=%s", clearRes.Code, clearRes.Body.String())
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "device code", path: "/oauth/device/code"},
		{name: "token", path: "/oauth/token"},
		{name: "revoke", path: "/oauth/revoke"},
		{name: "introspect", path: "/oauth/introspect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRaw(t, router, http.MethodPost, tc.path, "%zz", "application/x-www-form-urlencoded", adminSession, "")
			if res.Code != http.StatusBadRequest || res.Body.String() != "{\"error\":\"invalid_request\"}\n" {
				t.Fatalf("%s invalid form mismatch: status=%d body=%s", tc.name, res.Code, res.Body.String())
			}
		})
	}

	unsupportedGrant := url.Values{}
	unsupportedGrant.Set("grant_type", "password")
	unsupportedGrantRes := doForm(t, router, "/oauth/token", unsupportedGrant, "", "")
	if unsupportedGrantRes.Code != http.StatusBadRequest ||
		unsupportedGrantRes.Body.String() != "{\"error\":\"unsupported_grant_type\"}\n" {
		t.Fatalf("unsupported grant error mismatch: status=%d body=%s", unsupportedGrantRes.Code, unsupportedGrantRes.Body.String())
	}

	invalidClient := url.Values{}
	invalidClient.Set("grant_type", "client_credentials")
	invalidClient.Set("client_id", "missing-client")
	invalidClientRes := doForm(t, router, "/oauth/token", invalidClient, "", "")
	if invalidClientRes.Code != http.StatusUnauthorized ||
		invalidClientRes.Header().Get("WWW-Authenticate") != `Basic realm="oauth"` ||
		invalidClientRes.Body.String() != "{\"error\":\"invalid_client\"}\n" {
		t.Fatalf("invalid client error mismatch: status=%d header=%q body=%s", invalidClientRes.Code, invalidClientRes.Header().Get("WWW-Authenticate"), invalidClientRes.Body.String())
	}
}

func TestOAuthHandlerForwardsServiceErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	cfg.APIURL = ""
	cfg.SiteURL = "https://skin.example"
	handler := oauthapi.New(cfg, db, redisstore.NewMemoryStore(), nil)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		status int
		body   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "list apps", method: http.MethodGet, path: "/v2/oauth/apps", status: http.StatusForbidden, body: "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n", call: handler.ListApps},
		{name: "admin list apps", method: http.MethodGet, path: "/v2/admin/oauth/apps", status: http.StatusForbidden, body: "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n", call: handler.ListAdminApps},
		{name: "submit review", method: http.MethodPost, path: "/v2/oauth/apps/missing/review-submission", status: http.StatusNotFound, body: "{\"error\":{\"object\":\"oauth_client\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.SubmitAppReview(rec, req)
		}},
		{name: "rotate secret", method: http.MethodPost, path: "/v2/oauth/apps/missing/secret", status: http.StatusNotFound, body: "{\"error\":{\"object\":\"oauth_client\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.RotateSecret(rec, req)
		}},
		{name: "delete app", method: http.MethodDelete, path: "/v2/oauth/apps/missing", status: http.StatusNotFound, body: "{\"error\":{\"object\":\"oauth_client\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.DeleteApp(rec, req)
		}},
		{name: "client permissions", method: http.MethodGet, path: "/v2/oauth/apps/missing/permissions", status: http.StatusForbidden, body: "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n", call: func(rec http.ResponseWriter, req *http.Request) {
			req.SetPathValue("client_id", "missing")
			handler.ClientPermissions(rec, req)
		}},
		{name: "list grants", method: http.MethodGet, path: "/v2/oauth/grants", status: http.StatusForbidden, body: "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n", call: handler.ListGrants},
		{name: "authorize info", method: http.MethodGet, path: "/oauth/authorize", status: http.StatusBadRequest, body: "{\"error\":{\"object\":\"response_type\",\"operation\":\"validate\",\"reason\":\"unsupported\"}}\n", call: handler.AuthorizeInfo},
		{name: "device info", method: http.MethodGet, path: "/oauth/device?user_code=missing", status: http.StatusForbidden, body: "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n", call: handler.DeviceInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != tc.status || rec.Body.String() != tc.body {
				t.Fatalf("%s service error mismatch: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	handler.ProtectedResourceMetadata(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("site-url protected metadata status=%d body=%s", rec.Code, rec.Body.String())
	}
	protected := decodeMap(t, rec.Body.Bytes())
	if protected["resource"] != "https://skin.example/v2" ||
		protected["authorization_servers"].([]any)[0] != "https://skin.example" {
		t.Fatalf("site-url protected metadata mismatch: %#v", protected)
	}
}
