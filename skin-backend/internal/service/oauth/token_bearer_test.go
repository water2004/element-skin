package oauth_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceTokenErrorBranchesAndBearerInvalidationExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-token-errors@test.com", "Password123", "OAuthTokenErrors", false)
	admin := testutil.CreateUser(t, db, "oauth-token-errors-admin@test.com", "Password123", "OAuthTokenErrorsAdmin", true, true)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	confidential, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Token error app",
		RedirectURI:     "https://token-errors.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := confidential["client_id"].(string)
	clientSecret := confidential["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	other, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Other token app",
		RedirectURI:     "https://other-token.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherID := other["client_id"].(string)
	otherSecret := other["client_secret"].(string)
	activateOAuthClient(t, db, otherID)

	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "password"})
	assertHTTPError(t, err, 400, "grant_type.validate.unsupported")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "client_credentials", ClientID: clientID, ClientSecret: "wrong"})
	assertHTTPError(t, err, 400, "client_secret.verify.invalid")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "authorization_code", ClientID: clientID, ClientSecret: clientSecret, Code: "missing-code", CodeVerifier: "valid-verifier-abcdefghijklmnopqrstuvwxyz", RedirectURI: "https://token-errors.example/callback"})
	assertHTTPError(t, err, 400, "authorization_code.verify.invalid")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "refresh_token", ClientID: clientID, ClientSecret: clientSecret, RefreshToken: "missing-refresh"})
	assertHTTPError(t, err, 400, "refresh_token.verify.invalid")
	for _, tc := range []struct {
		name     string
		verifier string
		detail   string
	}{
		{name: "wrong verifier", verifier: "wrong-verifier-abcdefghijklmnopqrstuvwxyz", detail: "code_verifier.verify.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := "token-verifier-abcdefghijklmnopqrstuvwxyz"
			approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
				ResponseType:        "code",
				ClientID:            clientID,
				RedirectURI:         "https://token-errors.example/callback",
				Scope:               "account.read.self",
				CodeChallenge:       pkceChallenge(verifier),
				CodeChallengeMethod: "S256",
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "authorization_code",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         approved["code"].(string),
				CodeVerifier: tc.verifier,
				RedirectURI:  "https://token-errors.example/callback",
			})
			assertHTTPError(t, err, 400, tc.detail)
			_, err = svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "authorization_code",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         approved["code"].(string),
				CodeVerifier: verifier,
				RedirectURI:  "https://token-errors.example/callback",
			})
			assertHTTPError(t, err, 400, "authorization_code.verify.invalid")
		})
	}
	refreshVerifier := "refresh-revoke-verifier-abcdefghijklmnopqrstuvwxyz"
	approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://token-errors.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(refreshVerifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshToken, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         approved["code"].(string),
		CodeVerifier: refreshVerifier,
		RedirectURI:  "https://token-errors.example/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeToken(ctx, otherID, otherSecret, refreshToken.RefreshToken); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("wrong client refresh revoke error mismatch: %#v", err)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, refreshToken.RefreshToken); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken.RefreshToken,
	})
	assertHTTPError(t, err, 400, "refresh_token.verify.invalid")
	if err := svc.RevokeToken(ctx, clientID, clientSecret, "missing-token"); err != nil {
		t.Fatalf("revoking a missing token should be a no-op: %v", err)
	}
	if _, err := svc.Introspect(ctx, actor, "missing-token"); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("introspect without permission error mismatch: %#v", err)
	}
	inactive, err := svc.Introspect(ctx, adminActor, "missing-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(inactive) != 1 || inactive["active"] != false {
		t.Fatalf("missing token introspection mismatch: %#v", inactive)
	}
	if _, ok, err := svc.ActorForBearer(ctx, "missing-token"); err != nil || ok {
		t.Fatalf("missing bearer should not authenticate: ok=%v err=%v", ok, err)
	}

	now := database.NowMS()
	rawAccess := "raw-access-token"
	token := redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawAccess),
		ClientID:      otherID,
		UserID:        user.ID,
		GrantID:       "grant-1",
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)},
		ExpiresAt:     now + int64(time.Hour/time.Millisecond),
		CreatedAt:     now,
	}
	if err := svc.Redis.SetOAuthAccessToken(ctx, token, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, rawAccess); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("wrong client revoke error mismatch: %#v", err)
	}
	if err := svc.RevokeToken(ctx, otherID, otherSecret, rawAccess); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := svc.ActorForBearer(ctx, rawAccess); err != nil || ok {
		t.Fatalf("revoked wrong-client test token should not authenticate: ok=%v err=%v", ok, err)
	}

	expiredRaw := "expired-access-token"
	expiredToken := redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(expiredRaw),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("minecraft_session.hasjoined.server").ID)},
		ExpiresAt:     now - 1000,
		CreatedAt:     now - 2000,
	}
	if err := svc.Redis.SetOAuthAccessToken(ctx, expiredToken, time.Hour); err != nil {
		t.Fatal(err)
	}
	introspection, err := svc.Introspect(ctx, adminActor, expiredRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(introspection) != 1 || introspection["active"] != false {
		t.Fatalf("expired introspection mismatch: %#v", introspection)
	}
	if _, ok, err := svc.ActorForBearer(ctx, expiredRaw); err != nil || ok {
		t.Fatalf("expired bearer should not authenticate: ok=%v err=%v", ok, err)
	}
}
