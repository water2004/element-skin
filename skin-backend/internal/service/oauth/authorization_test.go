package oauth_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
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
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         code,
		RedirectURI:  "https://client.example/callback",
		CodeVerifier: verifier,
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
	assertHTTPError(t, err, 400, "invalid refresh_token")
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
	assertHTTPError(t, err, 400, "invalid refresh_token")
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
	assertHTTPError(t, err, 400, "response_type must be code")
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge("invalid-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	})
	assertHTTPError(t, err, 400, "invalid client_id")
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
		{name: "bad client", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.ClientID = "missing-client" }), status: 400, detail: "invalid client_id", actor: actor},
		{name: "bad redirect", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.RedirectURI = "https://client.example/other" }), status: 400, detail: "invalid redirect_uri", actor: actor},
		{name: "missing pkce", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.CodeChallengeMethod = "plain" }), status: 400, detail: "PKCE S256 is required", actor: actor},
		{name: "empty scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "" }), status: 400, detail: "scope is required", actor: actor},
		{name: "invalid scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "permission.catalog.system" }), status: 400, detail: "invalid scope", actor: actor},
		{name: "actor lacks scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "texture.delete.any" }), status: 403, detail: "permission denied", actor: otherActor},
		{name: "client lacks scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "account.update.self" }), status: 400, detail: "scope exceeds client permission limit", actor: actor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.AuthorizationDetails(ctx, tc.actor, tc.req)
			assertHTTPError(t, err, tc.status, tc.detail)
		})
	}
}

func TestServiceClientPermissionAndGrantManagementExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-perms@test.com", "Password123", "OAuthClientPerms", false)
	admin := testutil.CreateUser(t, db, "oauth-client-perms-admin@test.com", "Password123", "OAuthClientPermsAdmin", true, true)
	userActor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, userActor, oauth.ClientInput{
		Name:            "Permission app",
		RedirectURI:     "https://perms.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	if _, err := svc.ClientPermissions(ctx, userActor, clientID); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("client permissions without admin read error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, userActor, clientID, "minecraft_session.hasjoined.server", "allow"); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("client permission set without grant error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "permission.catalog.system", "allow"); !isHTTPError(err, 400, "invalid permission") {
		t.Fatalf("client system permission set error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server", "allow"); err != nil {
		t.Fatal(err)
	}
	perms, err := svc.ClientPermissions(ctx, adminActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if perms["subject_id"] != permissiondb.SubjectIDForClient(clientID) {
		t.Fatalf("client permission subject mismatch: %#v", perms)
	}
	effective := stringSetFromStrings(perms["effective_permissions"].([]string))
	if !effective["minecraft_session.hasjoined.server"] {
		t.Fatalf("effective client permissions missing override: %#v", perms)
	}
	overrides := perms["overrides"].([]map[string]any)
	if len(overrides) != 1 || overrides[0]["permission_code"] != "minecraft_session.hasjoined.server" || overrides[0]["effect"] != "allow" {
		t.Fatalf("client overrides mismatch: %#v", overrides)
	}
	if err := svc.ClearClientPermissionOverride(ctx, userActor, clientID, "minecraft_session.hasjoined.server"); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("client permission clear without revoke error mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server"); !isHTTPError(err, 404, "permission override not found") {
		t.Fatalf("client permission clear missing error mismatch: %#v", err)
	}

	activateOAuthClient(t, db, clientID)
	verifier := "grant-verifier-abcdefghijklmnopqrstuvwxyz"
	approved, err := svc.ApproveAuthorization(ctx, userActor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://perms.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(verifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: created["client_secret"].(string),
		Code:         approved["code"].(string),
		RedirectURI:  "https://perms.example/callback",
		CodeVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken == "" {
		t.Fatalf("token should include refresh token: %#v", token)
	}
	grants, err := svc.ListGrants(ctx, userActor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0]["client_id"] != clientID || grants[0]["status"] != "active" {
		t.Fatalf("grant list mismatch: %#v", grants)
	}
	grantPermissions := grants[0]["permissions"].([]string)
	if len(grantPermissions) != 1 || grantPermissions[0] != "account.read.self" {
		t.Fatalf("grant permissions mismatch: %#v", grantPermissions)
	}
	if err := svc.RevokeGrant(ctx, userActor, grants[0]["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeGrant(ctx, userActor, grants[0]["id"].(string)); !isHTTPError(err, 404, "oauth grant not found") {
		t.Fatalf("second grant revoke error mismatch: %#v", err)
	}
	if _, err := svc.ListGrants(ctx, permission.Actor{}, 10); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("list grants without permission mismatch: %#v", err)
	}
	if err := svc.RevokeGrant(ctx, permission.Actor{}, grants[0]["id"].(string)); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("revoke grant without permission mismatch: %#v", err)
	}
}

func TestServiceOAuthPermissionCodeDependencyErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-code-dependency@test.com", "Password123", "OAuthCodeDependency", false)
	admin := testutil.CreateUser(t, db, "oauth-code-dependency-admin@test.com", "Password123", "OAuthCodeDependencyAdmin", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Permission code dependency app",
		RedirectURI:     "https://code-dependency.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	if ok, err := db.OAuth.UpdateClientStatus(ctx, clientID, oauth.StatusActive, database.NowMS()); err != nil || !ok {
		t.Fatalf("activate client before dependency drop: ok=%v err=%v", ok, err)
	}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE delegated_client_permissions CASCADE`); err != nil {
		t.Fatal(err)
	}

	adminList, err := svc.ListClientsForAdmin(ctx, adminActor, "all", 10)
	if err != nil {
		t.Fatalf("admin lightweight list should not load permission details: %v", err)
	}
	if len(adminList) != 1 || adminList[0]["client_id"] != clientID || adminList[0]["status"] != oauth.StatusActive {
		t.Fatalf("admin lightweight list mismatch after permission table drop: %#v", adminList)
	}
	if _, ok := adminList[0]["permissions"]; ok {
		t.Fatalf("admin lightweight list must not include permissions: %#v", adminList[0])
	}
	if _, ok := adminList[0]["redirect_uri"]; ok {
		t.Fatalf("admin lightweight list must not include redirect_uri: %#v", adminList[0])
	}

	checks := []struct {
		name string
		call func() error
	}{
		{name: "get client", call: func() error {
			_, err := svc.GetClient(ctx, ownerActor, clientID)
			return err
		}},
		{name: "list owned clients", call: func() error {
			_, err := svc.ListClients(ctx, ownerActor, 10)
			return err
		}},
		{name: "update client", call: func() error {
			_, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
				Name:            "Permission code dependency app updated",
				RedirectURI:     "https://code-dependency.example/updated",
				ClientType:      oauth.ClientTypeConfidential,
				PermissionCodes: []string{"account.read.self"},
			}, oauth.StatusActive)
			return err
		}},
		{name: "submit client", call: func() error {
			_, err := svc.SubmitClientForReview(ctx, ownerActor, clientID)
			return err
		}},
		{name: "review client", call: func() error {
			_, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, "")
			return err
		}},
		{name: "rotate secret", call: func() error {
			_, err := svc.RotateClientSecret(ctx, ownerActor, clientID)
			return err
		}},
		{name: "client permissions", call: func() error {
			_, err := svc.ClientPermissions(ctx, adminActor, clientID)
			return err
		}},
		{name: "authorization details", call: func() error {
			_, err := svc.AuthorizationDetails(ctx, ownerActor, oauth.AuthorizationRequest{
				ResponseType:        "code",
				ClientID:            clientID,
				RedirectURI:         "https://code-dependency.example/callback",
				Scope:               "account.read.self",
				CodeChallenge:       pkceChallenge("code-dependency-verifier"),
				CodeChallengeMethod: "S256",
			})
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			assertPgCode(t, tc.call(), "42P01")
		})
	}
}
