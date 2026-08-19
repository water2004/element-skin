package oauth_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServiceOAuthRevokedGrantRejectsAuthorizationCodeAndRefreshExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-revoked-grant-token@test.com", "Password123", "OAuthRevokedGrantToken", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Revoked grant token app",
		RedirectURI:     "https://revoked-grant-token.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	activateOAuthClient(t, db, clientID)

	blockedVerifier := "revoked-grant-code-verifier"
	blockedApproval, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://revoked-grant-token.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(blockedVerifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	grants, err := svc.ListGrants(ctx, actor, 10)
	if err != nil {
		t.Fatal(err)
	}
	blockedGrantID := grants[0]["id"].(string)
	if err := svc.RevokeGrant(ctx, actor, blockedGrantID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         blockedApproval["code"].(string),
		CodeVerifier: blockedVerifier,
		RedirectURI:  "https://revoked-grant-token.example/callback",
	})
	assertHTTPError(t, err, 400, "authorization_code.verify.invalid")

	allowedVerifier := "active-grant-refresh-verifier"
	allowedApproval, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://revoked-grant-token.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(allowedVerifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         allowedApproval["code"].(string),
		CodeVerifier: allowedVerifier,
		RedirectURI:  "https://revoked-grant-token.example/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	grants, err = svc.ListGrants(ctx, actor, 10)
	if err != nil {
		t.Fatal(err)
	}
	activeGrantID := grants[0]["id"].(string)
	if err := svc.RevokeGrant(ctx, actor, activeGrantID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: token.RefreshToken,
	})
	assertHTTPError(t, err, 400, "refresh_token.verify.invalid")
	var activeRefreshCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM oauth_refresh_tokens
		WHERE client_id=$1 AND user_id=$2 AND revoked_at IS NULL
	`, clientID, user.ID).Scan(&activeRefreshCount); err != nil {
		t.Fatal(err)
	}
	if activeRefreshCount != 0 {
		t.Fatalf("revoked grant should revoke its refresh token, active refresh count=%d want 0", activeRefreshCount)
	}
}
