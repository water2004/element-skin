package oauth_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

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
		name      string
		call      func() error
		assertErr func(*testing.T, error)
	}{
		{name: "get client", call: func() error {
			_, err := svc.GetClient(ctx, ownerActor, clientID)
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
		{name: "list owned clients", call: func() error {
			_, err := svc.ListClients(ctx, ownerActor, 10)
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
		{name: "update client", call: func() error {
			_, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
				Name:            "Permission code dependency app updated",
				RedirectURI:     "https://code-dependency.example/updated",
				ClientType:      oauth.ClientTypeConfidential,
				PermissionCodes: []string{"account.read.self"},
			}, oauth.StatusActive)
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
		{name: "submit client", call: func() error {
			_, err := svc.SubmitClientForReview(ctx, ownerActor, clientID)
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
		{name: "review client", call: func() error {
			_, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, "")
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
		{name: "rotate secret", call: func() error {
			_, err := svc.RotateClientSecret(ctx, ownerActor, clientID)
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
		{name: "client permissions", call: func() error {
			_, err := svc.ClientPermissions(ctx, adminActor, clientID)
			return err
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
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
		}, assertErr: func(t *testing.T, err error) {
			assertPgCode(t, err, "42P01")
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			tc.assertErr(t, tc.call())
		})
	}
	client, err := db.OAuth.GetClient(ctx, clientID)
	if err != nil || client == nil || client.Status != oauth.StatusActive {
		t.Fatalf("failed dependency operations changed client status: client=%#v err=%v", client, err)
	}
}
