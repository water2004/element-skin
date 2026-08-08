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

type delegatedCredentialFixture struct {
	clientID     string
	clientSecret string
	grantID      string
	accessToken  string
	refreshToken string
}

func TestServiceClientNonActiveTransitionsRevokeAuthorizationsAndCredentialsExactly(t *testing.T) {
	for _, tc := range []struct {
		name              string
		transition        func(context.Context, oauth.Service, permission.Actor, permission.Actor, string) error
		wantRefreshDetail string
	}{
		{
			name: "administrator rejects client",
			transition: func(ctx context.Context, svc oauth.Service, _ permission.Actor, admin permission.Actor, clientID string) error {
				_, err := svc.ReviewClient(ctx, admin, clientID, oauth.StatusRejected, "review rejected")
				return err
			},
			wantRefreshDetail: "invalid client_id",
		},
		{
			name: "administrator disables client",
			transition: func(ctx context.Context, svc oauth.Service, _ permission.Actor, admin permission.Actor, clientID string) error {
				_, err := svc.ReviewClient(ctx, admin, clientID, oauth.StatusDisabled, "security incident")
				return err
			},
			wantRefreshDetail: "invalid client_id",
		},
		{
			name: "developer submits active client for review",
			transition: func(ctx context.Context, svc oauth.Service, owner permission.Actor, _ permission.Actor, clientID string) error {
				_, err := svc.SubmitClientForReview(ctx, owner, clientID)
				return err
			},
			wantRefreshDetail: "invalid client_id",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestAppTB(t)
			ctx := context.Background()
			owner := testutil.CreateUser(t, db, "client-transition-owner@test.com", "Password123", "ClientTransitionOwner", false)
			admin := testutil.CreateUser(t, db, "client-transition-admin@test.com", "Password123", "ClientTransitionAdmin", true, true)
			ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
			if err != nil {
				t.Fatal(err)
			}
			adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
			if err != nil {
				t.Fatal(err)
			}
			svc := newOAuthService(db)
			credential := issueDelegatedCredential(t, ctx, db, svc, ownerActor, "Transition app", "https://transition.example/callback", "account.read.self")
			createPendingAuthorization(t, ctx, svc, ownerActor, credential.clientID, "https://transition.example/callback", "account.read.self")

			if err := tc.transition(ctx, svc, ownerActor, adminActor, credential.clientID); err != nil {
				t.Fatal(err)
			}
			assertClientCredentialRows(t, ctx, db, credential.clientID, 0, 1, 0, 0)
			assertAccessTokenMissing(t, ctx, svc, credential.accessToken)
			if _, ok, err := svc.ActorForBearer(ctx, credential.accessToken); err != nil || ok {
				t.Fatalf("invalidated access token should not authenticate: ok=%v err=%v", ok, err)
			}
			introspection, err := svc.Introspect(ctx, adminActor, credential.accessToken)
			if err != nil {
				t.Fatal(err)
			}
			if len(introspection) != 1 || introspection["active"] != false {
				t.Fatalf("invalidated access token introspection mismatch: %#v", introspection)
			}
			_, err = svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "refresh_token",
				ClientID:     credential.clientID,
				ClientSecret: credential.clientSecret,
				RefreshToken: credential.refreshToken,
			})
			assertHTTPError(t, err, 400, tc.wantRefreshDetail)
		})
	}
}

func TestServiceSecretRotationInvalidatesCredentialsButPreservesAuthorizationsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "secret-rotation-owner@test.com", "Password123", "SecretRotationOwner", false)
	admin := testutil.CreateUser(t, db, "secret-rotation-admin@test.com", "Password123", "SecretRotationAdmin", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	credential := issueDelegatedCredential(t, ctx, db, svc, ownerActor, "Rotation app", "https://rotation.example/callback", "account.read.self")
	createPendingAuthorization(t, ctx, svc, ownerActor, credential.clientID, "https://rotation.example/callback", "account.read.self")

	rotated, err := svc.RotateClientSecret(ctx, ownerActor, credential.clientID)
	if err != nil {
		t.Fatal(err)
	}
	newSecret := rotated["client_secret"].(string)
	if newSecret == "" || newSecret == credential.clientSecret {
		t.Fatalf("rotated secret mismatch: old=%q new=%q", credential.clientSecret, newSecret)
	}
	assertClientCredentialRows(t, ctx, db, credential.clientID, 1, 0, 0, 0)
	assertAccessTokenMissing(t, ctx, svc, credential.accessToken)
	introspection, err := svc.Introspect(ctx, adminActor, credential.accessToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(introspection) != 1 || introspection["active"] != false {
		t.Fatalf("rotated access token introspection mismatch: %#v", introspection)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     credential.clientID,
		ClientSecret: newSecret,
		RefreshToken: credential.refreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     credential.clientID,
		ClientSecret: credential.clientSecret,
		Scope:        "account.read.self",
	})
	assertHTTPError(t, err, 400, "invalid client_secret")
}

func TestServiceGrantRevocationInvalidatesOnlyThatGrantCredentialsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "grant-revoke-owner@test.com", "Password123", "GrantRevokeOwner", false)
	other := testutil.CreateUser(t, db, "grant-revoke-other@test.com", "Password123", "GrantRevokeOther", false)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := db.Permissions.ActorForUser(ctx, other.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	first := issueDelegatedCredential(t, ctx, db, svc, ownerActor, "Grant revoke app", "https://grant-revoke.example/callback", "account.read.self")
	second := issueDelegatedCredentialForClient(t, ctx, svc, otherActor, first.clientID, first.clientSecret, "https://grant-revoke.example/callback", "account.read.self")

	if err := svc.RevokeGrant(ctx, ownerActor, first.grantID); err != nil {
		t.Fatal(err)
	}
	assertGrantCredentialRows(t, ctx, db, first.grantID, "revoked", 0, 0)
	assertGrantCredentialRows(t, ctx, db, second.grantID, "active", 1, 1)
	assertAccessTokenMissing(t, ctx, svc, first.accessToken)
	if _, ok, err := svc.ActorForBearer(ctx, second.accessToken); err != nil || !ok {
		t.Fatalf("unrelated grant access token should remain active: ok=%v err=%v", ok, err)
	}
}

func TestServiceClientPermissionReductionRevokesOnlyIncompatibleGrantExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "permission-reduction-owner@test.com", "Password123", "PermissionReductionOwner", false)
	other := testutil.CreateUser(t, db, "permission-reduction-other@test.com", "Password123", "PermissionReductionOther", false)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := db.Permissions.ActorForUser(ctx, other.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Permission reduction app",
		RedirectURI:     "https://permission-reduction.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "account.update.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	readCredential := issueDelegatedCredentialForClient(t, ctx, svc, ownerActor, clientID, clientSecret, "https://permission-reduction.example/callback", "account.read.self")
	updateCredential := issueDelegatedCredentialForClient(t, ctx, svc, otherActor, clientID, clientSecret, "https://permission-reduction.example/callback", "account.update.self")

	updated, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
		Name:            "Permission reduction app",
		RedirectURI:     "https://permission-reduction.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, oauth.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if updated["status"] != oauth.StatusActive {
		t.Fatalf("permission reduction should preserve active client status: %#v", updated)
	}
	assertGrantCredentialRows(t, ctx, db, readCredential.grantID, "active", 1, 1)
	assertGrantCredentialRows(t, ctx, db, updateCredential.grantID, "revoked", 0, 0)
	assertAccessTokenMissing(t, ctx, svc, readCredential.accessToken)
	assertAccessTokenMissing(t, ctx, svc, updateCredential.accessToken)

	refreshed, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: readCredential.refreshToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.Scope != "account.read.self" {
		t.Fatalf("compatible grant refresh mismatch: %#v", refreshed)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: updateCredential.refreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
}

func TestServiceClientMetadataUpdatePreservesCredentialsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "metadata-update-owner@test.com", "Password123", "MetadataUpdateOwner", false)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	credential := issueDelegatedCredential(t, ctx, db, svc, ownerActor, "Metadata app", "https://metadata.example/callback", "account.read.self")
	createPendingAuthorization(t, ctx, svc, ownerActor, credential.clientID, "https://metadata.example/callback", "account.read.self")

	updated, err := svc.UpdateClient(ctx, ownerActor, credential.clientID, oauth.ClientInput{
		Name:            "Renamed metadata app",
		Description:     "Updated description",
		RedirectURI:     "https://metadata.example/callback",
		WebsiteURL:      "https://metadata.example",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, oauth.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Renamed metadata app" || updated["description"] != "Updated description" || updated["website_url"] != "https://metadata.example" {
		t.Fatalf("metadata update response mismatch: %#v", updated)
	}
	assertClientCredentialRows(t, ctx, db, credential.clientID, 1, 0, 1, 2)
	if _, ok, err := svc.ActorForBearer(ctx, credential.accessToken); err != nil || !ok {
		t.Fatalf("metadata update must preserve access token: ok=%v err=%v", ok, err)
	}
	refreshed, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     credential.clientID,
		ClientSecret: credential.clientSecret,
		RefreshToken: credential.refreshToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.Scope != "account.read.self" {
		t.Fatalf("metadata update refresh mismatch: %#v", refreshed)
	}
}

func TestServiceClientRedirectUpdateInvalidatesCredentialsButPreservesGrantExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "redirect-update-owner@test.com", "Password123", "RedirectUpdateOwner", false)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	credential := issueDelegatedCredential(t, ctx, db, svc, ownerActor, "Redirect app", "https://redirect.example/callback", "account.read.self")
	createPendingAuthorization(t, ctx, svc, ownerActor, credential.clientID, "https://redirect.example/callback", "account.read.self")

	updated, err := svc.UpdateClient(ctx, ownerActor, credential.clientID, oauth.ClientInput{
		Name:            "Redirect app",
		RedirectURI:     "https://redirect.example/new-callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, oauth.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if updated["redirect_uri"] != "https://redirect.example/new-callback" || updated["status"] != oauth.StatusActive {
		t.Fatalf("redirect update response mismatch: %#v", updated)
	}
	assertClientCredentialRows(t, ctx, db, credential.clientID, 1, 0, 0, 0)
	assertAccessTokenMissing(t, ctx, svc, credential.accessToken)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     credential.clientID,
		ClientSecret: credential.clientSecret,
		RefreshToken: credential.refreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
}

func issueDelegatedCredential(t *testing.T, ctx context.Context, db *database.DB, svc oauth.Service, actor permission.Actor, name, redirectURI, scope string) delegatedCredentialFixture {
	t.Helper()
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            name,
		RedirectURI:     redirectURI,
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{scope},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	return issueDelegatedCredentialForClient(t, ctx, svc, actor, clientID, created["client_secret"].(string), redirectURI, scope)
}

func issueDelegatedCredentialForClient(t *testing.T, ctx context.Context, svc oauth.Service, actor permission.Actor, clientID, clientSecret, redirectURI, scope string) delegatedCredentialFixture {
	t.Helper()
	verifier := "credential-lifecycle-verifier-abcdefghijklmnopqrstuvwxyz"
	approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
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
		CodeVerifier: verifier,
		RedirectURI:  redirectURI,
	})
	if err != nil {
		t.Fatal(err)
	}
	var grantID string
	if err := svc.DB.Pool.QueryRow(ctx, `
		SELECT grant_id FROM oauth_refresh_tokens WHERE token_hash=$1
	`, util.HashRefreshToken(token.RefreshToken)).Scan(&grantID); err != nil {
		t.Fatal(err)
	}
	return delegatedCredentialFixture{
		clientID:     clientID,
		clientSecret: clientSecret,
		grantID:      grantID,
		accessToken:  token.AccessToken,
		refreshToken: token.RefreshToken,
	}
}

func createPendingAuthorization(t *testing.T, ctx context.Context, svc oauth.Service, actor permission.Actor, clientID, redirectURI, scope string) {
	t.Helper()
	if _, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		CodeChallenge:       pkceChallenge("pending-credential-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	}); err != nil {
		t.Fatal(err)
	}
}

func assertClientCredentialRows(t *testing.T, ctx context.Context, db *database.DB, clientID string, activeGrants, revokedGrants, activeRefresh, authorizationCodes int) {
	t.Helper()
	var gotActiveGrants, gotRevokedGrants, gotActiveRefresh, gotAuthorizationCodes int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='active'),
			COUNT(*) FILTER (WHERE status='revoked')
		FROM delegated_permission_grants
		WHERE client_id=$1
	`, clientID).Scan(&gotActiveGrants, &gotRevokedGrants); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oauth_refresh_tokens WHERE client_id=$1 AND revoked_at IS NULL
	`, clientID).Scan(&gotActiveRefresh); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oauth_authorization_codes WHERE client_id=$1
	`, clientID).Scan(&gotAuthorizationCodes); err != nil {
		t.Fatal(err)
	}
	if gotActiveGrants != activeGrants || gotRevokedGrants != revokedGrants || gotActiveRefresh != activeRefresh || gotAuthorizationCodes != authorizationCodes {
		t.Fatalf(
			"client credential rows mismatch: active_grants=%d revoked_grants=%d active_refresh=%d codes=%d want %d/%d/%d/%d",
			gotActiveGrants,
			gotRevokedGrants,
			gotActiveRefresh,
			gotAuthorizationCodes,
			activeGrants,
			revokedGrants,
			activeRefresh,
			authorizationCodes,
		)
	}
}

func assertGrantCredentialRows(t *testing.T, ctx context.Context, db *database.DB, grantID, status string, activeRefresh, authorizationCodes int) {
	t.Helper()
	var gotStatus string
	if err := db.Pool.QueryRow(ctx, `SELECT status FROM delegated_permission_grants WHERE id=$1`, grantID).Scan(&gotStatus); err != nil {
		t.Fatal(err)
	}
	var gotActiveRefresh, gotAuthorizationCodes int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oauth_refresh_tokens WHERE grant_id=$1 AND revoked_at IS NULL
	`, grantID).Scan(&gotActiveRefresh); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oauth_authorization_codes WHERE grant_id=$1
	`, grantID).Scan(&gotAuthorizationCodes); err != nil {
		t.Fatal(err)
	}
	if gotStatus != status || gotActiveRefresh != activeRefresh || gotAuthorizationCodes != authorizationCodes {
		t.Fatalf(
			"grant credential rows mismatch: status=%q active_refresh=%d codes=%d want %q/%d/%d",
			gotStatus,
			gotActiveRefresh,
			gotAuthorizationCodes,
			status,
			activeRefresh,
			authorizationCodes,
		)
	}
}

func assertAccessTokenMissing(t *testing.T, ctx context.Context, svc oauth.Service, rawToken string) {
	t.Helper()
	_, err := svc.Redis.GetOAuthAccessToken(ctx, util.HashRefreshToken(rawToken))
	if !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("oauth access token should be deleted, got %v", err)
	}
}
