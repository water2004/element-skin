package identity_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	identityapi "element-skin/backend/internal/httpapi/identity"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	identitysvc "element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
)

func TestProviderRoutesUseExactV2ContractsAndNeverExposeClientSecret(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	var discoveryServer *httptest.Server
	discoveryServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"userinfo_endpoint":%q,"jwks_uri":%q}`,
			discoveryServer.URL, discoveryServer.URL+"/authorize", discoveryServer.URL+"/token",
			discoveryServer.URL+"/userinfo", discoveryServer.URL+"/jwks")
	}))
	defer discoveryServer.Close()
	h := identityapi.New(cfg, db, testutil.NewMemoryRedis(), nil)
	admin := testutil.CreateUser(t, db, "identity-route-admin@example.com", "Password123", "IdentityRouteAdmin", true)
	adminActor := identityRouteActor(t, db, admin.ID)

	requestBody := fmt.Sprintf(`{"name":"Example Accounts","issuer_url":%q,"client_id":"client-id","client_secret":"super-secret","scopes":["profile email"],"adapter":"generic_oidc","icon_url":"https://accounts.example/icon.png","enabled":true,"login_enabled":true,"link_enabled":true,"registration_enabled":true,"display_order":7}`, discoveryServer.URL)
	req := identityRouteRequest(http.MethodPost, "/v2/admin/identity-providers", requestBody, adminActor)
	rec := httptest.NewRecorder()
	h.CreateProvider(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create provider status=%d body=%q", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	providerID, _ := created["id"].(string)
	if providerID == "" || len(created) != 19 || created["name"] != "Example Accounts" ||
		created["issuer_url"] != discoveryServer.URL || created["client_id"] != "client-id" ||
		created["has_client_secret"] != true || created["enabled"] != true || created["display_order"] != float64(7) ||
		strings.Contains(rec.Body.String(), "super-secret") || strings.Contains(rec.Body.String(), "client_secret_ciphertext") {
		t.Fatalf("create provider response mismatch: %#v", created)
	}
	if scopes, ok := created["scopes"].([]any); !ok || len(scopes) != 3 || scopes[0] != "email" || scopes[1] != "openid" || scopes[2] != "profile" {
		t.Fatalf("normalized scopes mismatch: %#v", created["scopes"])
	}

	req = identityRouteRequest(http.MethodGet, "/v2/auth/identity-providers", "", permission.GuestActor())
	rec = httptest.NewRecorder()
	h.PublicProviders(rec, req)
	wantPublic := `{"items":[{"adapter":"generic_oidc","icon_url":"https://accounts.example/icon.png","id":"` + providerID + `","link_enabled":true,"login_enabled":true,"name":"Example Accounts","registration_enabled":true}]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != wantPublic {
		t.Fatalf("public provider response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = identityRouteRequest(http.MethodDelete, "/v2/admin/identity-providers/"+providerID, "", adminActor)
	req.SetPathValue("provider_id", providerID)
	rec = httptest.NewRecorder()
	h.DeleteProvider(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete provider response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if provider, err := db.Identities.GetProvider(req.Context(), providerID); err != nil || provider != nil {
		t.Fatalf("deleted provider remains: provider=%#v err=%v", provider, err)
	}
}

func TestIdentityRoutesListUpdateDeleteOnlyOwnedRecordsWithExactErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := identityapi.New(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)
	owner := testutil.CreateUser(t, db, "identity-owner@example.com", "Password123", "IdentityOwner", false)
	other := testutil.CreateUser(t, db, "identity-other@example.com", "Password123", "IdentityOther", false)
	provider := model.IdentityProvider{
		ID: "identity-route-provider", Name: "Route Provider", IssuerURL: "https://route.example",
		AuthorizationEndpoint: "https://route.example/authorize", TokenEndpoint: "https://route.example/token",
		JWKSURI: "https://route.example/jwks", ClientID: "client", Scopes: []string{"openid"},
		Adapter: identitysvc.AdapterGenericOIDC, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		RegistrationEnabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(t.Context(), provider); err != nil {
		t.Fatal(err)
	}
	for _, item := range []model.ExternalIdentity{
		{ID: "identity-a", UserID: owner.ID, ProviderID: provider.ID, Subject: "subject-a", Label: "first", Email: "a@remote.example", EmailVerified: true, DisplayName: "Remote A", CreatedAt: 10, UpdatedAt: 11},
		{ID: "identity-b", UserID: owner.ID, ProviderID: provider.ID, Subject: "subject-b", Label: "second", Email: "b@remote.example", DisplayName: "Remote B", CreatedAt: 20, UpdatedAt: 21},
	} {
		if err := db.Identities.CreateIdentity(t.Context(), item, model.ExternalIdentityCredential{IdentityID: item.ID, GrantedScopes: []string{}, UpdatedAt: item.UpdatedAt}); err != nil {
			t.Fatal(err)
		}
	}
	if updated, err := db.Identities.MarkCredentialRefreshRejected(t.Context(), "identity-b", 22); err != nil || !updated {
		t.Fatalf("mark identity-b reauthorization state updated=%v err=%v", updated, err)
	}
	ownerActor := identityRouteActor(t, db, owner.ID)
	otherActor := identityRouteActor(t, db, other.ID)

	req := identityRouteRequest(http.MethodGet, "/v2/users/me/identities", "", ownerActor)
	rec := httptest.NewRecorder()
	h.ListIdentities(rec, req)
	wantList := `{"items":[{"authorization_status":"active","avatar_url":"","created_at":10,"display_name":"Remote A","email":"a@remote.example","email_verified":true,"id":"identity-a","label":"first","last_login_at":null,"last_refresh_at":null,"last_refresh_error_at":null,"provider_adapter":"generic_oidc","provider_id":"identity-route-provider","provider_name":"Route Provider","subject":"subject-a","updated_at":11},{"authorization_status":"reauthorization_required","avatar_url":"","created_at":20,"display_name":"Remote B","email":"b@remote.example","email_verified":false,"id":"identity-b","label":"second","last_login_at":null,"last_refresh_at":null,"last_refresh_error_at":22,"provider_adapter":"generic_oidc","provider_id":"identity-route-provider","provider_name":"Route Provider","subject":"subject-b","updated_at":21}]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != wantList {
		t.Fatalf("identity list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = identityRouteRequest(http.MethodPatch, "/v2/users/me/identities/identity-a", `{"label":"  primary  "}`, ownerActor)
	req.SetPathValue("identity_id", "identity-a")
	rec = httptest.NewRecorder()
	h.UpdateIdentity(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("identity update response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Identities.GetIdentity(t.Context(), "identity-a")
	if err != nil || updated == nil || updated.Label != "primary" {
		t.Fatalf("identity label state mismatch: identity=%#v err=%v", updated, err)
	}

	req = identityRouteRequest(http.MethodDelete, "/v2/users/me/identities/identity-a", "", otherActor)
	req.SetPathValue("identity_id", "identity-a")
	rec = httptest.NewRecorder()
	h.DeleteIdentity(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"external identity not found\"}\n" {
		t.Fatalf("cross-account delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if item, err := db.Identities.GetIdentity(t.Context(), "identity-a"); err != nil || item == nil {
		t.Fatalf("cross-account delete mutated identity: identity=%#v err=%v", item, err)
	}

	req = identityRouteRequest(http.MethodDelete, "/v2/users/me/identities/identity-a", "", ownerActor)
	req.SetPathValue("identity_id", "identity-a")
	rec = httptest.NewRecorder()
	h.DeleteIdentity(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("owned identity delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if item, err := db.Identities.GetIdentity(t.Context(), "identity-a"); err != nil || item != nil {
		t.Fatalf("owned identity was not deleted: identity=%#v err=%v", item, err)
	}
}

func TestIdentityRoutesRejectInvalidJSONAndMissingPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := identityapi.New(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)

	req := identityRouteRequest(http.MethodPost, "/v2/admin/identity-providers", `{`, permission.Actor{})
	rec := httptest.NewRecorder()
	h.CreateProvider(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("provider invalid JSON response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = identityRouteRequest(http.MethodGet, "/v2/users/me/identities", "", permission.Actor{})
	rec = httptest.NewRecorder()
	h.ListIdentities(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
		t.Fatalf("identity permission response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func identityRouteRequest(method, target, body string, actor permission.Actor) *http.Request {
	var reader *strings.Reader
	reader = strings.NewReader(body)
	req := httptest.NewRequest(method, target, reader)
	return req.WithContext(shared.WithActor(req.Context(), actor))
}

func identityRouteActor(t *testing.T, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(t.Context(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatal(err)
	}
	return actor
}
