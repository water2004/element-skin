package oauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceAuthorizationCodeFlowNarrowsActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-service@test.com", "Password123", "OAuthService", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
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
}

func TestServiceRejectsInvalidAuthorizationRequestExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-invalid@test.com", "Password123", "OAuthInvalid", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	_, err = svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Invalid app",
		RedirectURI:     "https://client.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{ResponseType: "token"})
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 400 || httpErr.Detail != "response_type must be code" {
		t.Fatalf("invalid response_type error mismatch: err=%#v", err)
	}
}

func TestServiceClientCredentialsIssueAppOnlyActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials@test.com", "Password123", "OAuthClientCredentials", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Server plugin",
		RedirectURI:     "https://server.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public"},
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

	if err := db.Permissions.SetPermissionOverrideForSubject(
		ctx,
		permissiondb.SubjectIDForClient(clientID),
		permission.MustDefinitionByCode("minecraft_session.hasjoined.server"),
		"deny",
		"",
	); err != nil {
		t.Fatal(err)
	}
	appActor, ok, err = svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || appActor.Has(permission.MustDefinitionByCode("minecraft_session.hasjoined.server")) {
		t.Fatalf("app-only actor should be narrowed after permission revoke: ok=%v actor=%#v", ok, appActor)
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
	assertHTTPError(t, err, 400, "client_credentials requires a confidential client")

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
	assertHTTPError(t, err, 403, "permission denied")
}

func TestServiceDeviceCodeFlowIssuesDelegatedTokenExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device-flow@test.com", "Password123", "OAuthDeviceFlow", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Device app",
		RedirectURI:     "https://device.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.read.self",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.DeviceCode == "" || started.UserCode == "" || started.ExpiresIn != 600 || started.Interval != 5 ||
		started.Scope != "account.read.self" || len(started.Permissions) != 1 || started.Permissions[0] != "account.read.self" {
		t.Fatalf("device authorization response mismatch: %#v", started)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "authorization_pending")

	details, err := svc.DeviceAuthorizationDetails(ctx, actor, started.UserCode)
	if err != nil {
		t.Fatal(err)
	}
	if details.Status != "pending" || details.Client["client_id"] != clientID || len(details.Scopes) != 1 {
		t.Fatalf("device details mismatch: %#v", details)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: true}); err != nil {
		t.Fatal(err)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.Scope != "account.read.self" ||
		len(token.Permissions) != 1 || token.Permissions[0] != "account.read.self" {
		t.Fatalf("device token response mismatch: %#v", token)
	}
	delegated, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || delegated.UserID != user.ID || !delegated.Has(permission.MustDefinitionByCode("account.read.self")) {
		t.Fatalf("device delegated actor mismatch: ok=%v actor=%#v", ok, delegated)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "invalid_grant")
}

func TestServiceDeviceCodeFlowRejectsDeniedAndUnauthorizedScopesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device-deny@test.com", "Password123", "OAuthDeviceDeny", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Denied device app",
		RedirectURI:     "https://device.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	_, err = svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.update.self",
	})
	assertHTTPError(t, err, 400, "scope exceeds client permission limit")
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.read.self",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: false}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "access_denied")
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func newOAuthService(db *database.DB) oauth.Service {
	return oauth.Service{DB: db, Redis: redisstore.NewMemoryStore()}
}

func grantClientPermission(t *testing.T, db *database.DB, clientID, code string) {
	t.Helper()
	def := permission.MustDefinitionByCode(code)
	if err := db.Permissions.SetPermissionOverrideForSubject(context.Background(), permissiondb.SubjectIDForClient(clientID), def, "allow", ""); err != nil {
		t.Fatal(err)
	}
}

func activateOAuthClient(t *testing.T, db *database.DB, clientID string) {
	t.Helper()
	if ok, err := db.OAuth.UpdateClientStatus(context.Background(), clientID, oauth.StatusActive, database.NowMS()); err != nil || !ok {
		t.Fatalf("activate oauth client: ok=%v err=%v", ok, err)
	}
}

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Detail != detail {
		t.Fatalf("HTTP error mismatch: err=%#v want status=%d detail=%q", err, status, detail)
	}
}
