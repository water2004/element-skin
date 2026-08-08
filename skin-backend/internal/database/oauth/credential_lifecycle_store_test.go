package oauth_test

import (
	"context"
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestOAuthCredentialLifecycleStorePrimitivesTargetExactClientAndGrant(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-lifecycle-store@test.com", "Password123", "OAuthLifecycleStore", false)
	permissionSet := permissionIDs("account.read.self")
	clients := []model.OAuthClient{
		{ID: "lifecycle-client-a", OwnerUserID: owner.ID, Name: "Lifecycle A", RedirectURI: "https://a.example/callback", ClientType: "confidential", SecretHash: "secret-a", Status: "active", CreatedAt: 1000, UpdatedAt: 1000},
		{ID: "lifecycle-client-b", OwnerUserID: owner.ID, Name: "Lifecycle B", RedirectURI: "https://b.example/callback", ClientType: "confidential", SecretHash: "secret-b", Status: "active", CreatedAt: 1100, UpdatedAt: 1100},
	}
	for _, client := range clients {
		if err := db.OAuth.CreateClient(ctx, client, permissionSet, nil); err != nil {
			t.Fatal(err)
		}
	}
	clientIDs, err := db.OAuth.ClientIDsByOwner(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(clientIDs, []string{"lifecycle-client-a", "lifecycle-client-b"}) {
		t.Fatalf("client ids by owner mismatch: %v", clientIDs)
	}

	grants := []model.OAuthGrant{
		{ID: "lifecycle-grant-a", UserID: owner.ID, SubjectID: permissiondb.SubjectIDForUser(owner.ID), ClientID: clients[0].ID, Status: "active", CreatedAt: 1200},
		{ID: "lifecycle-grant-b", UserID: owner.ID, SubjectID: permissiondb.SubjectIDForUser(owner.ID), ClientID: clients[1].ID, Status: "active", CreatedAt: 1300},
	}
	for index, grant := range grants {
		if err := db.OAuth.CreateGrant(ctx, grant, permissionSet); err != nil {
			t.Fatal(err)
		}
		if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
			TokenHash: "lifecycle-refresh-" + string(rune('a'+index)),
			ClientID:  grant.ClientID,
			UserID:    owner.ID,
			GrantID:   grant.ID,
			ExpiresAt: 9000,
			CreatedAt: 1400 + int64(index),
		}); err != nil {
			t.Fatal(err)
		}
		if err := db.OAuth.CreateAuthorizationCode(ctx, model.OAuthAuthorizationCode{
			CodeHash:            "lifecycle-code-" + string(rune('a'+index)),
			ClientID:            grant.ClientID,
			UserID:              owner.ID,
			GrantID:             grant.ID,
			RedirectURI:         clients[index].RedirectURI,
			CodeChallenge:       "challenge",
			CodeChallengeMethod: "S256",
			ExpiresAt:           9000,
			CreatedAt:           1500 + int64(index),
		}, permissionSet); err != nil {
			t.Fatal(err)
		}
	}
	revokedGrantIDs, err := db.OAuth.RevokeGrantsByClient(ctx, clients[0].ID, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(revokedGrantIDs, []string{"lifecycle-grant-a"}) {
		t.Fatalf("revoked grants by client mismatch: %v", revokedGrantIDs)
	}
	if count, err := db.OAuth.RevokeRefreshTokensByGrant(ctx, grants[0].ID, 2100); err != nil || count != 1 {
		t.Fatalf("revoke refresh by grant mismatch: count=%d err=%v", count, err)
	}
	if count, err := db.OAuth.RevokeRefreshTokensByGrant(ctx, grants[0].ID, 2200); err != nil || count != 0 {
		t.Fatalf("second revoke refresh by grant mismatch: count=%d err=%v", count, err)
	}
	if count, err := db.OAuth.DeleteAuthorizationCodesByGrant(ctx, grants[0].ID); err != nil || count != 1 {
		t.Fatalf("delete authorization code by grant mismatch: count=%d err=%v", count, err)
	}
	if count, err := db.OAuth.DeleteAuthorizationCodesByGrant(ctx, grants[0].ID); err != nil || count != 0 {
		t.Fatalf("second delete authorization code by grant mismatch: count=%d err=%v", count, err)
	}

	if count, err := db.OAuth.RevokeRefreshTokensByClient(ctx, clients[1].ID, 2300); err != nil || count != 1 {
		t.Fatalf("revoke refresh by client mismatch: count=%d err=%v", count, err)
	}
	if count, err := db.OAuth.DeleteAuthorizationCodesByClient(ctx, clients[1].ID); err != nil || count != 1 {
		t.Fatalf("delete authorization code by client mismatch: count=%d err=%v", count, err)
	}
	assertOAuthGrantStatus(t, db, grants[0].ID, "revoked", 2000)
	assertOAuthGrantStatus(t, db, grants[1].ID, "active", 0)
}

func assertOAuthGrantStatus(t *testing.T, db *database.DB, grantID, wantStatus string, wantRevokedAt int64) {
	t.Helper()
	var status string
	var revokedAt *int64
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT status, revoked_at FROM delegated_permission_grants WHERE id=$1
	`, grantID).Scan(&status, &revokedAt); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("grant %q status=%q want %q", grantID, status, wantStatus)
	}
	if wantRevokedAt == 0 {
		if revokedAt != nil {
			t.Fatalf("grant %q revoked_at=%v want nil", grantID, revokedAt)
		}
		return
	}
	if revokedAt == nil || *revokedAt != wantRevokedAt {
		t.Fatalf("grant %q revoked_at=%v want %d", grantID, revokedAt, wantRevokedAt)
	}
}
