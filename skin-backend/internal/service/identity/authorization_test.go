package identity_test

import (
	"context"
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
	started, err := service.StartAuthorization(ctx, actor, provider.ID, identity.AuthorizationIntentLink)
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
		query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
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
	assertHTTPError(t, err, 400, "invalid or expired OIDC state")
}

func TestOIDCLoginReturnsExistingUserOrRegistrationTicketWithoutCreatingAccount(t *testing.T) {
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
	started, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login")
	if err != nil {
		t.Fatal(err)
	}
	state := mustAuthorizationState(t, started.AuthorizationURL)
	loggedIn, err := service.CompleteAuthorization(ctx, "code-existing", state, "")
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.Intent != "login" || loggedIn.UserID != user.ID || loggedIn.IdentityID != existing.ID {
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
	client.tokens = identity.OIDCTokens{AccessToken: "registration-access", RefreshToken: "registration-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour), Scopes: []string{"openid", "email"}}
	started, err = service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login")
	if err != nil {
		t.Fatal(err)
	}
	registration, err := service.CompleteAuthorization(ctx, "code-new", mustAuthorizationState(t, started.AuthorizationURL), "")
	if err != nil {
		t.Fatal(err)
	}
	if registration.Intent != "registration" || registration.RegistrationTicket == "" || registration.UserID != "" || registration.IdentityID != "" {
		t.Fatalf("registration handoff mismatch: %#v", registration)
	}
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
	pending, err := service.ConsumeRegistration(ctx, registration.RegistrationTicket)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Claims.Subject != "new-subject" || pending.Claims.Email != "suggested@example.com" ||
		pending.Tokens.RefreshToken != "registration-refresh" || pending.Provider.ID != provider.ID {
		t.Fatalf("pending registration mismatch: %#v", pending)
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
	if err := db.Identities.CreateIdentity(ctx, existing, model.ExternalIdentityCredential{IdentityID: existing.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	client := &fakeOIDCClient{
		claims: identity.OIDCClaims{Subject: existing.Subject, Email: "new@example.com", EmailVerified: true, DisplayName: "Reauthorized"},
		tokens: identity.OIDCTokens{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour), Scopes: []string{"openid", "profile"}},
	}
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache, OIDCClient: client}
	actor := actorWithPermissions(user.ID, "external_identity.create.owned")
	started, err := service.StartAuthorization(ctx, actor, provider.ID, "link")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CompleteAuthorization(ctx, "reauthorize-code", mustAuthorizationState(t, started.AuthorizationURL), "")
	if err != nil || result.Intent != "link" || result.UserID != user.ID || result.IdentityID != existing.ID || result.ProviderID != provider.ID {
		t.Fatalf("reauthorization result=%#v err=%v", result, err)
	}
	stored, err := db.Identities.GetIdentity(ctx, existing.ID)
	if err != nil || stored == nil || stored.Email != "new@example.com" || !stored.EmailVerified || stored.DisplayName != "Reauthorized" || stored.LastLoginAt == nil {
		t.Fatalf("reauthorized identity=%#v err=%v", stored, err)
	}
	credential, err := db.Identities.GetCredential(ctx, existing.ID)
	if err != nil || credential == nil {
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

func TestOIDCAuthorizationRejectsUnauthorizedLinkAndConsumesDeniedStateExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	provider := oidcTestProvider(t, db, "oidc-denied-provider")
	redis := redisstore.NewMemoryStore()
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: redis, OIDCClient: &fakeOIDCClient{}}
	_, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "link")
	assertHTTPError(t, err, 403, "permission denied")
	started, err := service.StartAuthorization(ctx, permission.GuestActor(), provider.ID, "login")
	if err != nil {
		t.Fatal(err)
	}
	state := mustAuthorizationState(t, started.AuthorizationURL)
	_, err = service.CompleteAuthorization(ctx, "", state, "access_denied")
	assertHTTPError(t, err, 400, "OIDC authorization was denied")
	_, err = service.CompleteAuthorization(ctx, "code", state, "")
	assertHTTPError(t, err, 400, "invalid or expired OIDC state")
	if client := service.OIDCClient.(*fakeOIDCClient); client.calls != 0 {
		t.Fatalf("denied authorization must not exchange a code, calls=%d", client.calls)
	}
}

func oidcTestProvider(t *testing.T, db *database.DB, id string) model.IdentityProvider {
	t.Helper()
	provider := model.IdentityProvider{
		ID: id, Name: "Example OIDC", IssuerURL: "https://issuer.example",
		AuthorizationEndpoint: "https://issuer.example/authorize", TokenEndpoint: "https://issuer.example/token",
		JWKSURI: "https://issuer.example/jwks", ClientID: "client-id", Scopes: []string{"openid", "profile"},
		Adapter: identity.AdapterGenericOIDC, Enabled: true, LoginEnabled: true, LinkEnabled: true,
		RegistrationEnabled: true, CreatedAt: 1, UpdatedAt: 1,
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
