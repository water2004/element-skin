package oauth_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceClientCredentialsIssueAppOnlyActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials@test.com", "Password123", "OAuthClientCredentials", false)
	admin := testutil.CreateUser(t, db, "oauth-client-credentials-admin@test.com", "Password123", "OAuthClientCredentialsAdmin", true, true)
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
		Name:            "Server plugin",
		RedirectURI:     "https://server.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public", "minecraft_session.hasjoined.server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	if clientID == "" || clientSecret == "" {
		t.Fatalf("client credentials response missing secret: %#v", clientRes)
	}
	grantClientPermission(t, db, clientID, "minecraft_profile.read.public")
	grantClientPermission(t, db, clientID, "minecraft_session.hasjoined.server")

	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "minecraft_session.hasjoined.server",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken != "" || token.TokenType != "Bearer" || token.ExpiresIn != 3600 ||
		token.Scope != "minecraft_session.hasjoined.server" ||
		len(token.Permissions) != 1 || token.Permissions[0] != "minecraft_session.hasjoined.server" {
		t.Fatalf("client credentials token mismatch: %#v", token)
	}
	appActor, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || appActor.UserID != "" || appActor.SubjectID != permissiondb.SubjectIDForClient(clientID) ||
		appActor.SessionKind != permission.SessionKindClient || appActor.Entrypoint != permission.EntrypointAPI ||
		!appActor.Has(permission.MustDefinitionByCode("minecraft_session.hasjoined.server")) ||
		appActor.Has(permission.MustDefinitionByCode("minecraft_profile.read.public")) {
		t.Fatalf("app-only actor mismatch: ok=%v actor=%#v", ok, appActor)
	}
	introspection, err := svc.Introspect(ctx, adminActor, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if introspection["active"] != true || introspection["client_id"] != clientID ||
		introspection["subject_id"] != permissiondb.SubjectIDForClient(clientID) ||
		introspection["scope"] != "minecraft_session.hasjoined.server" {
		t.Fatalf("client credentials introspection mismatch: %#v", introspection)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, token.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := svc.ActorForBearer(ctx, token.AccessToken); err != nil || ok {
		t.Fatalf("revoked app-only token should not authenticate: ok=%v err=%v", ok, err)
	}

	token, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "minecraft_session.hasjoined.server",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server", "deny"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redis.GetOAuthAccessToken(ctx, util.HashRefreshToken(token.AccessToken)); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("client permission override should remove existing access token exactly, got %v", err)
	}
	appActor, ok, err = svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if ok || appActor.SubjectID != "" {
		t.Fatalf("app-only token with no remaining permission should be inactive: ok=%v actor=%#v", ok, appActor)
	}
	if ok, err := db.OAuth.UpdateClientStatus(ctx, clientID, oauth.StatusDisabled, database.NowMS()); err != nil || !ok {
		t.Fatalf("disable client after token issue: ok=%v err=%v", ok, err)
	}
	appActor, ok, err = svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if ok || appActor.SubjectID != "" {
		t.Fatalf("disabled client token should not authenticate: ok=%v actor=%#v", ok, appActor)
	}
}

func TestServiceClientCredentialsReviewApprovalGrantsRequestedClientScopesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-client-credentials-invite@test.com", "Password123", "OAuthInviteClient", true, true)
	admin := testutil.CreateUser(t, db, "oauth-client-credentials-invite-admin@test.com", "Password123", "OAuthInviteAdmin", true, true)
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
		Name:        "Invite manager",
		RedirectURI: "https://invite-manager.example/callback",
		ClientType:  oauth.ClientTypeConfidential,
		PermissionCodes: []string{
			"account.read.self",
			"invite.read.any",
			"invite.create.any",
			"invite.delete.any",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)

	reviewed, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, "")
	if err != nil {
		t.Fatal(err)
	}
	if reviewed["status"] != oauth.StatusActive {
		t.Fatalf("reviewed client status mismatch: %#v", reviewed)
	}
	var selfOverrideCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM subject_permission_overrides
		WHERE subject_id=$1 AND permission_id=$2
	`, permissiondb.SubjectIDForClient(clientID), int64(permission.MustDefinitionByCode("account.read.self").ID)).Scan(&selfOverrideCount); err != nil {
		t.Fatal(err)
	}
	if selfOverrideCount != 0 {
		t.Fatalf("review should not grant user-context permission to client subject: count=%d", selfOverrideCount)
	}

	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "invite.read.any invite.create.any invite.delete.any",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" ||
		token.RefreshToken != "" ||
		token.TokenType != "Bearer" ||
		token.ExpiresIn != 3600 ||
		token.Scope != "invite.create.any invite.delete.any invite.read.any" ||
		len(token.Permissions) != 3 ||
		token.Permissions[0] != "invite.create.any" ||
		token.Permissions[1] != "invite.delete.any" ||
		token.Permissions[2] != "invite.read.any" {
		t.Fatalf("review-approved invite client token mismatch: %#v", token)
	}
	appActor, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok ||
		appActor.UserID != "" ||
		appActor.SubjectID != permissiondb.SubjectIDForClient(clientID) ||
		appActor.SessionKind != permission.SessionKindClient ||
		appActor.Entrypoint != permission.EntrypointAPI ||
		!appActor.Has(permission.MustDefinitionByCode("invite.read.any")) ||
		!appActor.Has(permission.MustDefinitionByCode("invite.create.any")) ||
		!appActor.Has(permission.MustDefinitionByCode("invite.delete.any")) {
		t.Fatalf("review-approved invite app actor mismatch: ok=%v actor=%#v", ok, appActor)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "account.delete.any",
	})
	assertHTTPError(t, err, 403, "permission.check.denied")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "account.read.self",
	})
	assertHTTPError(t, err, 403, "permission.check.denied")
}

func TestServiceClientCredentialsCannotUseRemovedApplicationPermissionExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-client-limit-owner@test.com", "Password123", "OAuthClientLimitOwner", true, true)
	admin := testutil.CreateUser(t, db, "oauth-client-limit-admin@test.com", "Password123", "OAuthClientLimitAdmin", true, true)
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
		Name:            "Requested permission limit",
		RedirectURI:     "https://client-limit.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"invite.read.any", "invite.create.any"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	if _, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, ""); err != nil {
		t.Fatal(err)
	}
	before, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "invite.create.any invite.read.any",
	})
	if err != nil {
		t.Fatal(err)
	}
	if before.Scope != "invite.create.any invite.read.any" {
		t.Fatalf("initial client credentials scope mismatch: %#v", before)
	}

	if _, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
		Name:            "Requested permission limit",
		RedirectURI:     "https://client-limit.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"invite.read.any"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redis.GetOAuthAccessToken(ctx, util.HashRefreshToken(before.AccessToken)); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("permission limit reduction should remove old app-only access token, got %v", err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "invite.create.any",
	})
	assertHTTPError(t, err, 403, "permission.check.denied")
	kept, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "invite.read.any",
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Scope != "invite.read.any" || len(kept.Permissions) != 1 || kept.Permissions[0] != "invite.read.any" {
		t.Fatalf("remaining client credentials scope mismatch: %#v", kept)
	}
}

func TestServiceClientCredentialsRejectsPublicClientAndExcessScopeExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials-reject@test.com", "Password123", "OAuthClientReject", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	publicClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Public app",
		RedirectURI:     "https://public.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"minecraft_profile.read.public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	activateOAuthClient(t, db, publicClient["client_id"].(string))
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType: "client_credentials",
		ClientID:  publicClient["client_id"].(string),
	})
	assertHTTPError(t, err, 400, "oauth_client.authenticate.unsupported")

	confidential, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Confidential app",
		RedirectURI:     "https://confidential.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	activateOAuthClient(t, db, confidential["client_id"].(string))
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     confidential["client_id"].(string),
		ClientSecret: confidential["client_secret"].(string),
		Scope:        "minecraft_session.hasjoined.server",
	})
	assertHTTPError(t, err, 403, "permission.check.denied")
}

func TestServiceClientCredentialsDefaultScopeAndInactiveClientExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials-default@test.com", "Password123", "OAuthClientDefault", true, true)
	if err := db.Permissions.SetSubjectPermissionOverride(
		ctx,
		user.ID,
		permission.MustDefinitionByCode("minecraft_session.hasjoined.server"),
		"allow",
		"",
	); err != nil {
		t.Fatal(err)
	}
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Default scope app",
		RedirectURI:     "https://default-scope.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public", "minecraft_session.hasjoined.server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	assertHTTPError(t, err, 400, "client_id.verify.invalid")
	activateOAuthClient(t, db, clientID)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	assertHTTPError(t, err, 403, "permission.check.denied")

	grantClientPermission(t, db, clientID, "minecraft_profile.read.public")
	grantClientPermission(t, db, clientID, "minecraft_session.hasjoined.server")
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" ||
		token.RefreshToken != "" ||
		token.TokenType != "Bearer" ||
		token.ExpiresIn != 3600 ||
		token.Scope != "minecraft_profile.read.public minecraft_session.hasjoined.server" ||
		len(token.Permissions) != 2 ||
		token.Permissions[0] != "minecraft_profile.read.public" ||
		token.Permissions[1] != "minecraft_session.hasjoined.server" {
		t.Fatalf("default client credentials token mismatch: %#v", token)
	}
}
