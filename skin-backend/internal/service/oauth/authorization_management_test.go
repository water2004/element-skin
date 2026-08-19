package oauth_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

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
	if _, err := svc.ClientPermissions(ctx, userActor, clientID); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("client permissions without admin read error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, userActor, clientID, "minecraft_session.hasjoined.server", "allow"); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("client permission set without grant error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "permission.catalog.system", "allow"); !isHTTPError(err, 400, "permission.validate.invalid") {
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
	if err := svc.ClearClientPermissionOverride(ctx, userActor, clientID, "minecraft_session.hasjoined.server"); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("client permission clear without revoke error mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server"); !isHTTPError(err, 404, "permission_override.resolve.not_found") {
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
	if err := svc.RevokeGrant(ctx, userActor, grants[0]["id"].(string)); !isHTTPError(err, 404, "oauth_grant.resolve.not_found") {
		t.Fatalf("second grant revoke error mismatch: %#v", err)
	}
	if _, err := svc.ListGrants(ctx, permission.Actor{}, 10); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("list grants without permission mismatch: %#v", err)
	}
	if err := svc.RevokeGrant(ctx, permission.Actor{}, grants[0]["id"].(string)); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("revoke grant without permission mismatch: %#v", err)
	}
}
