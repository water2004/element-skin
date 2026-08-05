package oauth_test

import (
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthAppManagementRoutesCoverReviewSecretListsAndDelete(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	owner := testutil.CreateUser(t, db, "oauth-app-owner@test.com", "Password123", "OAuthAppOwner", false)
	admin := testutil.CreateUser(t, db, "oauth-app-admin@test.com", "Password123", "OAuthAppAdmin", true, true)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	ownerSession := webCookie(t, cfg.JWTSecret, owner.ID)
	adminSession := webCookie(t, cfg.JWTSecret, admin.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Route managed app",
		"description":  "Route app description",
		"redirect_uri": "https://route.example/callback",
		"website_url":  "https://route.example",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self", "account.update.self"},
	}, ownerSession, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	app := decodeMap(t, createRes.Body.Bytes())
	clientID := app["client_id"].(string)
	firstSecret := app["client_secret"].(string)
	if clientID == "" || firstSecret == "" || app["status"] != "pending" {
		t.Fatalf("created app mismatch: %#v", app)
	}

	listRes := doJSON(t, router, http.MethodGet, "/v2/oauth/apps?limit=5", nil, ownerSession, "")
	if listRes.Code != http.StatusOK {
		t.Fatalf("list apps status=%d body=%s", listRes.Code, listRes.Body.String())
	}
	list := decodeMap(t, listRes.Body.Bytes())["items"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["client_id"] != clientID {
		t.Fatalf("owned app list mismatch: %#v", list)
	}
	getRes := doJSON(t, router, http.MethodGet, "/v2/oauth/apps/"+clientID, nil, ownerSession, "")
	if getRes.Code != http.StatusOK {
		t.Fatalf("get app status=%d body=%s", getRes.Code, getRes.Body.String())
	}
	got := decodeMap(t, getRes.Body.Bytes())
	if got["client_id"] != clientID || got["name"] != "Route managed app" {
		t.Fatalf("get app mismatch: %#v", got)
	}
	updateRes := doJSON(t, router, http.MethodPatch, "/v2/oauth/apps/"+clientID, map[string]any{
		"name":         "Route managed app updated",
		"description":  "Updated route description",
		"redirect_uri": "https://route.example/new-callback",
		"website_url":  "https://route.example/docs",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self"},
		"status":       "active",
	}, ownerSession, "")
	if updateRes.Code != http.StatusOK {
		t.Fatalf("update app status=%d body=%s", updateRes.Code, updateRes.Body.String())
	}
	updated := decodeMap(t, updateRes.Body.Bytes())
	if updated["name"] != "Route managed app updated" || updated["status"] != "pending" ||
		updated["redirect_uri"] != "https://route.example/new-callback" {
		t.Fatalf("owner update app mismatch: %#v", updated)
	}
	submitRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps/"+clientID+"/review-submission", nil, ownerSession, "")
	if submitRes.Code != http.StatusOK || decodeMap(t, submitRes.Body.Bytes())["status"] != "pending" {
		t.Fatalf("submit app mismatch: status=%d body=%s", submitRes.Code, submitRes.Body.String())
	}
	adminListRes := doJSON(t, router, http.MethodGet, "/v2/admin/oauth/apps?status=pending&limit=10", nil, adminSession, "")
	if adminListRes.Code != http.StatusOK {
		t.Fatalf("admin list status=%d body=%s", adminListRes.Code, adminListRes.Body.String())
	}
	adminItems := decodeMap(t, adminListRes.Body.Bytes())["items"].([]any)
	if len(adminItems) != 1 || adminItems[0].(map[string]any)["client_id"] != clientID {
		t.Fatalf("admin pending list mismatch: %#v", adminItems)
	}
	adminSummary := adminItems[0].(map[string]any)
	if _, ok := adminSummary["permissions"]; ok {
		t.Fatalf("admin list must not include permissions: %#v", adminSummary)
	}
	if _, ok := adminSummary["redirect_uri"]; ok {
		t.Fatalf("admin list must not include redirect_uri: %#v", adminSummary)
	}
	if adminSummary["name"] != "Route managed app updated" || adminSummary["status"] != "pending" ||
		adminSummary["client_type"] != "confidential" {
		t.Fatalf("admin summary fields mismatch: %#v", adminSummary)
	}
	adminDetailRes := doJSON(t, router, http.MethodGet, "/v2/admin/oauth/apps/"+clientID, nil, adminSession, "")
	if adminDetailRes.Code != http.StatusOK {
		t.Fatalf("admin detail status=%d body=%s", adminDetailRes.Code, adminDetailRes.Body.String())
	}
	adminDetail := decodeMap(t, adminDetailRes.Body.Bytes())
	if adminDetail["client_id"] != clientID ||
		adminDetail["redirect_uri"] != "https://route.example/new-callback" ||
		adminDetail["website_url"] != "https://route.example/docs" {
		t.Fatalf("admin detail fields mismatch: %#v", adminDetail)
	}
	adminPermissions := adminDetail["permissions"].([]any)
	if len(adminPermissions) != 1 || adminPermissions[0] != "account.read.self" {
		t.Fatalf("admin detail permissions mismatch: %#v", adminPermissions)
	}
	reviewRes := doJSON(t, router, http.MethodPatch, "/v2/admin/oauth/apps/"+clientID+"/review", map[string]any{"status": "active"}, adminSession, "")
	if reviewRes.Code != http.StatusOK || decodeMap(t, reviewRes.Body.Bytes())["status"] != "active" {
		t.Fatalf("review app mismatch: status=%d body=%s", reviewRes.Code, reviewRes.Body.String())
	}
	secretRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps/"+clientID+"/secret", nil, ownerSession, "")
	if secretRes.Code != http.StatusOK {
		t.Fatalf("rotate secret status=%d body=%s", secretRes.Code, secretRes.Body.String())
	}
	rotated := decodeMap(t, secretRes.Body.Bytes())
	if rotated["client_secret"] == "" || rotated["client_secret"] == firstSecret || rotated["status"] != "active" {
		t.Fatalf("rotated secret mismatch: %#v", rotated)
	}
	deleteRes := doJSON(t, router, http.MethodDelete, "/v2/oauth/apps/"+clientID, nil, ownerSession, "")
	if deleteRes.Code != http.StatusNoContent || deleteRes.Body.Len() != 0 {
		t.Fatalf("delete app mismatch: status=%d body=%s", deleteRes.Code, deleteRes.Body.String())
	}
	missingRes := doJSON(t, router, http.MethodGet, "/v2/oauth/apps/"+clientID, nil, ownerSession, "")
	if missingRes.Code != http.StatusNotFound || !strings.Contains(missingRes.Body.String(), "oauth client not found") {
		t.Fatalf("deleted get mismatch: status=%d body=%s", missingRes.Code, missingRes.Body.String())
	}
}

func TestOAuthCreateAppRejectsScopeMissingFromActor(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-scope-deny@test.com", "Password123", "OAuthScopeDeny", false)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	res := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Denied app",
		"redirect_uri": "https://client.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"account.ban.any"},
	}, webCookie(t, cfg.JWTSecret, user.ID), "")
	if res.Code != http.StatusForbidden || !strings.Contains(res.Body.String(), "permission denied") {
		t.Fatalf("scope deny mismatch: status=%d body=%s", res.Code, res.Body.String())
	}
}
