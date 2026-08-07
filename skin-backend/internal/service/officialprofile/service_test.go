package officialprofile_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	identitysvc "element-skin/backend/internal/service/identity"
	microsoftsvc "element-skin/backend/internal/service/microsoft"
	officialsvc "element-skin/backend/internal/service/officialprofile"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type fakeMicrosoftResolver struct {
	calls        int
	accessToken  string
	accessTokens []string
	result       microsoftsvc.ProfileResult
	err          error
	responses    []resolverResponse
}

type resolverResponse struct {
	result microsoftsvc.ProfileResult
	err    error
}

func (f *fakeMicrosoftResolver) Resolve(_ context.Context, accessToken string) (microsoftsvc.ProfileResult, error) {
	f.calls++
	f.accessToken = accessToken
	f.accessTokens = append(f.accessTokens, accessToken)
	if len(f.responses) >= f.calls {
		response := f.responses[f.calls-1]
		return response.result, response.err
	}
	return f.result, f.err
}

type fakeOfficialTokenRefresher struct {
	calls  int
	tokens identitysvc.OIDCTokens
	err    error
}

func (f *fakeOfficialTokenRefresher) Refresh(context.Context, model.IdentityProvider, string, string, []string) (identitysvc.OIDCTokens, error) {
	f.calls++
	return f.tokens, f.err
}

func TestOfficialBindingCreationRecordsRelationshipWithoutMutatingProfile(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, identity, profile, identities, cache := officialFixture(t, db, identitysvc.AdapterMicrosoft)
	resolver := &fakeMicrosoftResolver{result: microsoftsvc.ProfileResult{HasGame: true, Profile: remoteProfile()}}
	service := officialsvc.Service{DB: db, Identities: identities, Resolver: resolver, TexturesDir: t.TempDir()}
	actor := officialActor(user.ID, "official_profile.create.owned")

	created, err := service.Create(ctx, actor, identity.ID, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	bindingID, _ := created["id"].(string)
	if bindingID == "" || created["identity_id"] != identity.ID || created["profile_id"] != profile.ID || created["remote_uuid"] != "0123456789abcdef0123456789abcdef" || created["remote_name"] != "RemoteSteve" || created["remote_skin_url"] != "https://textures.example/skin.png" || created["remote_cape_url"] != "https://textures.example/cape.png" || created["remote_skin_model"] != "slim" || created["last_synced_at"] != (*int64)(nil) {
		t.Fatalf("created binding mismatch: %#v", created)
	}
	storedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || storedProfile == nil || *storedProfile != profile {
		t.Fatalf("binding creation mutated profile: profile=%#v err=%v", storedProfile, err)
	}
	if resolver.calls != 1 || resolver.accessToken != "microsoft-access" {
		t.Fatalf("resolver calls=%d access=%q", resolver.calls, resolver.accessToken)
	}
	if _, err := cache.GetExternalAccessToken(ctx, identity.ID); err != nil {
		t.Fatalf("binding creation should retain cached identity token: %v", err)
	}

	_, err = service.Create(ctx, actor, identity.ID, profile.ID)
	assertOfficialHTTPError(t, err, 409, "profile already has an official binding")
}

func TestOfficialBindingSyncUpdatesRoleAndTexturesOnlyWhenExplicitlyRequested(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, identity, profile, identities, _ := officialFixture(t, db, identitysvc.AdapterMicrosoft)
	resolver := &fakeMicrosoftResolver{result: microsoftsvc.ProfileResult{HasGame: true, Profile: remoteProfile()}}
	textureDir := t.TempDir()
	service := officialsvc.Service{
		DB: db, Identities: identities, Resolver: resolver, TexturesDir: textureDir,
		Download: func(_ context.Context, rawURL string) ([]byte, error) {
			switch rawURL {
			case "https://textures.example/skin.png":
				return texturePNG(t, 64, 64, color.RGBA{R: 10, G: 20, B: 30, A: 255}), nil
			case "https://textures.example/cape.png":
				return texturePNG(t, 64, 32, color.RGBA{R: 40, G: 50, B: 60, A: 255}), nil
			default:
				t.Fatalf("unexpected texture URL %q", rawURL)
				return nil, nil
			}
		},
	}
	created, err := service.Create(ctx, officialActor(user.ID, "official_profile.create.owned"), identity.ID, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	bindingID := created["id"].(string)
	result, err := service.Sync(ctx, officialActor(user.ID, "official_profile.refresh.owned"), bindingID)
	if err != nil {
		t.Fatal(err)
	}
	if result["remote_name"] != "RemoteSteve" || result["last_synced_at"] == nil {
		t.Fatalf("sync response mismatch: %#v", result)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.Name != "RemoteSteve" || updated.TextureModel != "slim" || updated.SkinHash == nil || updated.CapeHash == nil {
		t.Fatalf("synced profile=%#v err=%v", updated, err)
	}
	if _, err := os.Stat(textureDir + string(os.PathSeparator) + *updated.SkinHash + ".png"); err != nil {
		t.Fatalf("synced skin file missing: %v", err)
	}
	if _, err := os.Stat(textureDir + string(os.PathSeparator) + *updated.CapeHash + ".png"); err != nil {
		t.Fatalf("synced cape file missing: %v", err)
	}
	if skin, err := db.Textures.GetInfo(ctx, user.ID, *updated.SkinHash, "skin"); err != nil || skin == nil || skin["model"] != "slim" || skin["is_public"] != 0 {
		t.Fatalf("synced skin library row=%#v err=%v", skin, err)
	}
	if cape, err := db.Textures.GetInfo(ctx, user.ID, *updated.CapeHash, "cape"); err != nil || cape == nil || cape["model"] != "default" || cape["is_public"] != 0 {
		t.Fatalf("synced cape library row=%#v err=%v", cape, err)
	}
}

func TestOfficialBindingSyncFailureCleansFilesAndLeavesBindingAndProfileUntouched(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, identity, profile, identities, _ := officialFixture(t, db, identitysvc.AdapterMicrosoft)
	resolver := &fakeMicrosoftResolver{result: microsoftsvc.ProfileResult{HasGame: true, Profile: remoteProfile()}}
	textureDir := t.TempDir()
	service := officialsvc.Service{
		DB: db, Identities: identities, Resolver: resolver, TexturesDir: textureDir,
		Download: func(_ context.Context, rawURL string) ([]byte, error) {
			if rawURL == "https://textures.example/skin.png" {
				return texturePNG(t, 64, 64, color.RGBA{R: 1, A: 255}), nil
			}
			return []byte("not a PNG"), nil
		},
	}
	created, err := service.Create(ctx, officialActor(user.ID, "official_profile.create.owned"), identity.ID, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Sync(ctx, officialActor(user.ID, "official_profile.refresh.owned"), created["id"].(string))
	assertOfficialHTTPError(t, err, 502, "Microsoft profile texture is invalid")
	entries, readErr := os.ReadDir(textureDir)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("failed sync left files=%#v err=%v", entries, readErr)
	}
	stored, getErr := db.Profiles.GetByID(ctx, profile.ID)
	if getErr != nil || stored == nil || *stored != profile {
		t.Fatalf("failed sync mutated profile=%#v err=%v", stored, getErr)
	}
	bindings, listErr := db.OfficialProfiles.ListByUser(ctx, user.ID)
	if listErr != nil || len(bindings) != 1 || bindings[0].Binding.LastSyncedAt != nil || bindings[0].Binding.RemoteName != "RemoteSteve" {
		t.Fatalf("failed sync mutated binding=%#v err=%v", bindings, listErr)
	}
}

func TestOfficialBindingRejectsWrongAdapterAndPermissionsBeforeRemoteCalls(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, identity, profile, identities, _ := officialFixture(t, db, identitysvc.AdapterGenericOIDC)
	resolver := &fakeMicrosoftResolver{err: errors.New("must not call")}
	service := officialsvc.Service{DB: db, Identities: identities, Resolver: resolver, TexturesDir: t.TempDir()}

	_, err := service.Create(ctx, officialActor(user.ID), identity.ID, profile.ID)
	assertOfficialHTTPError(t, err, 403, "permission denied")
	_, err = service.Create(ctx, officialActor(user.ID, "official_profile.create.owned"), identity.ID, profile.ID)
	assertOfficialHTTPError(t, err, 409, "external identity is not a Microsoft identity")
	if resolver.calls != 0 {
		t.Fatalf("rejected binding called Microsoft resolver %d times", resolver.calls)
	}
}

func TestOfficialBindingRetriesOneUnauthorizedResponseWithForcedIdentityRefresh(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, external, profile, identities, cache := officialFixture(t, db, identitysvc.AdapterMicrosoft)
	refresher := &fakeOfficialTokenRefresher{tokens: identitysvc.OIDCTokens{
		AccessToken: "refreshed-microsoft-access", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour),
	}}
	identities.TokenRefresher = refresher
	resolver := &fakeMicrosoftResolver{responses: []resolverResponse{
		{err: &microsoftsvc.UpstreamHTTPError{StatusCode: 401, Body: "expired"}},
		{result: microsoftsvc.ProfileResult{HasGame: true, Profile: remoteProfile()}},
	}}
	service := officialsvc.Service{DB: db, Identities: identities, Resolver: resolver, TexturesDir: t.TempDir()}

	created, err := service.Create(ctx, officialActor(user.ID, "official_profile.create.owned"), external.ID, profile.ID)
	if err != nil || created["identity_id"] != external.ID {
		t.Fatalf("binding after access refresh=%#v err=%v", created, err)
	}
	if resolver.calls != 2 || len(resolver.accessTokens) != 2 || resolver.accessTokens[0] != "microsoft-access" || resolver.accessTokens[1] != "refreshed-microsoft-access" || refresher.calls != 1 {
		t.Fatalf("unauthorized retry resolver=%#v refresher_calls=%d", resolver, refresher.calls)
	}
	cached, err := cache.GetExternalAccessToken(ctx, external.ID)
	if err != nil || cached.AccessToken != "refreshed-microsoft-access" {
		t.Fatalf("refreshed Microsoft token cache=%#v err=%v", cached, err)
	}
}

func TestOfficialBindingStopsAfterOneUnauthorizedRetry(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, external, profile, identities, _ := officialFixture(t, db, identitysvc.AdapterMicrosoft)
	refresher := &fakeOfficialTokenRefresher{tokens: identitysvc.OIDCTokens{
		AccessToken: "still-rejected-access", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour),
	}}
	identities.TokenRefresher = refresher
	resolver := &fakeMicrosoftResolver{responses: []resolverResponse{
		{err: &microsoftsvc.UpstreamHTTPError{StatusCode: 401}},
		{err: &microsoftsvc.UpstreamHTTPError{StatusCode: 401}},
	}}
	service := officialsvc.Service{DB: db, Identities: identities, Resolver: resolver, TexturesDir: t.TempDir()}

	_, err := service.Create(ctx, officialActor(user.ID, "official_profile.create.owned"), external.ID, profile.ID)
	assertOfficialHTTPError(t, err, 502, "Microsoft profile request failed")
	if resolver.calls != 2 || refresher.calls != 1 {
		t.Fatalf("unauthorized retry count resolver=%d refresh=%d; want 2/1", resolver.calls, refresher.calls)
	}
	bindings, listErr := db.OfficialProfiles.ListByUser(ctx, user.ID)
	if listErr != nil || len(bindings) != 0 {
		t.Fatalf("failed unauthorized retry created bindings=%#v err=%v", bindings, listErr)
	}
}

func officialFixture(t *testing.T, db *database.DB, adapter string) (model.User, model.ExternalIdentity, model.Profile, identitysvc.Service, redisstore.Store) {
	t.Helper()
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "official-owner-"+adapter+"@test.com", "Password123", "OfficialOwner", false)
	provider := model.IdentityProvider{
		ID: "official-provider-" + adapter, Name: "Provider", IssuerURL: "https://login.example",
		AuthorizationEndpoint: "https://login.example/authorize", TokenEndpoint: "https://login.example/token",
		JWKSURI: "https://login.example/jwks", ClientID: "client", Adapter: adapter, Enabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	box, err := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.ClientSecretCiphertext, err = box.Encrypt("client-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	identity := model.ExternalIdentity{ID: "official-identity-" + adapter, UserID: user.ID, ProviderID: provider.ID, Subject: "subject", Label: "Microsoft account", CreatedAt: 2, UpdatedAt: 2}
	refreshToken, err := box.Encrypt("stored-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Identities.CreateIdentity(ctx, identity, model.ExternalIdentityCredential{
		IdentityID: identity.ID, RefreshTokenCiphertext: refreshToken,
		GrantedScopes: []string{"openid", "offline_access"}, UpdatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "official-profile-"+adapter, "LocalRole")
	cache := redisstore.NewMemoryStore()
	if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{IdentityID: identity.ID, AccessToken: "microsoft-access", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	identities := identitysvc.Service{DB: db, Config: testutil.TestConfig(), Redis: cache}
	return user, identity, profile, identities, cache
}

func remoteProfile() *microsoftsvc.MinecraftProfile {
	return &microsoftsvc.MinecraftProfile{
		ID: "01234567-89ab-cdef-0123-456789abcdef", Name: "RemoteSteve",
		Skins: []microsoftsvc.MinecraftTexture{{ID: "skin", State: "ACTIVE", URL: "https://textures.example/skin.png", Variant: "SLIM"}},
		Capes: []microsoftsvc.MinecraftTexture{{ID: "cape", State: "ACTIVE", URL: "https://textures.example/cape.png"}},
	}
}

func officialActor(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{SubjectID: "user:" + userID, UserID: userID, Permissions: bits}
}

func texturePNG(t *testing.T, width, height int, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func assertOfficialHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Detail != detail {
		t.Fatalf("HTTP error mismatch: got=%#v want status=%d detail=%q", err, status, detail)
	}
}
