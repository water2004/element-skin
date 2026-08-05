package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	identitysvc "element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestOIDCRegistrationRequiresAndConsumesTheCompleteLocalRegistrationForm(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	svc := newAuthServiceWithRedis(db, cfg, redis)
	svc.Identity = identitysvc.Service{DB: db, Config: cfg, Redis: redis}

	for key, value := range map[string]string{
		"email_verify_enabled": "true",
		"require_invite":       "true",
	} {
		if err := db.Settings.Set(ctx, key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "OIDC_INVITE", testutil.Pointer(1), "OIDC registration"); err != nil {
		t.Fatal(err)
	}

	provider := model.IdentityProvider{
		ID:                    "registration-provider",
		Name:                  "Registration Provider",
		IssuerURL:             "https://identity.example",
		AuthorizationEndpoint: "https://identity.example/authorize",
		TokenEndpoint:         "https://identity.example/token",
		JWKSURI:               "https://identity.example/jwks",
		ClientID:              "registration-client",
		Scopes:                []string{"openid", "email", "profile"},
		Adapter:               identitysvc.AdapterGenericOIDC,
		Enabled:               true,
		LoginEnabled:          true,
		RegistrationEnabled:   true,
		CreatedAt:             1,
		UpdatedAt:             1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}

	ticket := "registration-ticket"
	remoteExpiry := time.Now().Add(30 * time.Minute).UnixMilli()
	if err := redis.SetState(ctx, ticket, map[string]any{
		"kind":           "oidc_registration",
		"provider_id":    provider.ID,
		"subject":        "remote-subject",
		"email":          "suggested-by-provider@example.com",
		"email_verified": true,
		"display_name":   "Remote Display Name",
		"avatar_url":     "https://identity.example/avatar.png",
		"access_token":   "short-lived-access-token",
		"refresh_token":  "long-lived-refresh-token",
		"token_type":     "Bearer",
		"expires_at":     remoteExpiry,
		"scopes":         []string{"openid", "email", "profile"},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}

	localEmail := "local-registration@example.com"
	for _, tc := range []struct {
		name     string
		email    string
		password string
		username string
		invite   string
		code     string
		detail   string
	}{
		{name: "username", email: localEmail, password: "Password123", detail: "Username is required"},
		{name: "email", password: "Password123", username: "OIDCLocalUser", detail: "Invalid email format"},
		{name: "password", email: localEmail, username: "OIDCLocalUser", detail: "Password is required"},
		{name: "verification", email: localEmail, password: "Password123", username: "OIDCLocalUser", detail: "Verification code required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := svc.RegisterWithIdentity(ctx, tc.email, tc.password, tc.username, tc.invite, tc.code, ticket)
			if id != "" || !httpError(err, 400, tc.detail) {
				t.Fatalf("RegisterWithIdentity() id=%q err=%#v; want exact %q", id, err, tc.detail)
			}
			if _, err := redis.GetState(ctx, ticket); err != nil {
				t.Fatalf("failed local validation consumed identity ticket: %v", err)
			}
		})
	}

	if err := redis.SetVerificationCode(ctx, localEmail, "register", "OIDC1234", time.Minute); err != nil {
		t.Fatal(err)
	}
	if id, err := svc.RegisterWithIdentity(ctx, localEmail, "Password123", "OIDCLocalUser", "", "oidc1234", ticket); id != "" || !httpError(err, 400, "invite code required") {
		t.Fatalf("missing invite id=%q err=%#v; want exact rejection", id, err)
	}
	if _, err := redis.GetState(ctx, ticket); err != nil {
		t.Fatalf("missing invite consumed identity ticket: %v", err)
	}

	userID, err := svc.RegisterWithIdentity(ctx, localEmail, "Password123", "OIDCLocalUser", "OIDC_INVITE", "oidc1234", ticket)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Users.GetByID(ctx, userID)
	if err != nil || user == nil || user.Email != localEmail || user.DisplayName != "OIDCLocalUser" || !util.VerifyPassword("Password123", user.Password) {
		t.Fatalf("local account fields mismatch: user=%#v err=%v", user, err)
	}
	if user.Email == "suggested-by-provider@example.com" {
		t.Fatal("OIDC claims must not replace explicitly submitted local registration fields")
	}
	identities, err := db.Identities.ListIdentitiesByUser(ctx, userID)
	if err != nil || len(identities) != 1 {
		t.Fatalf("linked identity count mismatch: items=%#v err=%v", identities, err)
	}
	linked := identities[0]
	if linked.ProviderID != provider.ID || linked.Subject != "remote-subject" || linked.Email != "suggested-by-provider@example.com" || !linked.EmailVerified {
		t.Fatalf("linked identity claims mismatch: %#v", linked)
	}
	credential, err := db.Identities.GetCredential(ctx, linked.ID)
	if err != nil || credential == nil || credential.RefreshTokenCiphertext == "" || credential.RefreshTokenCiphertext == "long-lived-refresh-token" {
		t.Fatalf("linked credential must be encrypted: credential=%#v err=%v", credential, err)
	}
	box, err := util.NewSecretBox(cfg.IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	if refreshToken, err := box.Decrypt(credential.RefreshTokenCiphertext); err != nil || refreshToken != "long-lived-refresh-token" {
		t.Fatalf("linked refresh token mismatch: token=%q err=%v", refreshToken, err)
	}
	access, err := redis.GetExternalAccessToken(ctx, linked.ID)
	if err != nil || access.AccessToken != "short-lived-access-token" || access.TokenType != "Bearer" || access.ExpiresAt != remoteExpiry {
		t.Fatalf("external access-token cache mismatch: token=%#v err=%v", access, err)
	}
	invite, err := db.Invites.Get(ctx, "OIDC_INVITE")
	if err != nil || invite == nil || invite.UsedCount != 1 || invite.UsedBy == nil || *invite.UsedBy != localEmail {
		t.Fatalf("OIDC registration invite state mismatch: invite=%#v err=%v", invite, err)
	}
	if _, err := redis.GetVerificationCode(ctx, localEmail, "register"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful OIDC registration must consume verification code, got %v", err)
	}
	if _, err := redis.GetState(ctx, ticket); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful OIDC registration must consume ticket, got %v", err)
	}
	if err := db.Invites.Create(ctx, "OIDC_REPLAY_INVITE", testutil.Pointer(1), "OIDC replay"); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetVerificationCode(ctx, "replay@example.com", "register", "REPLAY12", time.Minute); err != nil {
		t.Fatal(err)
	}
	if id, err := svc.RegisterWithIdentity(ctx, "replay@example.com", "Password123", "OIDCReplayUser", "OIDC_REPLAY_INVITE", "REPLAY12", ticket); id != "" || !httpError(err, 400, "invalid or expired identity_ticket") {
		t.Fatalf("identity ticket replay id=%q err=%#v; want exact rejection", id, err)
	}
	if user, err := db.Users.GetByEmail(ctx, "replay@example.com"); err != nil || user != nil {
		t.Fatalf("ticket replay must not create a user: user=%#v err=%v", user, err)
	}
	replayInvite, err := db.Invites.Get(ctx, "OIDC_REPLAY_INVITE")
	if err != nil || replayInvite == nil || replayInvite.UsedCount != 0 || replayInvite.UsedBy != nil {
		t.Fatalf("ticket replay must not consume invite: invite=%#v err=%v", replayInvite, err)
	}
	if code, err := redis.GetVerificationCode(ctx, "replay@example.com", "register"); err != nil || code != "REPLAY12" {
		t.Fatalf("ticket replay must preserve verification code: code=%q err=%v", code, err)
	}
}
