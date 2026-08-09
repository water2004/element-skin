package oauth_test

import (
	"net/http"
	"testing"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestOAuthAppRoutesCreateAndReadOptionalPermissionBoundWebhookEndpointsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	owner := testutil.CreateUser(t, db, "oauth-webhook-route@test.com", "Password123", "OAuthWebhookRoute", false)
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, owner.ID)

	catalogRes := doJSON(t, router, http.MethodGet, "/v2/oauth/webhook-events", nil, session, "")
	if catalogRes.Code != http.StatusOK {
		t.Fatalf("webhook catalog status=%d body=%s", catalogRes.Code, catalogRes.Body.String())
	}
	events := decodeMap(t, catalogRes.Body.Bytes())["events"].([]any)
	if len(events) != 15 {
		t.Fatalf("webhook catalog events=%#v", events)
	}
	first := events[0].(map[string]any)
	if first["type"] != "account.created" || first["description"] == "" ||
		len(first["required_permissions"].([]any)) != 1 || first["application_permission"] != "account.read.any" {
		t.Fatalf("webhook catalog first event mismatch: %#v", first)
	}

	createRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":         "Webhook route app",
		"redirect_uri": "",
		"client_type":  "public",
		"permissions":  []string{"profile.read.owned"},
		"webhook_endpoints": []map[string]any{{
			"url":     "https://hooks.example/route",
			"events":  []string{"profile.updated", "profile.created"},
			"enabled": true,
		}},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create webhook app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	created := decodeMap(t, createRes.Body.Bytes())
	createdEndpoints := created["webhook_endpoints"].([]any)
	if len(createdEndpoints) != 1 {
		t.Fatalf("created webhook endpoints=%#v", createdEndpoints)
	}
	createdEndpoint := createdEndpoints[0].(map[string]any)
	if createdEndpoint["id"] == "" || createdEndpoint["signing_secret"] == "" || createdEndpoint["url"] != "https://hooks.example/route" || createdEndpoint["enabled"] != true {
		t.Fatalf("created webhook endpoint mismatch: %#v", createdEndpoint)
	}
	getRes := doJSON(t, router, http.MethodGet, "/v2/oauth/apps/"+created["client_id"].(string), nil, session, "")
	if getRes.Code != http.StatusOK {
		t.Fatalf("get webhook app status=%d body=%s", getRes.Code, getRes.Body.String())
	}
	gotEndpoint := decodeMap(t, getRes.Body.Bytes())["webhook_endpoints"].([]any)[0].(map[string]any)
	if _, exposed := gotEndpoint["signing_secret"]; exposed {
		t.Fatalf("get app exposed signing secret: %#v", gotEndpoint)
	}

	badRes := doJSON(t, router, http.MethodPost, "/v2/oauth/apps", map[string]any{
		"name":        "Webhook route bad app",
		"client_type": "public",
		"permissions": []string{"account.read.self"},
		"webhook_endpoints": []map[string]any{{
			"url":     "https://hooks.example/bad-route",
			"events":  []string{"profile.updated"},
			"enabled": true,
		}},
	}, session, "")
	if badRes.Code != http.StatusBadRequest || decodeMap(t, badRes.Body.Bytes())["detail"] != "webhook event exceeds client permission limit" {
		t.Fatalf("permission-bound webhook rejection status=%d body=%s", badRes.Code, badRes.Body.String())
	}
}
