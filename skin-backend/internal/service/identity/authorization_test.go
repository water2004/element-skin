package identity_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

type fakeOIDCClient struct {
	claims        identity.OIDCClaims
	tokens        identity.OIDCTokens
	err           error
	provider      model.IdentityProvider
	clientSecret  string
	code          string
	redirectURI   string
	pkceVerifier  string
	expectedNonce string
	calls         int
}

func (c *fakeOIDCClient) ExchangeAndVerify(
	_ context.Context,
	provider model.IdentityProvider,
	clientSecret string,
	code string,
	redirectURI string,
	pkceVerifier string,
	expectedNonce string,
) (identity.OIDCClaims, identity.OIDCTokens, error) {
	c.calls++
	c.provider = provider
	c.clientSecret = clientSecret
	c.code = code
	c.redirectURI = redirectURI
	c.pkceVerifier = pkceVerifier
	c.expectedNonce = expectedNonce
	return c.claims, c.tokens, c.err
}

func TestOIDCAuthorizationLinkUsesStateNoncePKCEAndStoresCredentialsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-link@test.com", "pw", "OIDCLink", false)
	provider := oidcTestProvider(t, db, "oidc-link-provider")
	redis := redisstore.NewMemoryStore()
	box, _ := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	encryptedSecret, _ := box.Encrypt("client-secret")
	provider.ClientSecretCiphertext = encryptedSecret
	if updated, err := db.Identities.UpdateProvider(ctx, provider); err != nil || !updated {
		t.Fatalf("store encrypted provider secret: updated=%v err=%v", updated, err)
	}
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{
			Subject: "subject-1", Email: "remote@example.com", EmailVerified: true,
			DisplayName: "Remote User", AvatarURL: "https://issuer.example/avatar.png",
		},
		tokens: identity.OIDCTokens{
			AccessToken: "access-1", RefreshToken: "refresh-1", TokenType: "Bearer",
			Expiry: expiresAt, Scopes: []string{"openid", "profile"},
		},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: redis, OIDCClient: client}
	actor := actorWithPermissions(user.ID, "external_identity.create.owned")
	started, err := service.StartAuthorization(ctx, actor, provider.ID, identity.AuthorizationIntentLink, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if started.ExpiresIn != 600 {
		t.Fatalf("authorization expiry mismatch: %#v", started)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := authorizationURL.Query()
	state := query.Get("state")
	if authorizationURL.String() == "" || authorizationURL.Scheme != "https" || authorizationURL.Host != "issuer.example" ||
		query.Get("response_type") != "code" || query.Get("client_id") != "client-id" ||
		query.Get("redirect_uri") != "http://localhost:8000/v2/auth/oidc/callback" ||
		query.Get("scope") != "openid profile" || state == "" || query.Get("nonce") == "" ||
		query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" ||
		query.Has("prompt") {
		t.Fatalf("authorization URL mismatch: %s", started.AuthorizationURL)
	}
	storedState, err := redis.GetState(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if storedState["kind"] != "oidc_authorization" || storedState["provider_id"] != provider.ID ||
		storedState["intent"] != "link" || storedState["user_id"] != user.ID ||
		storedState["nonce"] != query.Get("nonce") || storedState["pkce_verifier"] == "" {
		t.Fatalf("stored authorization state mismatch: %#v", storedState)
	}

	completed, err := service.CompleteAuthorization(ctx, "authorization-code", state, "")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Intent != "link" || completed.UserID != user.ID || completed.IdentityID == "" || completed.ProviderID != provider.ID {
		t.Fatalf("authorization completion mismatch: %#v", completed)
	}
	if client.calls != 1 || client.provider.ID != provider.ID || client.clientSecret != "client-secret" ||
		client.code != "authorization-code" || client.redirectURI != "http://localhost:8000/v2/auth/oidc/callback" ||
		client.pkceVerifier != storedState["pkce_verifier"] || client.expectedNonce != storedState["nonce"] {
		t.Fatalf("OIDC client call mismatch: %#v", client)
	}
	linked, err := db.Identities.GetIdentity(ctx, completed.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	if linked == nil || linked.UserID != user.ID || linked.ProviderID != provider.ID || linked.Subject != "subject-1" ||
		linked.Email != "remote@example.com" || !linked.EmailVerified || linked.DisplayName != "Remote User" {
		t.Fatalf("linked identity mismatch: %#v", linked)
	}
	credential, err := db.Identities.GetCredential(ctx, completed.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	refreshToken, err := box.Decrypt(credential.RefreshTokenCiphertext)
	if err != nil || refreshToken != "refresh-1" || strings.Join(credential.GrantedScopes, " ") != "openid profile" {
		t.Fatalf("credential mismatch: %#v decrypted=%q err=%v", credential, refreshToken, err)
	}
	accessToken, err := redis.GetExternalAccessToken(ctx, completed.IdentityID)
	if err != nil || accessToken.AccessToken != "access-1" || accessToken.TokenType != "Bearer" || accessToken.ExpiresAt != expiresAt.UnixMilli() {
		t.Fatalf("cached access token mismatch: %#v err=%v", accessToken, err)
	}
	_, err = service.CompleteAuthorization(ctx, "authorization-code", state, "")
	assertHTTPError(t, err, 400, "oidc_state.verify.invalid")
}

func TestQQAuthorizationUsesStateOnlyCallbackShapeAndStoresEmptyCredentialsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "qq-login@test.com", "pw", "QQLogin", false)
	box, err := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	encryptedSecret, err := box.Encrypt("qq-app-key")
	if err != nil {
		t.Fatal(err)
	}
	provider := model.IdentityProvider{
		ID: "qq-login-provider", Name: "QQ 登录", IssuerURL: identity.QQUIssuerURL,
		AuthorizationEndpoint: identity.QQAuthorizationEndpoint, TokenEndpoint: identity.QQTokenEndpoint,
		UserInfoEndpoint: identity.QQUserInfoEndpoint, ClientID: "client-app-id",
		ClientSecretCiphertext: encryptedSecret, Scopes: []string{"get_user_info"},
		Adapter: identity.AdapterQQ, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: "qq-openid-1", DisplayName: "QQ昵称", AvatarURL: "https://qlogo.example/100.png"},
		tokens: identity.OIDCTokens{Scopes: []string{"get_user_info"}},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, OIDCClient: client}
	existing := model.ExternalIdentity{
		ID: "qq-existing-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "qq-openid-1",
		CreatedAt: 5, UpdatedAt: 5,
	}
	if err := db.Identities.CreateIdentity(ctx, existing, model.ExternalIdentityCredential{IdentityID: existing.ID, GrantedScopes: []string{"get_user_info"}, UpdatedAt: 5}); err != nil {
		t.Fatal(err)
	}

	started, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, identity.AuthorizationIntentLogin, "", "")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := authorizationURL.Query()
	state := query.Get("state")
	if authorizationURL.Scheme != "https" || authorizationURL.Host != "graph.qq.com" ||
		authorizationURL.Path != "/oauth2.0/authorize" ||
		query.Get("response_type") != "code" || query.Get("client_id") != "client-app-id" ||
		query.Get("redirect_uri") != "http://localhost:8000/v2/auth/oidc/callback" ||
		query.Get("scope") != "get_user_info" || state == "" {
		t.Fatalf("qq authorization URL mismatch: %s", started.AuthorizationURL)
	}
	for _, unsupported := range []string{"nonce", "code_challenge", "code_challenge_method", "login_hint", "prompt"} {
		if query.Has(unsupported) {
			t.Fatalf("qq authorization URL must not send %q: %s", unsupported, started.AuthorizationURL)
		}
	}

	completed, err := service.CompleteAuthorization(ctx, "authorization-code", state, "")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Intent != identity.AuthorizationIntentLogin || completed.UserID != user.ID || completed.ProviderID != provider.ID {
		t.Fatalf("qq login completion mismatch: %#v", completed)
	}
	if client.calls != 1 || client.provider.Adapter != identity.AdapterQQ || client.clientSecret != "qq-app-key" ||
		client.redirectURI != "http://localhost:8000/v2/auth/oidc/callback" {
		t.Fatalf("injected client call mismatch: %#v", client)
	}
	linked, err := db.Identities.GetIdentity(ctx, completed.IdentityID)
	if err != nil || linked == nil || linked.ID != existing.ID {
		t.Fatalf("qq linked identity=%#v err=%v", linked, err)
	}
	if linked.Subject != "qq-openid-1" || linked.DisplayName != "QQ昵称" ||
		linked.AvatarURL != "https://qlogo.example/100.png" || linked.Email != "" || linked.LastLoginAt == nil {
		t.Fatalf("qq linked identity claims mismatch: %#v", linked)
	}
	credential, err := db.Identities.GetCredential(ctx, completed.IdentityID)
	if err != nil || credential == nil {
		t.Fatalf("qq credential=%#v err=%v", credential, err)
	}
	if credential.RefreshTokenCiphertext != "" ||
		credential.AuthorizationStatus != model.ExternalIdentityAuthorizationActive ||
		strings.Join(credential.GrantedScopes, ",") != "get_user_info" {
		t.Fatalf("qq credential must be empty but active with granted scopes: %#v", credential)
	}
	if _, err = cache.GetExternalAccessToken(ctx, completed.IdentityID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("qq login must not cache any access token: %v", err)
	}
	if _, err = service.CompleteAuthorization(ctx, "authorization-code", state, ""); err == nil {
		t.Fatal("replayed qq callback state must stay consumed")
	} else {
		assertHTTPError(t, err, 400, "oidc_state.verify.invalid")
	}
}

func TestOIDCLinkRequestsAccountSelectionOnlyFromMicrosoft(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-prompt@test.com", "pw", "OIDCPrompt", false)
	provider := oidcTestProvider(t, db, "oidc-prompt-provider")
	provider.Adapter = identity.AdapterMicrosoft
	updateOIDCTestProvider(t, db, provider)
	service := identity.Service{
		DB: db, Config: testutil.TestConfig(), Redis: redisstore.NewMemoryStore(),
	}

	linked, err := service.StartAuthorization(
		ctx,
		actorWithPermissions(user.ID, "external_identity.create.owned"),
		provider.ID,
		identity.AuthorizationIntentLink,
		"", "")

	if err != nil {
		t.Fatal(err)
	}
	linkedURL, err := url.Parse(linked.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if got := linkedURL.Query().Get("prompt"); got != "select_account" {
		t.Fatalf("Microsoft link prompt=%q want=%q URL=%s", got, "select_account", linked.AuthorizationURL)
	}

	loggedIn, err := service.StartAuthorization(
		ctx,
		permission.GuestActor(),
		provider.ID,
		identity.AuthorizationIntentLogin,
		"", "")

	if err != nil {
		t.Fatal(err)
	}
	loginURL, err := url.Parse(loggedIn.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if loginURL.Query().Has("prompt") {
		t.Fatalf("Microsoft login must not force a prompt: %s", loggedIn.AuthorizationURL)
	}
}

func TestOIDCLoginReturnsOnlyLinkedIdentitiesAndNeverCreatesAccounts(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-login@test.com", "pw", "OIDCLogin", false)
	provider := oidcTestProvider(t, db, "oidc-login-provider")
	redis := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: "existing-subject", Email: "claims@example.com", DisplayName: "Claims"},
		tokens: identity.OIDCTokens{AccessToken: "access-existing", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour), Scopes: []string{"openid"}},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: redis, OIDCClient: client}
	existing := model.ExternalIdentity{
		ID: "existing-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "existing-subject",
		Email: "old@example.com", CreatedAt: 10, UpdatedAt: 10,
	}
	if err := db.Identities.CreateIdentity(ctx, existing, model.ExternalIdentityCredential{IdentityID: existing.ID, UpdatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	returnTo := "/oauth/authorize?client_id=client-1&state=opaque"
	started, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", returnTo)
	if err != nil {
		t.Fatal(err)
	}
	state := mustAuthorizationState(t, started.AuthorizationURL)
	loggedIn, err := service.CompleteAuthorization(ctx, "code-existing", state, "")
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.Intent != "login" || loggedIn.UserID != user.ID || loggedIn.IdentityID != existing.ID || loggedIn.ReturnTo != returnTo {
		t.Fatalf("existing login mismatch: %#v", loggedIn)
	}
	updated, _ := db.Identities.GetIdentity(ctx, existing.ID)
	if updated == nil || updated.Email != "claims@example.com" || updated.DisplayName != "Claims" || updated.LastLoginAt == nil {
		t.Fatalf("existing identity claims were not refreshed: %#v", updated)
	}

	var beforeUsers int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&beforeUsers); err != nil {
		t.Fatal(err)
	}
	client.claims = identity.OIDCClaims{Subject: "new-subject", Email: "suggested@example.com", EmailVerified: true, DisplayName: "Suggested"}
	client.tokens = identity.OIDCTokens{AccessToken: "unlinked-access", RefreshToken: "unlinked-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour), Scopes: []string{"openid", "email"}}
	started, err = service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", returnTo)
	if err != nil {
		t.Fatal(err)
	}
	unlinkedState := mustAuthorizationState(t, started.AuthorizationURL)
	unlinked, err := service.CompleteAuthorization(ctx, "code-new", unlinkedState, "")
	if unlinked != (identity.AuthorizationResult{}) {
		t.Fatalf("unlinked login result=%#v", unlinked)
	}
	assertHTTPError(t, err, 403, "identity.login.not_linked")
	var afterUsers, identities int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&afterUsers); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identities WHERE provider_id=$1 AND subject='new-subject'`, provider.ID).Scan(&identities); err != nil {
		t.Fatal(err)
	}
	if afterUsers != beforeUsers || identities != 0 {
		t.Fatalf("OIDC callback must not create an account: users=%d/%d identities=%d", beforeUsers, afterUsers, identities)
	}
	if _, err := service.CompleteAuthorization(ctx, "code-replay", unlinkedState, ""); err == nil {
		t.Fatal("unlinked callback state must stay consumed")
	} else {
		assertHTTPError(t, err, 400, "oidc_state.verify.invalid")
	}
}

func TestOIDCLinkReauthorizesAnExistingOwnedIdentityThroughTheSamePath(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-reauthorize@test.com", "pw", "OIDCReauthorize", false)
	provider := oidcTestProvider(t, db, "oidc-reauthorize-provider")
	existing := model.ExternalIdentity{
		ID: "reauthorize-identity", UserID: user.ID, ProviderID: provider.ID,
		Subject: "reauthorize-subject", Email: "old@example.com", CreatedAt: 1, UpdatedAt: 1,
	}
	failedAt := int64(1)
	if err := db.Identities.CreateIdentity(ctx, existing, model.ExternalIdentityCredential{
		IdentityID: existing.ID, AuthorizationStatus: model.ExternalIdentityAuthorizationReauthorizationRequired,
		LastRefreshErrorAt: &failedAt, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: existing.Subject, Email: "new@example.com", EmailVerified: true, DisplayName: "Reauthorized"},
		tokens: identity.OIDCTokens{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour), Scopes: []string{"openid", "profile"}},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, OIDCClient: client}
	actor := actorWithPermissions(user.ID, "external_identity.create.owned")
	started, err := service.StartAuthorization(ctx, actor, provider.ID, "link", existing.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	state := mustAuthorizationState(t, started.AuthorizationURL)
	storedState, err := cache.GetState(ctx, state)
	if err != nil || authorizationURL.Query().Get("login_hint") != existing.Email ||
		storedState["target_identity_id"] != existing.ID || storedState["target_subject"] != existing.Subject {
		t.Fatalf("targeted reauthorization state=%#v url=%s err=%v", storedState, started.AuthorizationURL, err)
	}
	result, err := service.CompleteAuthorization(ctx, "reauthorize-code", state, "")
	if err != nil || result.Intent != "link" || result.UserID != user.ID || result.IdentityID != existing.ID || result.ProviderID != provider.ID {
		t.Fatalf("reauthorization result=%#v err=%v", result, err)
	}
	stored, err := db.Identities.GetIdentity(ctx, existing.ID)
	if err != nil || stored == nil || stored.Email != "new@example.com" || !stored.EmailVerified || stored.DisplayName != "Reauthorized" || stored.LastLoginAt == nil {
		t.Fatalf("reauthorized identity=%#v err=%v", stored, err)
	}
	credential, err := db.Identities.GetCredential(ctx, existing.ID)
	if err != nil || credential == nil || credential.AuthorizationStatus != model.ExternalIdentityAuthorizationActive || credential.LastRefreshAt != nil || credential.LastRefreshErrorAt != nil {
		t.Fatalf("reauthorized credential=%#v err=%v", credential, err)
	}
	box, _ := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	refreshToken, err := box.Decrypt(credential.RefreshTokenCiphertext)
	if err != nil || refreshToken != "new-refresh" {
		t.Fatalf("reauthorized refresh token=%q err=%v", refreshToken, err)
	}
	if access, err := cache.GetExternalAccessToken(ctx, existing.ID); err != nil || access.AccessToken != "new-access" {
		t.Fatalf("reauthorized access token=%#v err=%v", access, err)
	}
}

func TestOIDCTargetedReauthorizationRejectsAnotherSelectedAccountWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-target-mismatch@test.com", "pw", "OIDCTargetMismatch", false)
	provider := oidcTestProvider(t, db, "oidc-target-mismatch-provider")
	target := model.ExternalIdentity{
		ID: "target-identity", UserID: user.ID, ProviderID: provider.ID,
		Subject: "target-subject", Email: "target@example.com", CreatedAt: 1, UpdatedAt: 1,
	}
	failedAt := int64(2)
	if err := db.Identities.CreateIdentity(ctx, target, model.ExternalIdentityCredential{
		IdentityID: target.ID, RefreshTokenCiphertext: "original-ciphertext",
		AuthorizationStatus: model.ExternalIdentityAuthorizationReauthorizationRequired,
		LastRefreshErrorAt:  &failedAt, UpdatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: "different-subject", Email: "different@example.com"},
		tokens: identity.OIDCTokens{AccessToken: "different-access", RefreshToken: "different-refresh", Expiry: time.Now().Add(time.Hour)},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, OIDCClient: client}
	actor := actorWithPermissions(user.ID, "external_identity.create.owned")
	other := testutil.CreateUser(t, db, "oidc-target-other@test.com", "pw", "OIDCTargetOther", false)
	_, err := service.StartAuthorization(ctx, actorWithPermissions(other.ID, "external_identity.create.owned"), provider.ID, "link", target.ID, "")
	assertHTTPError(t, err, 404, "identity.resolve.not_found")
	_, err = service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", target.ID, "")
	assertHTTPError(t, err, 400, "identity_id.validate.invalid")
	started, err := service.StartAuthorization(ctx, actor, provider.ID, "link", target.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CompleteAuthorization(ctx, "wrong-account-code", mustAuthorizationState(t, started.AuthorizationURL), "")
	assertHTTPError(t, err, 409, "identity.authorize.mismatch")
	credential, getErr := db.Identities.GetCredential(ctx, target.ID)
	if getErr != nil || credential == nil || credential.RefreshTokenCiphertext != "original-ciphertext" ||
		credential.AuthorizationStatus != model.ExternalIdentityAuthorizationReauthorizationRequired ||
		credential.LastRefreshErrorAt == nil || *credential.LastRefreshErrorAt != failedAt {
		t.Fatalf("mismatched reauthorization credential=%#v err=%v", credential, getErr)
	}
	var identityCount int
	if countErr := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM external_identities WHERE user_id=$1`, user.ID).Scan(&identityCount); countErr != nil || identityCount != 1 {
		t.Fatalf("mismatched reauthorization identities=%d err=%v", identityCount, countErr)
	}
	if _, cacheErr := cache.GetExternalAccessToken(ctx, target.ID); !errors.Is(cacheErr, redisstore.ErrCacheMiss) {
		t.Fatalf("mismatched reauthorization cached token err=%v", cacheErr)
	}
}

func TestOIDCAuthorizationRejectsUnauthorizedLinkAndConsumesDeniedStateExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	provider := oidcTestProvider(t, db, "oidc-denied-provider")
	redis := redisstore.NewMemoryStore()
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: redis, OIDCClient: &fakeOIDCClient{}}
	_, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "link", "", "")
	assertHTTPError(t, err, 403, "permission.check.denied")
	started, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", "")
	if err != nil {
		t.Fatal(err)
	}
	state := mustAuthorizationState(t, started.AuthorizationURL)
	_, err = service.CompleteAuthorization(ctx, "", state, "oauth_authorization.grant.denied")
	assertHTTPError(t, err, 400, "identity.authorize.denied")
	_, err = service.CompleteAuthorization(ctx, "code", state, "")
	assertHTTPError(t, err, 400, "oidc_state.verify.invalid")
	if client := service.OIDCClient.(*fakeOIDCClient); client.calls != 0 {
		t.Fatalf("denied authorization must not exchange a code, calls=%d", client.calls)
	}
}

func TestOIDCAuthorizationStateMachineRejectsUnavailableAndMalformedTransitionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-state-user@test.com", "pw", "OIDCStateUser", false)
	other := testutil.CreateUser(t, db, "oidc-state-other@test.com", "pw", "OIDCStateOther", false)
	provider := oidcTestProvider(t, db, "oidc-state-provider")
	box, err := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.ClientSecretCiphertext, err = box.Encrypt("state-client-secret")
	if err != nil {
		t.Fatal(err)
	}
	updateOIDCTestProvider(t, db, provider)
	cache := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: "state-subject", Email: "state@remote.example"},
		tokens: identity.OIDCTokens{AccessToken: "state-access", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, OIDCClient: client}
	actor := actorWithPermissions(user.ID, "external_identity.create.owned")

	if _, err := service.StartAuthorization(ctx, actor, "missing-provider", "link", "", ""); err == nil {
		t.Fatal("missing provider authorization should fail")
	} else {
		assertHTTPError(t, err, 404, "identity_provider.resolve.not_found")
	}
	if _, err := service.StartAuthorization(ctx, actor, provider.ID, "unknown", "", ""); err == nil {
		t.Fatal("unknown authorization intent should fail")
	} else {
		assertHTTPError(t, err, 400, "authorization_intent.validate.invalid")
	}
	if _, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "unexpected", ""); err == nil {
		t.Fatal("login identity_id should fail")
	} else {
		assertHTTPError(t, err, 400, "identity_id.validate.invalid")
	}
	for _, invalidReturnTo := range []string{"https://attacker.example/callback", "//attacker.example/callback", `/\attacker.example/callback`, "/login?redirect=/oauth/authorize", "dashboard"} {
		if _, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", invalidReturnTo); err == nil {
			t.Fatalf("invalid return target %q should fail", invalidReturnTo)
		} else {
			assertHTTPError(t, err, 400, "return_to.validate.invalid")
		}
	}

	provider.LoginEnabled = false
	updateOIDCTestProvider(t, db, provider)
	if _, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", ""); err == nil {
		t.Fatal("disabled login should fail")
	} else {
		assertHTTPError(t, err, 403, "identity_provider.login.disabled")
	}
	provider.LoginEnabled = true
	provider.LinkEnabled = false
	updateOIDCTestProvider(t, db, provider)
	if _, err := service.StartAuthorization(ctx, actor, provider.ID, "link", "", ""); err == nil {
		t.Fatal("disabled linking should fail")
	} else {
		assertHTTPError(t, err, 403, "identity_provider.link.disabled")
	}
	provider.LinkEnabled = true
	updateOIDCTestProvider(t, db, provider)

	foreign := model.ExternalIdentity{
		ID: "oidc-state-foreign", UserID: other.ID, ProviderID: provider.ID, Subject: "foreign-subject",
		Email: "foreign@remote.example", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateIdentity(ctx, foreign, model.ExternalIdentityCredential{IdentityID: foreign.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAuthorization(ctx, actor, provider.ID, "link", foreign.ID, ""); err == nil {
		t.Fatal("foreign target identity should fail")
	} else {
		assertHTTPError(t, err, 404, "identity.resolve.not_found")
	}
	withoutCache := service
	withoutCache.Redis = nil
	if _, err := withoutCache.StartAuthorization(ctx, actor, provider.ID, "link", "", ""); err == nil || err.Error() != "identity state store is not configured" {
		t.Fatalf("missing state store start error=%v", err)
	}

	if _, err := service.CompleteAuthorization(ctx, "code", " ", ""); err == nil {
		t.Fatal("empty callback state should fail")
	} else {
		assertHTTPError(t, err, 400, "oidc_state.validate.required")
	}
	if _, err := withoutCache.CompleteAuthorization(ctx, "code", "state", ""); err == nil || err.Error() != "identity state store is not configured" {
		t.Fatalf("missing state store completion error=%v", err)
	}
	if _, err := service.CompleteAuthorization(ctx, "code", "missing-state", ""); err == nil {
		t.Fatal("missing callback state should fail")
	} else {
		assertHTTPError(t, err, 400, "oidc_state.verify.invalid")
	}
	if err := cache.SetState(ctx, "wrong-kind", map[string]any{"kind": "registration"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteAuthorization(ctx, "code", "wrong-kind", ""); err == nil {
		t.Fatal("wrong callback state kind should fail")
	} else {
		assertHTTPError(t, err, 400, "oidc_state.verify.invalid")
	}

	started, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteAuthorization(ctx, " ", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil {
		t.Fatal("missing authorization code should fail")
	} else {
		assertHTTPError(t, err, 400, "authorization_code.validate.required")
	}

	client.err = errors.New("identity provider exchange failed")
	started, err = service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteAuthorization(ctx, "code", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil {
		t.Fatal("OIDC exchange failure should fail")
	} else {
		assertHTTPError(t, err, 400, "identity.authorize.failed")
	}
	client.err = nil

	for state, values := range map[string]map[string]any{
		"invalid-intent": {
			"kind": "oidc_authorization", "provider_id": provider.ID, "intent": "invalid",
			"nonce": "nonce", "pkce_verifier": "verifier",
		},
		"missing-link-user": {
			"kind": "oidc_authorization", "provider_id": provider.ID, "intent": "link",
			"nonce": "nonce", "pkce_verifier": "verifier",
		},
	} {
		if err := cache.SetState(ctx, state, values, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.CompleteAuthorization(ctx, "code", "invalid-intent", ""); err == nil {
		t.Fatal("invalid stored intent should fail")
	} else {
		assertHTTPError(t, err, 400, "authorization_intent.validate.invalid")
	}
	client.claims.Subject = "missing-link-user-subject"
	if _, err := service.CompleteAuthorization(ctx, "code", "missing-link-user", ""); err == nil {
		t.Fatal("link state without user should fail")
	} else {
		assertHTTPError(t, err, 403, "permission.check.denied")
	}

	client.claims.Subject = "new-registration-subject"
	started, err = service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login", "", "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	unlinked, err := service.CompleteAuthorization(ctx, "code", mustAuthorizationState(t, started.AuthorizationURL), "")
	if unlinked != (identity.AuthorizationResult{}) {
		t.Fatalf("unmatched identity result=%#v", unlinked)
	}
	assertHTTPError(t, err, 403, "identity.login.not_linked")

	disappearing := provider
	disappearing.ID = "oidc-disappearing-provider"
	disappearing.IssuerURL = "https://disappearing.example"
	disappearing.AuthorizationEndpoint = disappearing.IssuerURL + "/authorize"
	disappearing.TokenEndpoint = disappearing.IssuerURL + "/token"
	disappearing.JWKSURI = disappearing.IssuerURL + "/jwks"
	disappearing.ClientID = "disappearing-client"
	disappearing.LoginEnabled = true
	if err := db.Identities.CreateProvider(ctx, disappearing); err != nil {
		t.Fatal(err)
	}
	started, err = service.StartAuthorization(ctx, permission.GuestActor(), disappearing.ID, "login", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if identityIDs, deleted, err := db.Identities.DeleteProvider(ctx, disappearing.ID); err != nil || !deleted || len(identityIDs) != 0 {
		t.Fatalf("delete disappearing provider identityIDs=%v deleted=%v err=%v", identityIDs, deleted, err)
	}
	if _, err := service.CompleteAuthorization(ctx, "code", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil {
		t.Fatal("removed provider callback should fail")
	} else {
		assertHTTPError(t, err, 400, "identity_provider.use.unavailable")
	}
}

func TestOIDCAuthorizationRejectsDuplicateIdentityLinksAndPropagatesDependencyFailuresExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oidc-reuse-user@test.com", "pw", "OIDCReuseUser", false)
	other := testutil.CreateUser(t, db, "oidc-reuse-other@test.com", "pw", "OIDCReuseOther", false)
	provider := oidcTestProvider(t, db, "oidc-reuse-provider")
	box, err := util.NewSecretBox(testutil.TestConfig().IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	provider.ClientSecretCiphertext, err = box.Encrypt("reuse-secret")
	if err != nil {
		t.Fatal(err)
	}
	updateOIDCTestProvider(t, db, provider)
	for _, item := range []model.ExternalIdentity{
		{ID: "oidc-reuse-owned", UserID: user.ID, ProviderID: provider.ID, Subject: "owned-subject", CreatedAt: 1, UpdatedAt: 1},
		{ID: "oidc-reuse-foreign", UserID: other.ID, ProviderID: provider.ID, Subject: "foreign-subject", CreatedAt: 2, UpdatedAt: 2},
	} {
		if err := db.Identities.CreateIdentity(ctx, item, model.ExternalIdentityCredential{IdentityID: item.ID, UpdatedAt: item.UpdatedAt}); err != nil {
			t.Fatal(err)
		}
	}
	cache := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: "owned-subject", Email: "updated@remote.example"},
		tokens: identity.OIDCTokens{TokenType: "Bearer"},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, OIDCClient: client}
	actor := actorWithPermissions(user.ID, "external_identity.create.owned")

	started, err := service.StartAuthorization(ctx, actor, provider.ID, "link", "", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CompleteAuthorization(ctx, "owned-code", mustAuthorizationState(t, started.AuthorizationURL), "")
	if result != (identity.AuthorizationResult{}) {
		t.Fatalf("duplicate owned identity result=%#v", result)
	}
	assertHTTPError(t, err, 409, "identity.link.already_exists")
	updated, err := db.Identities.GetIdentity(ctx, "oidc-reuse-owned")
	if err != nil || updated == nil || updated.Email != "" || updated.LastLoginAt != nil || updated.UpdatedAt != 1 {
		t.Fatalf("duplicate owned identity mutated state=%#v err=%v", updated, err)
	}
	if credential, err := db.Identities.GetCredential(ctx, updated.ID); err != nil || credential == nil || credential.UpdatedAt != 1 || credential.RefreshTokenCiphertext != "" {
		t.Fatalf("duplicate owned identity mutated credential=%#v err=%v", credential, err)
	}
	if _, err := cache.GetExternalAccessToken(ctx, updated.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("duplicate owned identity cached a token: %v", err)
	}

	client.claims = identity.OIDCClaims{Subject: "foreign-subject"}
	started, err = service.StartAuthorization(ctx, actor, provider.ID, "link", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteAuthorization(ctx, "foreign-code", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil {
		t.Fatal("foreign linked identity should fail")
	} else {
		assertHTTPError(t, err, 409, "identity.link.conflict")
	}
	foreign, err := db.Identities.GetIdentity(ctx, "oidc-reuse-foreign")
	if err != nil || foreign == nil || foreign.UserID != other.ID || foreign.UpdatedAt != 2 || foreign.LastLoginAt != nil {
		t.Fatalf("foreign duplicate link mutated identity=%#v err=%v", foreign, err)
	}

	cache.Err = errors.New("identity state unavailable")
	if _, err := service.StartAuthorization(ctx, actor, provider.ID, "link", "", ""); err == nil || err.Error() != "identity state unavailable" {
		t.Fatalf("state write error=%v", err)
	}
	cache.Err = nil
	started, err = service.StartAuthorization(ctx, actor, provider.ID, "link", "", "")
	if err != nil {
		t.Fatal(err)
	}
	cache.Err = errors.New("identity state read unavailable")
	if _, err := service.CompleteAuthorization(ctx, "code", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil || err.Error() != "identity state read unavailable" {
		t.Fatalf("state read error=%v", err)
	}
	cache.Err = nil

	provider.AuthorizationEndpoint = "://invalid-authorization-endpoint"
	updateOIDCTestProvider(t, db, provider)
	beforeStates := cache.Len()
	if _, err := service.StartAuthorization(ctx, actor, provider.ID, "link", "", ""); err == nil {
		t.Fatal("malformed authorization endpoint should fail")
	}
	if cache.Len() != beforeStates {
		t.Fatalf("malformed authorization endpoint leaked state: before=%d after=%d", beforeStates, cache.Len())
	}
	provider.AuthorizationEndpoint = "https://issuer.example/authorize"
	updateOIDCTestProvider(t, db, provider)

	started, err = service.StartAuthorization(ctx, actor, provider.ID, "link", "", "")
	if err != nil {
		t.Fatal(err)
	}
	invalidConfigService := service
	invalidConfigService.Config.IdentityEncryptionKey = "invalid-key"
	if _, err := invalidConfigService.CompleteAuthorization(ctx, "code", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil || !strings.Contains(err.Error(), "decode identity encryption key") {
		t.Fatalf("invalid encryption configuration error=%v", err)
	}

	provider.ClientSecretCiphertext = "malformed-ciphertext"
	updateOIDCTestProvider(t, db, provider)
	started, err = service.StartAuthorization(ctx, actor, provider.ID, "link", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteAuthorization(ctx, "code", mustAuthorizationState(t, started.AuthorizationURL), ""); err == nil || err.Error() != "unsupported encrypted secret version" {
		t.Fatalf("malformed encrypted client secret error=%v", err)
	}
}

func updateOIDCTestProvider(t *testing.T, db *database.DB, provider model.IdentityProvider) {
	t.Helper()
	updated, err := db.Identities.UpdateProvider(t.Context(), provider)
	if err != nil || !updated {
		t.Fatalf("update OIDC test provider updated=%v err=%v", updated, err)
	}
}

func oidcTestProvider(t *testing.T, db *database.DB, id string) model.IdentityProvider {
	t.Helper()
	provider := model.IdentityProvider{
		ID: id, Name: "Example OIDC", IssuerURL: "https://issuer.example",
		AuthorizationEndpoint: "https://issuer.example/authorize", TokenEndpoint: "https://issuer.example/token",
		JWKSURI: "https://issuer.example/jwks", ClientID: "client-id", Scopes: []string{"openid", "profile"},
		Adapter: identity.AdapterGenericOIDC, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(context.Background(), provider); err != nil {
		t.Fatal(err)
	}
	return provider
}

func actorWithPermissions(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{SubjectID: "user:" + userID, UserID: userID, Permissions: bits}
}

func mustAuthorizationState(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatalf("authorization URL has no state: %s", rawURL)
	}
	return state
}
