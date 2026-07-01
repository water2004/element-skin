package oauth_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceOAuthClosedDatabasePropagatesExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-db-closed@test.com", "Password123", "OAuthDBClosed", false)
	admin := testutil.CreateUser(t, db, "oauth-db-closed-admin@test.com", "Password123", "OAuthDBClosedAdmin", true, true)
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
		Name:            "Closed database app",
		RedirectURI:     "https://closed-db.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	rawUserAccess := "closed-db-user-access"
	if err := svc.Redis.SetOAuthAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawUserAccess),
		ClientID:      clientID,
		UserID:        user.ID,
		GrantID:       "closed-db-grant",
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)},
		ExpiresAt:     database.NowMS() + int64(time.Hour/time.Millisecond),
		CreatedAt:     database.NowMS(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	rawClientAccess := "closed-db-client-access"
	if err := svc.Redis.SetOAuthAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawClientAccess),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("minecraft_session.hasjoined.server").ID)},
		ExpiresAt:     database.NowMS() + int64(time.Hour/time.Millisecond),
		CreatedAt:     database.NowMS(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}

	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "create client", call: func() error {
			_, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
				Name:            "Closed database create",
				RedirectURI:     "https://closed-db.example/create",
				ClientType:      oauth.ClientTypePublic,
				PermissionCodes: []string{"account.read.self"},
			})
			return err
		}},
		{name: "list owned clients", call: func() error {
			_, err := svc.ListClients(ctx, actor, 10)
			return err
		}},
		{name: "list admin clients", call: func() error {
			_, err := svc.ListClientsForAdmin(ctx, adminActor, oauth.StatusActive, 10)
			return err
		}},
		{name: "get client", call: func() error {
			_, err := svc.GetClient(ctx, actor, clientID)
			return err
		}},
		{name: "update client", call: func() error {
			_, err := svc.UpdateClient(ctx, actor, clientID, oauth.ClientInput{
				Name:            "Closed database update",
				RedirectURI:     "https://closed-db.example/update",
				ClientType:      oauth.ClientTypePublic,
				PermissionCodes: []string{"account.read.self"},
			}, "")
			return err
		}},
		{name: "submit client", call: func() error {
			_, err := svc.SubmitClientForReview(ctx, actor, clientID)
			return err
		}},
		{name: "review client", call: func() error {
			_, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, "")
			return err
		}},
		{name: "rotate secret", call: func() error {
			_, err := svc.RotateClientSecret(ctx, actor, clientID)
			return err
		}},
		{name: "delete client", call: func() error {
			return svc.DeleteClient(ctx, actor, clientID)
		}},
		{name: "client permissions", call: func() error {
			_, err := svc.ClientPermissions(ctx, adminActor, clientID)
			return err
		}},
		{name: "list grants", call: func() error {
			_, err := svc.ListGrants(ctx, actor, 10)
			return err
		}},
		{name: "authorization details", call: func() error {
			_, err := svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
				ResponseType:        "code",
				ClientID:            clientID,
				RedirectURI:         "https://closed-db.example/callback",
				Scope:               "account.read.self",
				CodeChallenge:       pkceChallenge("closed-db-verifier"),
				CodeChallengeMethod: "S256",
			})
			return err
		}},
		{name: "start device authorization", call: func() error {
			_, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Scope:        "account.read.self",
			})
			return err
		}},
		{name: "device details", call: func() error {
			_, err := svc.DeviceAuthorizationDetails(ctx, actor, "ABCD-EFGH")
			return err
		}},
		{name: "device decision", call: func() error {
			return svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: "ABCD-EFGH", Approve: true})
		}},
		{name: "revoke token", call: func() error {
			return svc.RevokeToken(ctx, clientID, clientSecret, "any-token")
		}},
		{name: "client credentials token", call: func() error {
			_, err := svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "client_credentials",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Scope:        "minecraft_session.hasjoined.server",
			})
			return err
		}},
		{name: "user bearer actor", call: func() error {
			_, ok, err := svc.ActorForBearer(ctx, rawUserAccess)
			if ok {
				t.Fatal("closed database must not authenticate user bearer")
			}
			return err
		}},
		{name: "client bearer actor", call: func() error {
			_, ok, err := svc.ActorForBearer(ctx, rawClientAccess)
			if ok {
				t.Fatal("closed database must not authenticate client bearer")
			}
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			assertClosedPoolError(t, tc.call())
		})
	}
}
