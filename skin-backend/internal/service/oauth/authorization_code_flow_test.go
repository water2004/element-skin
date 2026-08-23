package oauth_test

import (
	"context"
	"strings"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServiceAuthorizationCodeFlowNarrowsActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-service@test.com", "Password123", "OAuthService", false)
	admin := testutil.CreateUser(t, db, "oauth-service-admin@test.com", "Password123", "OAuthServiceAdmin", true, true)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)

	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Service app",
		Description:     "Service app description",
		RedirectURI:     "https://client.example/callback",
		WebsiteURL:      "https://client.example",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "account.update.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	if clientID == "" || clientSecret == "" {
		t.Fatalf("client response should include exact credentials: %#v", clientRes)
	}
	permissions := clientRes["permissions"].([]string)
	if len(permissions) != 2 || permissions[0] != "account.read.self" || permissions[1] != "account.update.self" {
		t.Fatalf("client permissions mismatch: %#v", permissions)
	}

	verifier := "service-verifier-abcdefghijklmnopqrstuvwxyz"
	challenge := pkceChallenge(verifier)
	details, err := svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		State:               "state-service",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	if details.RedirectURI != "https://client.example/callback" || details.State != "state-service" || len(details.Scopes) != 1 {
		t.Fatalf("authorization details mismatch: %#v", details)
	}
	approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		State:               "state-service",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	code := approved["code"].(string)
	redirectURL := approved["redirect_url"].(string)
	if code == "" || !strings.Contains(redirectURL, "state=state-service") {
		t.Fatalf("approve response mismatch: %#v", approved)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         code,
		CodeVerifier: verifier,
		RedirectURI:  "https://client.example/wrong-callback",
	})
	assertHTTPError(t, err, 400, "authorization_code.verify.invalid")
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         code,
		CodeVerifier: verifier,
		RedirectURI:  "https://client.example/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.TokenType != "Bearer" || token.Scope != "account.read.self" ||
		len(token.Permissions) != 1 || token.Permissions[0] != "account.read.self" {
		t.Fatalf("token response mismatch: %#v", token)
	}
	delegated, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || delegated.UserID != user.ID || !delegated.Has(permission.MustDefinitionByCode("account.read.self")) || delegated.Has(permission.MustDefinitionByCode("account.update.self")) {
		t.Fatalf("delegated actor mismatch: ok=%v actor=%#v", ok, delegated)
	}
	introspection, err := svc.Introspect(ctx, adminActor, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if introspection["active"] != true || introspection["client_id"] != clientID || introspection["user_id"] != user.ID ||
		introspection["grant_id"] == "" || introspection["scope"] != "account.read.self" {
		t.Fatalf("introspection mismatch: %#v", introspection)
	}
	refreshed, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: token.RefreshToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken == "" || refreshed.AccessToken == token.AccessToken || refreshed.RefreshToken == "" ||
		refreshed.RefreshToken == token.RefreshToken || refreshed.Scope != "account.read.self" {
		t.Fatalf("refresh response mismatch: %#v", refreshed)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: token.RefreshToken,
	})
	assertHTTPError(t, err, 400, "refresh_token.verify.invalid")
	if err := svc.RevokeToken(ctx, clientID, clientSecret, refreshed.AccessToken); err != nil {
		t.Fatal(err)
	}
	inactive, err := svc.Introspect(ctx, adminActor, refreshed.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(inactive) != 1 || inactive["active"] != false {
		t.Fatalf("revoked access introspection mismatch: %#v", inactive)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, refreshed.RefreshToken); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshed.RefreshToken,
	})
	assertHTTPError(t, err, 400, "refresh_token.verify.invalid")
}

func TestServiceRejectsInvalidAuthorizationRequestExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-invalid@test.com", "Password123", "OAuthInvalid", false)
	other := testutil.CreateUser(t, db, "oauth-invalid-other@test.com", "Password123", "OAuthInvalidOther", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := db.Permissions.ActorForUser(ctx, other.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	client, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Invalid app",
		RedirectURI:     "https://client.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := client["client_id"].(string)
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{ResponseType: "token"})
	assertHTTPError(t, err, 400, "response_type.validate.unsupported")
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge("invalid-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	})
	assertHTTPError(t, err, 400, "client_id.verify.invalid")
	activateOAuthClient(t, db, clientID)
	baseReq := oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge("invalid-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	}
	for _, tc := range []struct {
		name   string
		req    oauth.AuthorizationRequest
		status int
		detail string
		actor  permission.Actor
	}{
		{name: "bad client", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.ClientID = "missing-client" }), status: 400, detail: "client_id.verify.invalid", actor: actor},
		{name: "bad redirect", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.RedirectURI = "https://client.example/other" }), status: 400, detail: "redirect_uri.validate.invalid", actor: actor},
		{name: "missing pkce", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.CodeChallengeMethod = "plain" }), status: 400, detail: "pkce.authorize.required", actor: actor},
		{name: "empty scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "" }), status: 400, detail: "oauth_scope.validate.required", actor: actor},
		{name: "oauth_scope.validate.invalid", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "permission.catalog.system" }), status: 400, detail: "oauth_scope.validate.invalid", actor: actor},
		{name: "actor lacks scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "texture.delete.any" }), status: 403, detail: "permission.check.denied", actor: otherActor},
		{name: "client lacks scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "account.update.self" }), status: 400, detail: "oauth_scope.authorize.denied", actor: actor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.AuthorizationDetails(ctx, tc.actor, tc.req)
			assertHTTPError(t, err, tc.status, tc.detail)
		})
	}

	serverClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Delegated server denied",
		RedirectURI:     "https://server-delegated.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_session.hasjoined.server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	serverClientID := serverClient["client_id"].(string)
	activateOAuthClient(t, db, serverClientID)
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            serverClientID,
		RedirectURI:         "https://server-delegated.example/callback",
		Scope:               "minecraft_session.hasjoined.server",
		CodeChallenge:       pkceChallenge("server-denied-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	})
	assertHTTPError(t, err, 400, "oauth_scope.validate.invalid")
}
