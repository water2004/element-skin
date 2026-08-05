package identity_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type fakeTokenRefresher struct {
	calls        int
	provider     model.IdentityProvider
	clientSecret string
	refreshToken string
	scopes       []string
	tokens       identity.OIDCTokens
	err          error
}

type blockingTokenRefresher struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	tokens  identity.OIDCTokens
}

func (f *blockingTokenRefresher) Refresh(context.Context, model.IdentityProvider, string, string, []string) (identity.OIDCTokens, error) {
	if f.calls.Add(1) == 1 {
		close(f.started)
	}
	<-f.release
	return f.tokens, nil
}

func (f *fakeTokenRefresher) Refresh(_ context.Context, provider model.IdentityProvider, clientSecret, refreshToken string, scopes []string) (identity.OIDCTokens, error) {
	f.calls++
	f.provider = provider
	f.clientSecret = clientSecret
	f.refreshToken = refreshToken
	f.scopes = append([]string(nil), scopes...)
	return f.tokens, f.err
}

func TestExternalIdentityAccessTokenUsesCacheWithoutRefreshing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, provider, external := createRefreshIdentity(t, db)
	cache := redisstore.NewMemoryStore()
	expiresAt := time.Now().Add(time.Hour).UnixMilli()
	if err := cache.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{IdentityID: external.ID, AccessToken: "cached-access", TokenType: "Bearer", ExpiresAt: expiresAt}, time.Hour); err != nil {
		t.Fatal(err)
	}
	refresher := &fakeTokenRefresher{err: errors.New("must not refresh")}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, TokenRefresher: refresher}
	result, err := service.AccessTokenForOwnedIdentity(ctx, user.ID, external.ID)
	if err != nil || result.AccessToken != "cached-access" || result.Identity != external || !reflect.DeepEqual(result.Provider, provider) || refresher.calls != 0 {
		t.Fatalf("cached token result=%#v calls=%d err=%v", result, refresher.calls, err)
	}
}

func TestExternalIdentityAccessTokenRefreshesOnDemandAndPersistsRotationExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, provider, external := createRefreshIdentity(t, db)
	cache := redisstore.NewMemoryStore()
	expires := time.Now().Add(45 * time.Minute)
	refresher := &fakeTokenRefresher{tokens: identity.OIDCTokens{
		AccessToken: "rotated-access", RefreshToken: "rotated-refresh", TokenType: "Bearer",
		Expiry: expires, Scopes: []string{"XboxLive.signin", "openid"},
	}}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, TokenRefresher: refresher}
	result, err := service.AccessTokenForOwnedIdentity(ctx, user.ID, external.ID)
	if err != nil || result.AccessToken != "rotated-access" || refresher.calls != 1 || refresher.provider.ID != provider.ID || refresher.clientSecret != "client-secret" || refresher.refreshToken != "stored-refresh" || !reflect.DeepEqual(refresher.scopes, []string{"XboxLive.signin", "offline_access", "openid"}) {
		t.Fatalf("refresh result=%#v refresher=%#v err=%v", result, refresher, err)
	}
	credential, err := db.Identities.GetCredential(ctx, external.ID)
	if err != nil || credential == nil || !reflect.DeepEqual(credential.GrantedScopes, []string{"XboxLive.signin", "openid"}) {
		t.Fatalf("rotated credential=%#v err=%v", credential, err)
	}
	box, _ := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	refreshToken, err := box.Decrypt(credential.RefreshTokenCiphertext)
	if err != nil || refreshToken != "rotated-refresh" {
		t.Fatalf("rotated refresh token=%q err=%v", refreshToken, err)
	}
	cached, err := cache.GetExternalAccessToken(ctx, external.ID)
	if err != nil || cached.AccessToken != "rotated-access" || cached.TokenType != "Bearer" || cached.ExpiresAt != expires.UnixMilli() {
		t.Fatalf("refreshed cache=%#v err=%v", cached, err)
	}
}

func TestExternalIdentityAccessTokenRejectsForeignAndRejectedRefreshWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, _, external := createRefreshIdentity(t, db)
	other := testutil.CreateUser(t, db, "refresh-other@test.com", "Password123", "RefreshOther", false)
	refresher := &fakeTokenRefresher{err: identity.ErrRefreshRejected}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: redisstore.NewMemoryStore(), TokenRefresher: refresher}

	_, err := service.AccessTokenForOwnedIdentity(ctx, other.ID, external.ID)
	assertHTTPError(t, err, 404, "external identity not found")
	if refresher.calls != 0 {
		t.Fatalf("foreign identity triggered refresh calls=%d", refresher.calls)
	}
	_, err = service.AccessTokenForOwnedIdentity(ctx, user.ID, external.ID)
	assertHTTPError(t, err, 409, "external identity must be reauthorized")
	credential, getErr := db.Identities.GetCredential(ctx, external.ID)
	if getErr != nil || credential == nil {
		t.Fatalf("credential lookup after rejected refresh=%#v err=%v", credential, getErr)
	}
	box, _ := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	refreshToken, decryptErr := box.Decrypt(credential.RefreshTokenCiphertext)
	if decryptErr != nil || refreshToken != "stored-refresh" || refresher.calls != 1 {
		t.Fatalf("rejected refresh mutated token=%q calls=%d err=%v", refreshToken, refresher.calls, decryptErr)
	}
}

func TestExternalIdentityConcurrentAccessCoalescesRefreshTokenRotation(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user, _, external := createRefreshIdentity(t, db)
	refresher := &blockingTokenRefresher{
		started: make(chan struct{}), release: make(chan struct{}),
		tokens: identity.OIDCTokens{AccessToken: "coalesced-access", RefreshToken: "coalesced-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: redisstore.NewMemoryStore(), TokenRefresher: refresher}
	const callers = 8
	results := make(chan string, callers)
	errorsFound := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.AccessTokenForOwnedIdentity(ctx, user.ID, external.ID)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- result.AccessToken
		}()
	}
	<-refresher.started
	time.Sleep(100 * time.Millisecond)
	close(refresher.release)
	group.Wait()
	close(results)
	close(errorsFound)
	if len(errorsFound) != 0 {
		t.Fatalf("concurrent refresh errors=%#v", <-errorsFound)
	}
	if len(results) != callers {
		t.Fatalf("concurrent refresh results=%d; want %d", len(results), callers)
	}
	for token := range results {
		if token != "coalesced-access" {
			t.Fatalf("concurrent access token=%q", token)
		}
	}
	if refresher.calls.Load() != 1 {
		t.Fatalf("concurrent refresh calls=%d; want 1", refresher.calls.Load())
	}
}

func createRefreshIdentity(t *testing.T, databaseDB *database.DB) (model.User, model.IdentityProvider, model.ExternalIdentity) {
	t.Helper()
	ctx := context.Background()
	user := testutil.CreateUser(t, databaseDB, "refresh-owner@test.com", "Password123", "RefreshOwner", false)
	box, _ := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	clientSecret, _ := box.Encrypt("client-secret")
	refreshToken, _ := box.Encrypt("stored-refresh")
	provider := model.IdentityProvider{
		ID: "refresh-provider", Name: "Microsoft", IssuerURL: "https://login.example",
		AuthorizationEndpoint: "https://login.example/authorize", TokenEndpoint: "https://login.example/token",
		JWKSURI: "https://login.example/jwks", ClientID: "client-id", ClientSecretCiphertext: clientSecret,
		Scopes: []string{"XboxLive.signin", "offline_access", "openid"}, Adapter: identity.AdapterMicrosoft,
		Enabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := databaseDB.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	external := model.ExternalIdentity{ID: "refresh-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", CreatedAt: 2, UpdatedAt: 2}
	credential := model.ExternalIdentityCredential{IdentityID: external.ID, RefreshTokenCiphertext: refreshToken, GrantedScopes: []string{"XboxLive.signin", "offline_access", "openid"}, UpdatedAt: 2}
	if err := databaseDB.Identities.CreateIdentity(ctx, external, credential); err != nil {
		t.Fatal(err)
	}
	return user, provider, external
}
