package oauth_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServiceCleanupGrantsRequiresSystemMaintenanceAndUsesLifecycleBoundariesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-grant-cleanup-service@test.com", "Password123", "OAuthGrantCleanupService", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	client, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Cleanup service app",
		RedirectURI:     "https://cleanup-service.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"profile.read.owned"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := client["client_id"].(string)
	createClientID := func(name string) string {
		t.Helper()
		created, createErr := svc.CreateClient(ctx, actor, oauth.ClientInput{
			Name:            name,
			RedirectURI:     "https://cleanup-service.example/callback",
			ClientType:      oauth.ClientTypePublic,
			PermissionCodes: []string{"profile.read.owned"},
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return created["client_id"].(string)
	}
	now := int64(10_000_000_000)
	expiredRevokedAt := now - int64(oauth.RevokedGrantRetention/time.Millisecond)
	recentRevokedAt := expiredRevokedAt + 1
	expired := model.OAuthGrant{ID: "service-expired-revoked-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: clientID, Status: "revoked", CreatedAt: 1000, RevokedAt: &expiredRevokedAt}
	recent := model.OAuthGrant{ID: "service-recent-revoked-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: clientID, Status: "revoked", CreatedAt: 2000, RevokedAt: &recentRevokedAt}
	stale := model.OAuthGrant{ID: "service-stale-active-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: createClientID("Cleanup stale app"), Status: oauth.StatusActive, CreatedAt: 3000}
	live := model.OAuthGrant{ID: "service-live-active-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: createClientID("Cleanup live app"), Status: oauth.StatusActive, CreatedAt: 4000}
	grace := model.OAuthGrant{ID: "service-issuance-grace-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: createClientID("Cleanup grace app"), Status: oauth.StatusActive, CreatedAt: now - int64(oauth.GrantIssuanceGrace/time.Millisecond) + 1}
	for _, grant := range []model.OAuthGrant{expired, recent, stale, live, grace} {
		if err := db.OAuth.CreateGrant(ctx, grant, permissionIDsFromCodesForTest([]string{"profile.read.owned"})); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
		TokenHash: "service-live-refresh-token",
		ClientID:  live.ClientID,
		UserID:    user.ID,
		GrantID:   live.ID,
		ExpiresAt: now + 1,
		CreatedAt: 5000,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.CleanupGrants(ctx, actor, now)
	assertHTTPError(t, err, 403, "permission.check.denied")
	if result != (oauth.GrantCleanupResult{}) {
		t.Fatalf("non-system cleanup result=%#v want zero result", result)
	}
	for _, onlyCode := range []string{"oauth_grant.revoke.system", "oauth_grant.delete.system"} {
		bits := permission.NewBitSet(len(permission.Definitions))
		bits.Set(permission.MustDefinitionByCode(onlyCode).BitIndex)
		partialActor := permission.Actor{SubjectID: "system:partial", Permissions: bits}
		result, err = svc.CleanupGrants(ctx, partialActor, now)
		assertHTTPError(t, err, 403, "permission.check.denied")
		if result != (oauth.GrantCleanupResult{}) {
			t.Fatalf("cleanup with only %s result=%#v want zero result", onlyCode, result)
		}
	}

	result, err = svc.CleanupGrants(ctx, permission.SystemMaintenanceActor(), now)
	if err != nil {
		t.Fatal(err)
	}
	if result != (oauth.GrantCleanupResult{Revoked: 1, Deleted: 1}) {
		t.Fatalf("system cleanup result=%#v want revoked=1 deleted=1", result)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 4 ||
		grants[0].ID != grace.ID || grants[0].Status != oauth.StatusActive || grants[0].RevokedAt != nil ||
		grants[1].ID != live.ID || grants[1].Status != oauth.StatusActive || grants[1].RevokedAt != nil ||
		grants[2].ID != stale.ID || grants[2].Status != "revoked" || grants[2].RevokedAt == nil || *grants[2].RevokedAt != now ||
		grants[3].ID != recent.ID || grants[3].Status != "revoked" || grants[3].RevokedAt == nil || *grants[3].RevokedAt != recentRevokedAt {
		t.Fatalf("remaining grants mismatch: %#v", grants)
	}
}
