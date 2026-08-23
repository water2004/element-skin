package identity_test

import (
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
)

func TestOIDCRegistrationTicketLifecycleRejectsInvalidStateAndRestoresExactPayload(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := t.Context()
	provider := oidcTestProvider(t, db, "oidc-registration-provider")
	cache := redisstore.NewMemoryStore()
	service := identity.Service{DB: db, Config: testutil.TestConfig(), Redis: cache}

	if _, err := service.ConsumeRegistration(ctx, " "); err == nil {
		t.Fatal("empty registration ticket should fail")
	} else {
		assertHTTPError(t, err, 400, "identity_ticket.validate.required")
	}
	withoutCache := service
	withoutCache.Redis = nil
	if _, err := withoutCache.ConsumeRegistration(ctx, "ticket"); err == nil || err.Error() != "identity state store is not configured" {
		t.Fatalf("missing registration state store error=%v", err)
	}
	if _, err := service.ConsumeRegistration(ctx, "missing-ticket"); err == nil {
		t.Fatal("missing registration ticket should fail")
	} else {
		assertHTTPError(t, err, 400, "identity_ticket.verify.invalid")
	}
	setRegistrationState(t, cache, "wrong-kind", map[string]any{"kind": "oidc_authorization"})
	if _, err := service.ConsumeRegistration(ctx, "wrong-kind"); err == nil {
		t.Fatal("wrong registration state kind should fail")
	} else {
		assertHTTPError(t, err, 400, "identity_ticket.verify.invalid")
	}

	provider.Enabled = false
	updateOIDCTestProvider(t, db, provider)
	setRegistrationState(t, cache, "disabled-provider", registrationState(provider.ID, "disabled-subject"))
	if _, err := service.ConsumeRegistration(ctx, "disabled-provider"); err == nil {
		t.Fatal("disabled registration provider should fail")
	} else {
		assertHTTPError(t, err, 403, "registration.create.disabled")
	}
	provider.Enabled = true
	provider.LoginEnabled = false
	updateOIDCTestProvider(t, db, provider)
	setRegistrationState(t, cache, "registration-disabled", registrationState(provider.ID, "disabled-registration-subject"))
	if _, err := service.ConsumeRegistration(ctx, "registration-disabled"); err == nil {
		t.Fatal("provider with registration disabled should fail")
	} else {
		assertHTTPError(t, err, 403, "registration.create.disabled")
	}
	provider.LoginEnabled = true
	updateOIDCTestProvider(t, db, provider)

	setRegistrationState(t, cache, "missing-subject", registrationState(provider.ID, ""))
	if _, err := service.ConsumeRegistration(ctx, "missing-subject"); err == nil {
		t.Fatal("registration without subject should fail")
	} else {
		assertHTTPError(t, err, 400, "identity_ticket.verify.invalid")
	}
	user := testutil.CreateUser(t, db, "registration-existing@test.com", "pw", "RegistrationExisting", false)
	existing := model.ExternalIdentity{
		ID: "registration-existing-identity", UserID: user.ID, ProviderID: provider.ID,
		Subject: "existing-subject", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateIdentity(ctx, existing, model.ExternalIdentityCredential{IdentityID: existing.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	setRegistrationState(t, cache, "existing-subject", registrationState(provider.ID, existing.Subject))
	if _, err := service.ConsumeRegistration(ctx, "existing-subject"); err == nil {
		t.Fatal("already linked registration subject should fail")
	} else {
		assertHTTPError(t, err, 409, "identity.link.already_exists")
	}

	expiresAt := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	valid := registrationState(provider.ID, "new-subject")
	valid["email"] = "new@remote.example"
	valid["email_verified"] = true
	valid["display_name"] = "New Remote User"
	valid["avatar_url"] = "https://images.example/new.png"
	valid["access_token"] = "registration-access"
	valid["refresh_token"] = "registration-refresh"
	valid["token_type"] = "Bearer"
	valid["expires_at"] = strconv.FormatInt(expiresAt.UnixMilli(), 10)
	valid["scopes"] = []any{"openid", 42, "email"}
	setRegistrationState(t, cache, "valid-ticket", valid)
	pending, err := service.ConsumeRegistration(ctx, "valid-ticket")
	if err != nil {
		t.Fatal(err)
	}
	if pending.Ticket != "valid-ticket" || pending.Provider.ID != provider.ID || pending.Claims.Subject != "new-subject" ||
		pending.Claims.Email != "new@remote.example" || !pending.Claims.EmailVerified || pending.Claims.DisplayName != "New Remote User" ||
		pending.Claims.AvatarURL != "https://images.example/new.png" || pending.Tokens.AccessToken != "registration-access" ||
		pending.Tokens.RefreshToken != "registration-refresh" || pending.Tokens.TokenType != "Bearer" ||
		!pending.Tokens.Expiry.Equal(expiresAt) || !reflect.DeepEqual(pending.Tokens.Scopes, []string{"openid", "email"}) {
		t.Fatalf("pending registration mismatch: %#v", pending)
	}
	if _, err := service.ConsumeRegistration(ctx, "valid-ticket"); err == nil {
		t.Fatal("consumed registration ticket should not be reusable")
	} else {
		assertHTTPError(t, err, 400, "identity_ticket.verify.invalid")
	}
	if err := service.RestoreRegistration(ctx, pending); err != nil {
		t.Fatal(err)
	}
	restored, err := service.ConsumeRegistration(ctx, pending.Ticket)
	if err != nil || restored.Ticket != pending.Ticket || restored.Claims != pending.Claims ||
		restored.Tokens.AccessToken != pending.Tokens.AccessToken || restored.Tokens.RefreshToken != pending.Tokens.RefreshToken ||
		!restored.Tokens.Expiry.Equal(pending.Tokens.Expiry) || !reflect.DeepEqual(restored.Tokens.Scopes, pending.Tokens.Scopes) {
		t.Fatalf("restored registration=%#v err=%v", restored, err)
	}

	identityRecord, credential, err := service.RegistrationRecords(user.ID, restored)
	if err != nil || identityRecord.ID == "" || identityRecord.UserID != user.ID || identityRecord.ProviderID != provider.ID ||
		identityRecord.Subject != restored.Claims.Subject || identityRecord.LastLoginAt == nil ||
		credential.IdentityID != identityRecord.ID || credential.RefreshTokenCiphertext == "" ||
		credential.AuthorizationStatus != model.ExternalIdentityAuthorizationActive ||
		!reflect.DeepEqual(credential.GrantedScopes, restored.Tokens.Scopes) {
		t.Fatalf("registration records identity=%#v credential=%#v err=%v", identityRecord, credential, err)
	}
	if err := service.CacheRegistrationAccess(ctx, identityRecord.ID, restored.Tokens); err != nil {
		t.Fatal(err)
	}
	cached, err := cache.GetExternalAccessToken(ctx, identityRecord.ID)
	if err != nil || cached.IdentityID != identityRecord.ID || cached.AccessToken != restored.Tokens.AccessToken ||
		cached.TokenType != restored.Tokens.TokenType || cached.ExpiresAt != expiresAt.UnixMilli() {
		t.Fatalf("registration access cache=%#v err=%v", cached, err)
	}
	if _, err := cache.GetState(ctx, pending.Ticket); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("restored ticket should be consumed: err=%v", err)
	}
}

func registrationState(providerID, subject string) map[string]any {
	return map[string]any{
		"kind": "oidc_registration", "provider_id": providerID, "subject": subject,
		"email_verified": false, "expires_at": int64(0), "scopes": []string{"openid"},
	}
}

func setRegistrationState(t *testing.T, cache redisstore.Store, ticket string, state map[string]any) {
	t.Helper()
	if err := cache.SetState(t.Context(), ticket, state, time.Minute); err != nil {
		t.Fatal(err)
	}
}
