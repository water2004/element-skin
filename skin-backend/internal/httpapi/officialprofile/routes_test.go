package officialprofile_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	officialapi "element-skin/backend/internal/httpapi/officialprofile"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	identitysvc "element-skin/backend/internal/service/identity"
	microsoftsvc "element-skin/backend/internal/service/microsoft"
	officialsvc "element-skin/backend/internal/service/officialprofile"
	"element-skin/backend/internal/testutil"
)

type routeResolver struct {
	result microsoftsvc.ProfileResult
}

func (r routeResolver) Resolve(context.Context, string) (microsoftsvc.ProfileResult, error) {
	return r.result, nil
}

func TestOfficialProfileRoutesUseExactSeparateV2ResourceContracts(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "official-route@test.com", "Password123", "OfficialRoute", false)
	provider := model.IdentityProvider{ID: "official-route-provider", Name: "Microsoft", IssuerURL: "https://login.example", AuthorizationEndpoint: "https://login.example/a", TokenEndpoint: "https://login.example/t", JWKSURI: "https://login.example/j", ClientID: "client", Adapter: identitysvc.AdapterMicrosoft, Enabled: true, CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	external := model.ExternalIdentity{ID: "official-route-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", Label: "Work account", CreatedAt: 2, UpdatedAt: 2}
	if err := db.Identities.CreateIdentity(ctx, external, model.ExternalIdentityCredential{IdentityID: external.ID, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "official-route-profile", "RouteLocal")
	cache := redisstore.NewMemoryStore()
	if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{IdentityID: external.ID, AccessToken: "route-access", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	h := officialapi.NewWithService(officialsvc.Service{
		DB:         db,
		Identities: identitysvc.Service{DB: db, Config: testutil.TestConfig(), Redis: cache},
		Resolver: routeResolver{result: microsoftsvc.ProfileResult{HasGame: true, Profile: &microsoftsvc.MinecraftProfile{
			ID: "0123456789abcdef0123456789abcdef", Name: "HttpRemote",
		}}},
		TexturesDir: t.TempDir(),
	}, nil)
	actor := routeActor(user.ID,
		"official_profile.read.owned", "official_profile.create.owned",
		"official_profile.refresh.owned", "official_profile.delete.owned",
	)

	badReq := httptest.NewRequest(http.MethodPost, "/v2/users/me/official-profile-bindings", strings.NewReader("{"))
	badReq = badReq.WithContext(shared.WithActor(badReq.Context(), actor))
	badRec := httptest.NewRecorder()
	h.Create(badRec, badReq)
	if badRec.Code != http.StatusBadRequest || badRec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("invalid create mismatch: status=%d body=%q", badRec.Code, badRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v2/users/me/official-profile-bindings", strings.NewReader(`{"identity_id":"`+external.ID+`","profile_id":"`+profile.ID+`"}`))
	createReq = createReq.WithContext(shared.WithActor(createReq.Context(), actor))
	createRec := httptest.NewRecorder()
	h.Create(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", createRec.Code, createRec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	bindingID, _ := created["id"].(string)
	if len(created) != 13 || bindingID == "" || created["identity_id"] != external.ID || created["profile_id"] != profile.ID || created["remote_name"] != "HttpRemote" || created["last_synced_at"] != nil {
		t.Fatalf("created resource mismatch: %#v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v2/users/me/official-profile-bindings", nil)
	listReq = listReq.WithContext(shared.WithActor(listReq.Context(), actor))
	listRec := httptest.NewRecorder()
	h.List(listRec, listReq)
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listRec.Code != http.StatusOK || len(listed.Items) != 1 || listed.Items[0]["id"] != bindingID {
		t.Fatalf("list mismatch: status=%d items=%#v body=%q", listRec.Code, listed.Items, listRec.Body.String())
	}

	syncReq := httptest.NewRequest(http.MethodPost, "/v2/users/me/official-profile-bindings/"+bindingID+"/sync", nil)
	syncReq.SetPathValue("binding_id", bindingID)
	syncReq = syncReq.WithContext(shared.WithActor(syncReq.Context(), actor))
	syncRec := httptest.NewRecorder()
	h.Sync(syncRec, syncReq)
	var synced map[string]any
	if err := json.Unmarshal(syncRec.Body.Bytes(), &synced); err != nil {
		t.Fatal(err)
	}
	if syncRec.Code != http.StatusOK || synced["id"] != bindingID || synced["last_synced_at"] == nil {
		t.Fatalf("sync mismatch: status=%d resource=%#v body=%q", syncRec.Code, synced, syncRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v2/users/me/official-profile-bindings/"+bindingID, nil)
	deleteReq.SetPathValue("binding_id", bindingID)
	deleteReq = deleteReq.WithContext(shared.WithActor(deleteReq.Context(), actor))
	deleteRec := httptest.NewRecorder()
	h.Delete(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent || deleteRec.Body.Len() != 0 {
		t.Fatalf("delete mismatch: status=%d body=%q", deleteRec.Code, deleteRec.Body.String())
	}
	if stored, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || stored == nil || stored.Name != "HttpRemote" {
		t.Fatalf("binding delete coupled profile deletion: profile=%#v err=%v", stored, err)
	}
}

func TestOfficialProfileRoutesReturnExactPermissionError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	h := officialapi.NewWithService(officialsvc.Service{DB: db}, nil)
	req := httptest.NewRequest(http.MethodGet, "/v2/users/me/official-profile-bindings", nil)
	req = req.WithContext(shared.WithActor(req.Context(), routeActor("user-without-permission")))
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
		t.Fatalf("permission error mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func routeActor(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{SubjectID: "user:" + userID, UserID: userID, Permissions: bits}
}
