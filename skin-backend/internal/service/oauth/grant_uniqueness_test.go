package oauth_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceRepeatedAuthorizationReusesGrantAndPreservesEachCodeScopeExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-repeated-authorization@test.com", "Password123", "OAuthRepeatedAuthorization", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Repeated authorization app",
		RedirectURI:     "https://repeated-authorization.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "profile.read.owned"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	activateOAuthClient(t, db, clientID)

	approve := func(verifier, scope string) string {
		t.Helper()
		approved, approveErr := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
			ResponseType:        "code",
			ClientID:            clientID,
			RedirectURI:         "https://repeated-authorization.example/callback",
			Scope:               scope,
			CodeChallenge:       pkceChallenge(verifier),
			CodeChallengeMethod: "S256",
		})
		if approveErr != nil {
			t.Fatal(approveErr)
		}
		return approved["code"].(string)
	}
	firstVerifier := "repeated-first-verifier-abcdefghijklmnopqrstuvwxyz"
	secondVerifier := "repeated-second-verifier-abcdefghijklmnopqrstuvwxyz"
	firstCode := approve(firstVerifier, "account.read.self")
	secondCode := approve(secondVerifier, "account.read.self profile.read.owned")

	var activeGrants int
	var grantID string
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*), MIN(id)
		FROM delegated_permission_grants
		WHERE user_id=$1 AND client_id=$2 AND status='active'
	`, user.ID, clientID).Scan(&activeGrants, &grantID); err != nil {
		t.Fatal(err)
	}
	if activeGrants != 1 || grantID == "" {
		t.Fatalf("active grant count=%d grant_id=%q want exactly one", activeGrants, grantID)
	}

	exchange := func(code, verifier string) oauth.TokenResponse {
		t.Helper()
		token, exchangeErr := svc.IssueToken(ctx, oauth.TokenRequest{
			GrantType:    "authorization_code",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Code:         code,
			CodeVerifier: verifier,
			RedirectURI:  "https://repeated-authorization.example/callback",
		})
		if exchangeErr != nil {
			t.Fatal(exchangeErr)
		}
		return token
	}
	firstToken := exchange(firstCode, firstVerifier)
	if firstToken.Scope != "account.read.self" || len(firstToken.Permissions) != 1 || firstToken.Permissions[0] != "account.read.self" {
		t.Fatalf("first authorization code scope mismatch: %#v", firstToken)
	}
	secondToken := exchange(secondCode, secondVerifier)
	if secondToken.Scope != "account.read.self profile.read.owned" || len(secondToken.Permissions) != 2 || secondToken.Permissions[0] != "account.read.self" || secondToken.Permissions[1] != "profile.read.owned" {
		t.Fatalf("second authorization code scope mismatch: %#v", secondToken)
	}
	for _, rawRefresh := range []string{firstToken.RefreshToken, secondToken.RefreshToken} {
		var storedGrantID string
		if err := db.Pool.QueryRow(ctx, `SELECT grant_id FROM oauth_refresh_tokens WHERE token_hash=$1`, util.HashRefreshToken(rawRefresh)).Scan(&storedGrantID); err != nil {
			t.Fatal(err)
		}
		if storedGrantID != grantID {
			t.Fatalf("refresh token grant_id=%q want shared %q", storedGrantID, grantID)
		}
	}
}
