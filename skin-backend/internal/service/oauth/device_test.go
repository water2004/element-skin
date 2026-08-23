package oauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

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
	assertHTTPError(t, err, 400, "oauth_authorization.grant.incomplete")

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
	assertHTTPError(t, err, 400, "oauth_grant.exchange.invalid")
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
	otherClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Other device app",
		RedirectURI:     "https://other-device.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherClientID := otherClient["client_id"].(string)
	activateOAuthClient(t, db, otherClientID)
	_, err = svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.update.self",
	})
	assertHTTPError(t, err, 400, "oauth_scope.authorize.denied")
	serverClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Device server denied",
		RedirectURI:     "https://device-server.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_session.hasjoined.server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	serverClientID := serverClient["client_id"].(string)
	serverClientSecret := serverClient["client_secret"].(string)
	activateOAuthClient(t, db, serverClientID)
	_, err = svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID:     serverClientID,
		ClientSecret: serverClientSecret,
		Scope:        "minecraft_session.hasjoined.server",
	})
	assertHTTPError(t, err, 400, "oauth_scope.validate.invalid")
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.read.self",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   otherClientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "device_code.verify.invalid")
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: false}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "oauth_authorization.grant.denied")
}

func TestServiceDeviceAuthorizationRejectsMissingExpiredAndRepeatedDecisionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device-errors@test.com", "Password123", "OAuthDeviceErrors", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Device errors app",
		RedirectURI:     "https://device-errors.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{ClientID: clientID, Scope: "account.read.self"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.DeviceAuthorizationDetails(ctx, permission.Actor{}, started.UserCode); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("anonymous device detail error mismatch: %#v", err)
	}
	if _, err := svc.DeviceAuthorizationDetails(ctx, actor, "missing-code"); !isHTTPError(err, 404, "device_code.resolve.not_found") {
		t.Fatalf("missing device detail error mismatch: %#v", err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: "missing-code", Approve: true}); !isHTTPError(err, 404, "device_code.resolve.not_found") {
		t.Fatalf("missing device decision error mismatch: %#v", err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: false}); !isHTTPError(err, 400, "device_code.authorize.conflict") {
		t.Fatalf("repeated device decision error mismatch: %#v", err)
	}

	expired := modelDeviceCode(t, db, clientID, "expired-device-hash", "expired-user-code", []string{"account.read.self"}, -time.Minute)
	if _, err := svc.DeviceAuthorizationDetails(ctx, actor, expired); !isHTTPError(err, 404, "device_code.resolve.not_found") {
		t.Fatalf("expired device detail error mismatch: %#v", err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: expired, Approve: true}); !isHTTPError(err, 404, "device_code.resolve.not_found") {
		t.Fatalf("expired device decision error mismatch: %#v", err)
	}

	expiredDeviceToken := "expired-device-token"
	modelDeviceCode(t, db, clientID, util.HashRefreshToken(expiredDeviceToken), "expired-token-code", []string{"account.read.self"}, -time.Minute)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: expiredDeviceToken,
	})
	assertHTTPError(t, err, 400, "device_code.verify.expired")

}

func TestServiceOAuthAdditionalTokenAndDeviceEdgesExactly(t *testing.T) {
	t.Run("device status branches and disabled client", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-device-extra@test.com", "Password123", "OAuthDeviceExtra", false)
		admin := testutil.CreateUser(t, db, "oauth-device-extra-admin@test.com", "Password123", "OAuthDeviceExtraAdmin", true, true)
		userActor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		svc := newOAuthService(db)
		created, err := svc.CreateClient(ctx, adminActor, oauth.ClientInput{
			Name:            "Extra device app",
			RedirectURI:     "https://device-extra.example/callback",
			ClientType:      oauth.ClientTypePublic,
			PermissionCodes: []string{"account.ban.any"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		activateOAuthClient(t, db, clientID)

		if _, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{ClientID: clientID}); !isHTTPError(err, 400, "oauth_scope.validate.required") {
			t.Fatalf("empty device scope error mismatch: %#v", err)
		}
		scopeDenied, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{ClientID: clientID, Scope: "account.ban.any"})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.DecideDeviceAuthorization(ctx, userActor, oauth.DeviceDecisionRequest{UserCode: scopeDenied.UserCode, Approve: true}); !isHTTPError(err, 403, "permission.check.denied") {
			t.Fatalf("device decision missing scope mismatch: %#v", err)
		}

		consumedRaw := "consumed-device-token"
		consumedUserCode := modelDeviceCode(t, db, clientID, util.HashRefreshToken(consumedRaw), "consumed-code", []string{"account.ban.any"}, time.Minute)
		if _, err := db.Pool.Exec(ctx, `UPDATE oauth_device_codes SET status='consumed' WHERE user_code_hash=$1`, util.HashRefreshToken(consumedUserCode)); err != nil {
			t.Fatal(err)
		}
		_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "urn:ietf:params:oauth:grant-type:device_code", ClientID: clientID, DeviceCode: consumedRaw})
		assertHTTPError(t, err, 400, "oauth_grant.exchange.invalid")

		unboundRaw := "approved-without-user-device-token"
		unboundUserCode := modelDeviceCode(t, db, clientID, util.HashRefreshToken(unboundRaw), "approved-unbound-code", []string{"account.ban.any"}, time.Minute)
		if _, err := db.Pool.Exec(ctx, `UPDATE oauth_device_codes SET status='approved' WHERE user_code_hash=$1`, util.HashRefreshToken(unboundUserCode)); err != nil {
			t.Fatal(err)
		}
		_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "urn:ietf:params:oauth:grant-type:device_code", ClientID: clientID, DeviceCode: unboundRaw})
		assertHTTPError(t, err, 400, "oauth_grant.exchange.invalid")

		detail, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{ClientID: clientID, Scope: "account.ban.any"})
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := db.OAuth.UpdateClientStatus(ctx, clientID, oauth.StatusDisabled, database.NowMS()); err != nil || !ok {
			t.Fatalf("disable client for device details: ok=%v err=%v", ok, err)
		}
		if _, err := svc.DeviceAuthorizationDetails(ctx, adminActor, detail.UserCode); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
			t.Fatalf("disabled client device details mismatch: %#v", err)
		}
	})

	t.Run("access revoke delete failure and disabled app bearer", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-token-extra@test.com", "Password123", "OAuthTokenExtra", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		healthy := newOAuthService(db)
		created, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
			Name:            "Extra token app",
			RedirectURI:     "https://token-extra.example/callback",
			ClientType:      oauth.ClientTypeConfidential,
			PermissionCodes: []string{"minecraft_profile.read.public"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		clientSecret := created["client_secret"].(string)
		activateOAuthClient(t, db, clientID)
		grantClientPermission(t, db, clientID, "minecraft_profile.read.public")
		token, err := healthy.IssueToken(ctx, oauth.TokenRequest{
			GrantType:    "client_credentials",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scope:        "minecraft_profile.read.public",
		})
		if err != nil {
			t.Fatal(err)
		}
		failingDelete := &oauthAccessDeleteFailStore{Store: healthy.Redis, err: errors.New("delete oauth access failed")}
		failingSvc := oauth.Service{DB: db, Redis: failingDelete}
		if err := failingSvc.RevokeToken(ctx, clientID, clientSecret, token.AccessToken); !errors.Is(err, failingDelete.err) {
			t.Fatalf("access delete failure mismatch: %v", err)
		}
		if failingDelete.deletedHash != util.HashRefreshToken(token.AccessToken) {
			t.Fatalf("delete hash=%q want exact token hash", failingDelete.deletedHash)
		}
		if ok, err := db.OAuth.UpdateClientStatus(ctx, clientID, oauth.StatusDisabled, database.NowMS()); err != nil || !ok {
			t.Fatalf("disable client for app-only bearer: ok=%v err=%v", ok, err)
		}
		if actor, ok, err := healthy.ActorForBearer(ctx, token.AccessToken); err != nil || ok || actor.SubjectID != "" {
			t.Fatalf("disabled app-only bearer actor=%#v ok=%v err=%v; want unauthenticated", actor, ok, err)
		}
	})

	t.Run("refresh token permission dependency failure", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-refresh-dependency@test.com", "Password123", "OAuthRefreshDependency", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		svc := newOAuthService(db)
		created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
			Name:            "Refresh dependency app",
			RedirectURI:     "https://refresh-dependency.example/callback",
			ClientType:      oauth.ClientTypeConfidential,
			PermissionCodes: []string{"account.read.self"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		clientSecret := created["client_secret"].(string)
		activateOAuthClient(t, db, clientID)
		verifier := "refresh-dependency-verifier"
		approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
			ResponseType:        "code",
			ClientID:            clientID,
			RedirectURI:         "https://refresh-dependency.example/callback",
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
			ClientSecret: clientSecret,
			Code:         approved["code"].(string),
			RedirectURI:  "https://refresh-dependency.example/callback",
			CodeVerifier: verifier,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `DROP TABLE delegated_grant_permissions CASCADE`); err != nil {
			t.Fatal(err)
		}
		_, err = svc.IssueToken(ctx, oauth.TokenRequest{
			GrantType:    "refresh_token",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RefreshToken: token.RefreshToken,
		})
		assertPgCode(t, err, "42P01")
	})
}
