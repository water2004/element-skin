package oauth_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestDeleteRevokedGrantsDeletesOnlyExpiredRevokedRowsAndDependenciesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-grant-cleanup@test.com", "pw", "OAuthGrantCleanup", false)
	client := model.OAuthClient{
		ID:          "client-grant-cleanup",
		OwnerUserID: user.ID,
		Name:        "Grant cleanup client",
		RedirectURI: "https://cleanup.example/callback",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	permissions := permissionIDs("profile.read.owned", "notice.read.owned")
	if err := db.OAuth.CreateClient(ctx, client, permissions); err != nil {
		t.Fatal(err)
	}
	oldRevokedAt := int64(1000)
	recentRevokedAt := int64(3000)
	grants := []model.OAuthGrant{
		{ID: "grant-cleanup-old", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "revoked", CreatedAt: 900, RevokedAt: &oldRevokedAt},
		{ID: "grant-cleanup-recent", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "revoked", CreatedAt: 2900, RevokedAt: &recentRevokedAt},
		{ID: "grant-cleanup-active", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 3100},
	}
	for _, grant := range grants {
		if err := db.OAuth.CreateGrant(ctx, grant, permissions); err != nil {
			t.Fatal(err)
		}
		if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
			TokenHash: "refresh-" + grant.ID,
			ClientID:  client.ID,
			UserID:    user.ID,
			GrantID:   grant.ID,
			ExpiresAt: 10000,
			CreatedAt: grant.CreatedAt,
		}); err != nil {
			t.Fatal(err)
		}
		if err := db.OAuth.CreateAuthorizationCode(ctx, model.OAuthAuthorizationCode{
			CodeHash:            "code-" + grant.ID,
			ClientID:            client.ID,
			UserID:              user.ID,
			GrantID:             grant.ID,
			RedirectURI:         client.RedirectURI,
			CodeChallenge:       "challenge",
			CodeChallengeMethod: "S256",
			ExpiresAt:           10000,
			CreatedAt:           grant.CreatedAt,
		}, permissions); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := db.OAuth.DeleteRevokedGrants(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteRevokedGrants deleted=%d want 1", deleted)
	}
	assertOAuthGrantExists(t, db, "grant-cleanup-old", false)
	assertOAuthGrantExists(t, db, "grant-cleanup-recent", true)
	assertOAuthGrantExists(t, db, "grant-cleanup-active", true)
	assertOAuthDependencyCount(t, db, "grant-cleanup-old", 0)
	assertOAuthDependencyCount(t, db, "grant-cleanup-recent", 6)
	assertOAuthDependencyCount(t, db, "grant-cleanup-active", 6)

	deleted, err = db.OAuth.DeleteRevokedGrants(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("second DeleteRevokedGrants deleted=%d want 0", deleted)
	}
}

func TestRevokeInactiveGrantsUsesRefreshCodeAndIssuanceBoundariesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-inactive-grant-cleanup@test.com", "pw", "OAuthInactiveGrantCleanup", false)
	permissions := permissionIDs("profile.read.owned")

	const (
		now           = int64(10_000)
		createdBefore = int64(9_000)
	)
	alreadyRevokedAt := int64(500)
	grants := []model.OAuthGrant{
		{ID: "grant-expired-refresh", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: 1000},
		{ID: "grant-revoked-refresh", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: 1100},
		{ID: "grant-without-credentials", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: 1200},
		{ID: "grant-expired-code", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: 1300},
		{ID: "grant-active-refresh", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: 1400},
		{ID: "grant-live-consumed-code", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: 1500},
		{ID: "grant-issuance-grace", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "active", CreatedAt: createdBefore + 1},
		{ID: "grant-already-revoked", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), Status: "revoked", CreatedAt: 1600, RevokedAt: &alreadyRevokedAt},
	}
	clientByGrant := make(map[string]model.OAuthClient, len(grants))
	for i := range grants {
		grant := &grants[i]
		client := model.OAuthClient{
			ID:          "client-" + grant.ID,
			OwnerUserID: user.ID,
			Name:        "Inactive grant cleanup client " + grant.ID,
			RedirectURI: "https://inactive-cleanup.example/callback/" + grant.ID,
			ClientType:  "public",
			Status:      "active",
			CreatedAt:   100,
			UpdatedAt:   100,
		}
		if err := db.OAuth.CreateClient(ctx, client, permissions); err != nil {
			t.Fatal(err)
		}
		grant.ClientID = client.ID
		clientByGrant[grant.ID] = client
	}
	for _, grant := range grants {
		if err := db.OAuth.CreateGrant(ctx, grant, permissions); err != nil {
			t.Fatal(err)
		}
	}

	revokedRefreshAt := int64(9000)
	refreshTokens := []model.OAuthToken{
		{TokenHash: "refresh-expired", ClientID: clientByGrant["grant-expired-refresh"].ID, UserID: user.ID, GrantID: "grant-expired-refresh", ExpiresAt: now, CreatedAt: 2000},
		{TokenHash: "refresh-revoked", ClientID: clientByGrant["grant-revoked-refresh"].ID, UserID: user.ID, GrantID: "grant-revoked-refresh", ExpiresAt: now + 1, CreatedAt: 2100, RevokedAt: &revokedRefreshAt},
		{TokenHash: "refresh-active", ClientID: clientByGrant["grant-active-refresh"].ID, UserID: user.ID, GrantID: "grant-active-refresh", ExpiresAt: now + 1, CreatedAt: 2200},
	}
	for _, refresh := range refreshTokens {
		if err := db.OAuth.CreateRefreshToken(ctx, refresh); err != nil {
			t.Fatal(err)
		}
	}

	consumedAt := int64(9500)
	codes := []model.OAuthAuthorizationCode{
		{CodeHash: "code-expired", ClientID: clientByGrant["grant-expired-code"].ID, UserID: user.ID, GrantID: "grant-expired-code", RedirectURI: clientByGrant["grant-expired-code"].RedirectURI, CodeChallenge: "challenge", CodeChallengeMethod: "S256", ExpiresAt: now, CreatedAt: 3000},
		{CodeHash: "code-live-consumed", ClientID: clientByGrant["grant-live-consumed-code"].ID, UserID: user.ID, GrantID: "grant-live-consumed-code", RedirectURI: clientByGrant["grant-live-consumed-code"].RedirectURI, CodeChallenge: "challenge", CodeChallengeMethod: "S256", ExpiresAt: now + 1, CreatedAt: 3100, ConsumedAt: &consumedAt},
	}
	for _, code := range codes {
		if err := db.OAuth.CreateAuthorizationCode(ctx, code, permissions); err != nil {
			t.Fatal(err)
		}
	}

	revoked, err := db.OAuth.RevokeInactiveGrants(ctx, now, createdBefore)
	if err != nil {
		t.Fatal(err)
	}
	if revoked != 4 {
		t.Fatalf("RevokeInactiveGrants revoked=%d want 4", revoked)
	}

	want := map[string]struct {
		status    string
		revokedAt *int64
	}{
		"grant-expired-refresh":     {status: "revoked", revokedAt: int64Pointer(now)},
		"grant-revoked-refresh":     {status: "revoked", revokedAt: int64Pointer(now)},
		"grant-without-credentials": {status: "revoked", revokedAt: int64Pointer(now)},
		"grant-expired-code":        {status: "revoked", revokedAt: int64Pointer(now)},
		"grant-active-refresh":      {status: "active"},
		"grant-live-consumed-code":  {status: "active"},
		"grant-issuance-grace":      {status: "active"},
		"grant-already-revoked":     {status: "revoked", revokedAt: &alreadyRevokedAt},
	}
	for grantID, expected := range want {
		var status string
		var revokedAt *int64
		if err := db.Pool.QueryRow(ctx, `
			SELECT status, revoked_at FROM delegated_permission_grants WHERE id=$1
		`, grantID).Scan(&status, &revokedAt); err != nil {
			t.Fatal(err)
		}
		if status != expected.status || !equalInt64Pointers(revokedAt, expected.revokedAt) {
			t.Fatalf("grant %s state=(%s,%v) want=(%s,%v)", grantID, status, revokedAt, expected.status, expected.revokedAt)
		}
	}

	revoked, err = db.OAuth.RevokeInactiveGrants(ctx, now, createdBefore)
	if err != nil {
		t.Fatal(err)
	}
	if revoked != 0 {
		t.Fatalf("second RevokeInactiveGrants revoked=%d want 0", revoked)
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

func equalInt64Pointers(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
