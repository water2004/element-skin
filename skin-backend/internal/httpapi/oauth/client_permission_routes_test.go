package oauth_test

import (
	"net/http"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthClientPermissionRoutesManageClientSubjectExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	admin := testutil.CreateUser(t, db, "oauth-client-permission@test.com", "Password123", "OAuthClientPermission", true, true)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, admin.ID)
	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Permission route app",
		"redirect_uri": "https://client.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"minecraft_profile.read.public"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	clientID := decodeMap(t, createRes.Body.Bytes())["client_id"].(string)
	grantRes := doJSON(t, router, http.MethodPut, "/v2/oauth/apps/"+clientID+"/permissions/minecraft_session.hasjoined.server", map[string]any{
		"effect": "allow",
	}, session, "")
	if grantRes.Code != http.StatusNoContent || grantRes.Body.Len() != 0 {
		t.Fatalf("grant client permission mismatch: status=%d body=%s", grantRes.Code, grantRes.Body.String())
	}
	permissionsRes := doJSON(t, router, http.MethodGet, "/v2/oauth/apps/"+clientID+"/permissions", nil, session, "")
	if permissionsRes.Code != http.StatusOK {
		t.Fatalf("client permissions status=%d body=%s", permissionsRes.Code, permissionsRes.Body.String())
	}
	body := decodeMap(t, permissionsRes.Body.Bytes())
	if body["subject_id"] != permissiondb.SubjectIDForClient(clientID) {
		t.Fatalf("client permission subject mismatch: %#v", body)
	}
	effective := stringSet(body["effective_permissions"].([]any))
	if !effective["minecraft_session.hasjoined.server"] {
		t.Fatalf("effective client permissions missing grant: %#v", effective)
	}
	overrides := body["overrides"].([]any)
	if len(overrides) != 1 || overrides[0].(map[string]any)["permission_code"] != "minecraft_session.hasjoined.server" ||
		overrides[0].(map[string]any)["effect"] != "allow" {
		t.Fatalf("client permission overrides mismatch: %#v", overrides)
	}
	clearRes := doJSON(t, router, http.MethodDelete, "/v2/oauth/apps/"+clientID+"/permissions/minecraft_session.hasjoined.server", nil, session, "")
	if clearRes.Code != http.StatusNoContent || clearRes.Body.Len() != 0 {
		t.Fatalf("clear client permission mismatch: status=%d body=%s", clearRes.Code, clearRes.Body.String())
	}
}
