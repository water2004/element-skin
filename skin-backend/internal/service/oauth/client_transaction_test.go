package oauth_test

import (
	"context"
	"errors"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceClientMutationFailureBoundariesExactly(t *testing.T) {
	t.Run("cache failure before update preserves database state", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-update-cache-fail@test.com", "Password123", "OAuthUpdateCacheFail", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		healthy := newOAuthService(db)
		created, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
			Name: "Before cache failure", RedirectURI: "https://cache-fail.example/callback",
			ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.self"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		forced := errors.New("forced client cache failure")
		failing := oauth.Service{DB: db, Redis: &oauthClientAccessDeleteFailStore{Store: healthy.Redis, err: forced}}
		_, err = failing.UpdateClient(ctx, actor, clientID, oauth.ClientInput{
			Name: "After cache failure", RedirectURI: "https://cache-fail.example/changed",
			ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.self"},
		})

		if !errors.Is(err, forced) {
			t.Fatalf("update cache failure=%v want=%v", err, forced)
		}
		stored, err := db.OAuth.GetClient(ctx, clientID)
		if err != nil || stored == nil || stored.Name != "Before cache failure" ||
			stored.RedirectURI != "https://cache-fail.example/callback" || stored.Status != oauth.StatusPending {
			t.Fatalf("cache failure changed client: client=%#v err=%v", stored, err)
		}
	})

	t.Run("post-commit cache failure does not report a rolled-back update", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-update-postcommit@test.com", "Password123", "OAuthUpdatePostCommit", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		healthy := newOAuthService(db)
		created, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
			Name: "Before post commit", RedirectURI: "https://post-commit.example/callback",
			ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.self"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		cache := &oauthClientAccessDeleteNthFailStore{
			Store: healthy.Redis, err: errors.New("forced post-commit cache failure"), failOnCall: 2,
		}
		service := oauth.Service{DB: db, Redis: cache}
		updated, err := service.UpdateClient(ctx, actor, clientID, oauth.ClientInput{
			Name: "After post commit", RedirectURI: "https://post-commit.example/changed",
			ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.self"},
		})

		if err != nil {
			t.Fatal(err)
		}
		if cache.calls != 2 || updated["name"] != "After post commit" ||
			updated["redirect_uri"] != "https://post-commit.example/changed" {
			t.Fatalf("post-commit update mismatch: calls=%d response=%#v", cache.calls, updated)
		}
		stored, err := db.OAuth.GetClient(ctx, clientID)
		if err != nil || stored == nil || stored.Name != "After post commit" ||
			stored.RedirectURI != "https://post-commit.example/changed" {
			t.Fatalf("post-commit update not persisted: client=%#v err=%v", stored, err)
		}
	})

	t.Run("credential failure rolls back client and persistent credentials", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-update-transaction@test.com", "Password123", "OAuthUpdateTransaction", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		service := newOAuthService(db)
		credential := issueDelegatedCredential(
			t, ctx, db, service, actor, "Transactional update", "https://transaction.example/callback", "account.read.self",
		)
		createPendingAuthorization(t, ctx, service, actor, credential.clientID, "https://transaction.example/callback", "account.read.self")
		if _, err := db.Pool.Exec(ctx, `
			CREATE FUNCTION fail_oauth_refresh_update() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'forced oauth refresh update failure';
			END;
			$$ LANGUAGE plpgsql;
		`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `
			CREATE TRIGGER fail_oauth_refresh_update
			BEFORE UPDATE ON oauth_refresh_tokens
			FOR EACH ROW EXECUTE FUNCTION fail_oauth_refresh_update();
		`); err != nil {
			t.Fatal(err)
		}
		_, err = service.UpdateClient(ctx, actor, credential.clientID, oauth.ClientInput{
			Name: "Changed transactional update", RedirectURI: "https://transaction.example/changed",
			ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.self"},
		})

		assertPgCode(t, err, "P0001")

		stored, err := db.OAuth.GetClient(ctx, credential.clientID)
		if err != nil || stored == nil || stored.Name != "Transactional update" ||
			stored.RedirectURI != "https://transaction.example/callback" || stored.Status != oauth.StatusActive {
			t.Fatalf("credential failure changed client: client=%#v err=%v", stored, err)
		}
		refresh, err := db.OAuth.GetRefreshToken(ctx, util.HashRefreshToken(credential.refreshToken))
		if err != nil || refresh == nil || refresh.RevokedAt != nil {
			t.Fatalf("credential failure changed refresh token: token=%#v err=%v", refresh, err)
		}
		var grantStatus string
		if err := db.Pool.QueryRow(ctx, `SELECT status FROM delegated_permission_grants WHERE id=$1`, credential.grantID).Scan(&grantStatus); err != nil || grantStatus != "active" {
			t.Fatalf("credential failure grant status=%q err=%v", grantStatus, err)
		}
		var codeCount int
		if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_authorization_codes WHERE client_id=$1`, credential.clientID).Scan(&codeCount); err != nil || codeCount != 2 {
			t.Fatalf("credential failure authorization code count=%d err=%v", codeCount, err)
		}
		if _, err := service.Redis.GetOAuthAccessToken(ctx, util.HashRefreshToken(credential.accessToken)); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("pre-transaction access invalidation=%v want cache miss", err)
		}
	})

	t.Run("review failure rolls back client permission grants and status", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		admin := testutil.CreateUser(t, db, "oauth-review-transaction@test.com", "Password123", "OAuthReviewTransaction", true, true)
		actor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		service := newOAuthService(db)
		created, err := service.CreateClient(ctx, actor, oauth.ClientInput{
			Name: "Transactional review", RedirectURI: "https://review-transaction.example/callback",
			ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.any"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		if _, err := db.Pool.Exec(ctx, `
			CREATE FUNCTION fail_oauth_client_review() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'forced oauth client review failure';
			END;
			$$ LANGUAGE plpgsql;
		`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `
			CREATE TRIGGER fail_oauth_client_review
			BEFORE UPDATE ON delegated_clients
			FOR EACH ROW EXECUTE FUNCTION fail_oauth_client_review();
		`); err != nil {
			t.Fatal(err)
		}

		_, err = service.ReviewClient(ctx, actor, clientID, oauth.StatusActive, "")
		assertPgCode(t, err, "P0001")
		stored, err := db.OAuth.GetClient(ctx, clientID)
		if err != nil || stored == nil || stored.Status != oauth.StatusPending {
			t.Fatalf("review failure changed client status: client=%#v err=%v", stored, err)
		}
		var overrideCount int
		if err := db.Pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM subject_permission_overrides
			WHERE subject_id=$1 AND permission_id=$2
		`, permissiondb.SubjectIDForClient(clientID), int64(permission.MustDefinitionByCode("account.read.any").ID)).Scan(&overrideCount); err != nil {
			t.Fatal(err)
		}
		if overrideCount != 0 {
			t.Fatalf("review failure left permission overrides: got=%d want=0", overrideCount)
		}
	})
}

func TestServiceClientCreationSucceedsWhenPostCommitNoticeFailsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-create-notice-fail@test.com", "Password123", "OAuthCreateNoticeFail", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE notices CASCADE`); err != nil {
		t.Fatal(err)
	}
	created, err := newOAuthService(db).CreateClient(ctx, actor, oauth.ClientInput{
		Name: "Notice failure app", RedirectURI: "https://notice-fail.example/callback",
		ClientType: oauth.ClientTypeConfidential, PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	stored, err := db.OAuth.GetClient(ctx, clientID)
	if err != nil || stored == nil || stored.Name != "Notice failure app" || stored.Status != oauth.StatusPending {
		t.Fatalf("notice failure client=%#v err=%v", stored, err)
	}
}
