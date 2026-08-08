package oauth_test

import (
	"context"
	"reflect"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestUpsertActiveGrantReusesLogicalGrantAndRollsBackAuthorizationCodeFailureExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-grant-upsert@test.com", "pw", "OAuthGrantUpsert", false)
	client := model.OAuthClient{
		ID:          "client-grant-upsert",
		OwnerUserID: user.ID,
		Name:        "Grant upsert client",
		RedirectURI: "https://grant-upsert.example/callback",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	firstPermissions := permissionIDs("account.read.self")
	secondPermissions := permissionIDs("account.read.self", "profile.read.owned")
	if err := db.OAuth.CreateClient(ctx, client, secondPermissions, nil); err != nil {
		t.Fatal(err)
	}
	first := model.OAuthGrant{
		ID:         "grant-upsert-first",
		UserID:     user.ID,
		SubjectID:  permissiondb.SubjectIDForUser(user.ID),
		ClientID:   client.ID,
		OIDCScopes: []string{"openid"},
		Status:     "active",
		CreatedAt:  1100,
	}
	firstCode := model.OAuthAuthorizationCode{
		CodeHash:            "grant-upsert-first-code",
		ClientID:            client.ID,
		UserID:              user.ID,
		RedirectURI:         client.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		OIDCScopes:          []string{"openid"},
		ExpiresAt:           5000,
		CreatedAt:           1100,
	}
	grantID, err := db.OAuth.UpsertActiveGrantAndCreateAuthorizationCode(ctx, first, firstPermissions, firstCode)
	if err != nil {
		t.Fatal(err)
	}
	if grantID != first.ID {
		t.Fatalf("initial upsert grant id=%q want %q", grantID, first.ID)
	}

	second := first
	second.ID = "grant-upsert-second"
	second.OIDCScopes = []string{"email", "openid"}
	second.CreatedAt = 1200
	secondCode := firstCode
	secondCode.CodeHash = "grant-upsert-second-code"
	secondCode.OIDCScopes = append([]string(nil), second.OIDCScopes...)
	secondCode.CreatedAt = 1200
	grantID, err = db.OAuth.UpsertActiveGrantAndCreateAuthorizationCode(ctx, second, secondPermissions, secondCode)
	if err != nil {
		t.Fatal(err)
	}
	if grantID != first.ID {
		t.Fatalf("repeated upsert grant id=%q want existing %q", grantID, first.ID)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	wantGrant := first
	wantGrant.OIDCScopes = []string{"email", "openid"}
	if !reflect.DeepEqual(grants, []model.OAuthGrant{wantGrant}) {
		t.Fatalf("upserted grants mismatch:\n got=%#v\nwant=%#v", grants, []model.OAuthGrant{wantGrant})
	}
	gotPermissions, err := db.OAuth.GrantPermissionIDs(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(gotPermissions, secondPermissions) {
		t.Fatalf("updated grant permissions=%v err=%v want=%v", gotPermissions, err, secondPermissions)
	}
	if err := db.OAuth.CreateGrant(ctx, second, firstPermissions); err == nil {
		t.Fatal("direct duplicate active grant should violate the unique constraint")
	} else {
		assertPgCode(t, err, "23505")
	}

	failing := second
	failing.ID = "grant-upsert-failing"
	failing.OIDCScopes = []string{"openid"}
	failing.CreatedAt = 1300
	duplicateCode := secondCode
	duplicateCode.CreatedAt = 1300
	if _, err := db.OAuth.UpsertActiveGrantAndCreateAuthorizationCode(ctx, failing, firstPermissions, duplicateCode); err == nil {
		t.Fatal("duplicate authorization code should fail the complete grant transaction")
	} else {
		assertPgCode(t, err, "23505")
	}
	gotPermissions, err = db.OAuth.GrantPermissionIDs(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(gotPermissions, secondPermissions) {
		t.Fatalf("failed code insert changed grant permissions=%v err=%v want=%v", gotPermissions, err, secondPermissions)
	}
	gotScopes, active, err := db.OAuth.ActiveGrantOIDCScopes(ctx, first.ID, user.ID, client.ID)
	if err != nil || !active || !reflect.DeepEqual(gotScopes, []string{"email", "openid"}) {
		t.Fatalf("failed code insert changed grant scopes=%v active=%v err=%v", gotScopes, active, err)
	}
}

func TestGrantAuthorizationCodeAndTokenLifecycle(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-grant@test.com", "pw", "OAuthGrant", false)
	clientPermissions := permissionIDs("profile.read.owned", "texture.read.owned", "notice.read.owned")
	grantPermissions := permissionIDs("profile.read.owned", "notice.read.owned")
	client := model.OAuthClient{
		ID:          "client-grant",
		OwnerUserID: user.ID,
		Name:        "Grant client",
		Description: "Grant test",
		RedirectURI: "https://app.example/callback",
		WebsiteURL:  "https://app.example",
		ClientType:  "confidential",
		SecretHash:  "secret-hash",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	if err := db.OAuth.CreateClient(ctx, client, clientPermissions, nil); err != nil {
		t.Fatal(err)
	}

	grant := model.OAuthGrant{
		ID:        "grant-1",
		UserID:    user.ID,
		SubjectID: permissiondb.SubjectIDForUser(user.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 1100,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, grantPermissions); err != nil {
		t.Fatal(err)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(grants, []model.OAuthGrant{grant}) {
		t.Fatalf("grants mismatch:\n got=%#v\nwant=%#v", grants, []model.OAuthGrant{grant})
	}
	gotGrantPermissions, err := db.OAuth.GrantPermissionIDs(ctx, grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotGrantPermissions, grantPermissions) {
		t.Fatalf("grant permissions=%v want=%v", gotGrantPermissions, grantPermissions)
	}
	missingGrantPermissions, err := db.OAuth.GrantPermissionIDs(ctx, "missing-grant")
	if err != nil {
		t.Fatal(err)
	}
	if len(missingGrantPermissions) != 0 {
		t.Fatalf("missing grant permissions should be empty: %v", missingGrantPermissions)
	}
	emptyGrantList, err := db.OAuth.ListGrantsByUser(ctx, "missing-user", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyGrantList) != 0 {
		t.Fatalf("missing user grant list should be empty: %#v", emptyGrantList)
	}

	code := model.OAuthAuthorizationCode{
		CodeHash:            "code-hash-1",
		ClientID:            client.ID,
		UserID:              user.ID,
		GrantID:             grant.ID,
		RedirectURI:         client.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           5000,
		CreatedAt:           1200,
	}
	if err := db.OAuth.CreateAuthorizationCode(ctx, code, grantPermissions); err != nil {
		t.Fatal(err)
	}
	consumedAt := int64(1300)
	wantCode := code
	wantCode.ConsumedAt = &consumedAt
	gotCode, gotCodePermissions, err := db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, code.ClientID, "https://wrong.example/callback", consumedAt)
	if err != nil || gotCode != nil || gotCodePermissions != nil {
		t.Fatalf("redirect mismatch must not consume code: code=%#v permissions=%v err=%v", gotCode, gotCodePermissions, err)
	}
	var storedConsumedAt *int64
	if err := db.Pool.QueryRow(ctx, `SELECT consumed_at FROM oauth_authorization_codes WHERE code_hash=$1`, code.CodeHash).Scan(&storedConsumedAt); err != nil {
		t.Fatal(err)
	}
	if storedConsumedAt != nil {
		t.Fatalf("redirect mismatch persisted consumed_at=%d", *storedConsumedAt)
	}
	gotCode, gotCodePermissions, err = db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, code.ClientID, code.RedirectURI, consumedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotCode, &wantCode) {
		t.Fatalf("consumed code mismatch:\n got=%#v\nwant=%#v", gotCode, &wantCode)
	}
	if !reflect.DeepEqual(gotCodePermissions, grantPermissions) {
		t.Fatalf("code permissions=%v want=%v", gotCodePermissions, grantPermissions)
	}
	gotCode, gotCodePermissions, err = db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, code.ClientID, code.RedirectURI, 1400)
	if err != nil {
		t.Fatal(err)
	}
	if gotCode != nil || gotCodePermissions != nil {
		t.Fatalf("authorization code replay should return nils: code=%#v permissions=%v", gotCode, gotCodePermissions)
	}
	expiredCode := code
	expiredCode.CodeHash = "expired-code"
	expiredCode.ExpiresAt = 1500
	if err := db.OAuth.CreateAuthorizationCode(ctx, expiredCode, grantPermissions); err != nil {
		t.Fatal(err)
	}
	gotCode, gotCodePermissions, err = db.OAuth.ConsumeAuthorizationCode(ctx, expiredCode.CodeHash, expiredCode.ClientID, expiredCode.RedirectURI, 1600)
	if err != nil {
		t.Fatal(err)
	}
	if gotCode != nil || gotCodePermissions != nil {
		t.Fatalf("expired authorization code should return nils: code=%#v permissions=%v", gotCode, gotCodePermissions)
	}

	refresh := model.OAuthToken{TokenHash: "refresh-1", ClientID: client.ID, UserID: user.ID, GrantID: grant.ID, ExpiresAt: 19000, CreatedAt: 2000}
	if err := db.OAuth.CreateRefreshToken(ctx, refresh); err != nil {
		t.Fatal(err)
	}
	gotRefresh, err := db.OAuth.GetRefreshToken(ctx, refresh.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotRefresh, &refresh) {
		t.Fatalf("refresh token mismatch:\n got=%#v\nwant=%#v", gotRefresh, &refresh)
	}

	newRefresh := model.OAuthToken{TokenHash: "refresh-2", ClientID: client.ID, UserID: user.ID, GrantID: grant.ID, ExpiresAt: 20000, CreatedAt: 3000}
	rotated, err := db.OAuth.RotateRefreshToken(ctx, refresh.TokenHash, newRefresh, 3100)
	if err != nil {
		t.Fatal(err)
	}
	if !rotated {
		t.Fatal("RotateRefreshToken should rotate active refresh token")
	}
	gotRefresh, err = db.OAuth.GetRefreshToken(ctx, refresh.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if gotRefresh.RevokedAt == nil || *gotRefresh.RevokedAt != 3100 {
		t.Fatalf("old refresh revoked_at mismatch: %#v", gotRefresh)
	}
	gotRefresh, err = db.OAuth.GetRefreshToken(ctx, newRefresh.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotRefresh, &newRefresh) {
		t.Fatalf("new refresh token mismatch:\n got=%#v\nwant=%#v", gotRefresh, &newRefresh)
	}
	rotated, err = db.OAuth.RotateRefreshToken(ctx, refresh.TokenHash, model.OAuthToken{TokenHash: "refresh-3"}, 3200)
	if err != nil {
		t.Fatal(err)
	}
	if rotated {
		t.Fatal("RotateRefreshToken should reject reused refresh token")
	}
	revoked, err := db.OAuth.RevokeRefreshToken(ctx, newRefresh.TokenHash, 3300)
	if err != nil || !revoked {
		t.Fatalf("RevokeRefreshToken should revoke active token: revoked=%v err=%v", revoked, err)
	}
	gotRefresh, err = db.OAuth.GetRefreshToken(ctx, newRefresh.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if gotRefresh.RevokedAt == nil || *gotRefresh.RevokedAt != 3300 {
		t.Fatalf("revoked refresh timestamp mismatch: %#v", gotRefresh)
	}
	revoked, err = db.OAuth.RevokeRefreshToken(ctx, newRefresh.TokenHash, 3400)
	if err != nil || revoked {
		t.Fatalf("RevokeRefreshToken should reject already revoked token: revoked=%v err=%v", revoked, err)
	}
	missingRefresh, err := db.OAuth.GetRefreshToken(ctx, "missing-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if missingRefresh != nil {
		t.Fatalf("missing refresh token should be nil: %#v", missingRefresh)
	}
	if revoked, err := db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 5000); err != nil || !revoked {
		t.Fatalf("RevokeGrant should revoke active grant: revoked=%v err=%v", revoked, err)
	} else if revoked, err = db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 5100); err != nil || revoked {
		t.Fatalf("RevokeGrant should reject already revoked grant: revoked=%v err=%v", revoked, err)
	}
	otherGrant := grant
	otherGrant.ID = "grant-owner-mismatch"
	if err := db.OAuth.CreateGrant(ctx, otherGrant, grantPermissions[:1]); err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.OAuth.RevokeGrant(ctx, otherGrant.ID, "other-user", 5200); err != nil || revoked {
		t.Fatalf("RevokeGrant should reject owner mismatch: revoked=%v err=%v", revoked, err)
	}
	storedGrantPermissions, err := db.OAuth.GrantPermissionIDs(ctx, otherGrant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(storedGrantPermissions, grantPermissions[:1]) {
		t.Fatalf("owner mismatch revoke should preserve grant permissions: %v", storedGrantPermissions)
	}
}

func TestActiveGrantPermissionIDsIntersectsActiveClientPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-active-grant@test.com", "pw", "OAuthActiveGrant", false)
	clientPermissions := permissionIDs("profile.read.owned", "texture.read.owned")
	grantPermissions := permissionIDs("profile.read.owned", "texture.read.owned", "notice.read.owned")
	client := model.OAuthClient{
		ID:          "client-active-grant",
		OwnerUserID: user.ID,
		Name:        "Active grant client",
		RedirectURI: "https://active.example/callback",
		WebsiteURL:  "https://active.example",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	if err := db.OAuth.CreateClient(ctx, client, clientPermissions, nil); err != nil {
		t.Fatal(err)
	}
	grant := model.OAuthGrant{
		ID:        "grant-active-permissions",
		UserID:    user.ID,
		SubjectID: permissiondb.SubjectIDForUser(user.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 1100,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, grantPermissions); err != nil {
		t.Fatal(err)
	}

	activePermissions, err := db.OAuth.ActiveGrantPermissionIDs(ctx, grant.ID, user.ID, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(activePermissions, clientPermissions) {
		t.Fatalf("active grant permissions=%v want client-approved intersection %v", activePermissions, clientPermissions)
	}
	state, err := db.OAuth.AuthorizationPermissionState(ctx, user.ID, client.ID, clientPermissions[0], clientPermissions[1])
	if err != nil || !state.OwnedGranted || !state.ApplicationRequested {
		t.Fatalf("active authorization permission state=%#v err=%v", state, err)
	}
	state, err = db.OAuth.AuthorizationPermissionState(ctx, user.ID, client.ID, grantPermissions[2], grantPermissions[2])
	if err != nil || state.OwnedGranted || state.ApplicationRequested {
		t.Fatalf("permissions outside client request state=%#v err=%v", state, err)
	}
	state, err = db.OAuth.AuthorizationPermissionState(ctx, "other-user", client.ID, clientPermissions[0], clientPermissions[1])
	if err != nil || state.OwnedGranted || !state.ApplicationRequested {
		t.Fatalf("wrong-user authorization permission state=%#v err=%v", state, err)
	}
	if activePermissions, err = db.OAuth.ActiveGrantPermissionIDs(ctx, grant.ID, "other-user", client.ID); err != nil || len(activePermissions) != 0 {
		t.Fatalf("active grant with wrong user should return empty: permissions=%v err=%v", activePermissions, err)
	}
	if activePermissions, err = db.OAuth.ActiveGrantPermissionIDs(ctx, grant.ID, user.ID, "other-client"); err != nil || len(activePermissions) != 0 {
		t.Fatalf("active grant with wrong client should return empty: permissions=%v err=%v", activePermissions, err)
	}
	if revoked, err := db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 1200); err != nil || !revoked {
		t.Fatalf("RevokeGrant before active permission check mismatch: revoked=%v err=%v", revoked, err)
	}
	if activePermissions, err = db.OAuth.ActiveGrantPermissionIDs(ctx, grant.ID, user.ID, client.ID); err != nil || len(activePermissions) != 0 {
		t.Fatalf("revoked grant should return empty active permissions: permissions=%v err=%v", activePermissions, err)
	}
	state, err = db.OAuth.AuthorizationPermissionState(ctx, user.ID, client.ID, clientPermissions[0], clientPermissions[1])
	if err != nil || state.OwnedGranted || !state.ApplicationRequested {
		t.Fatalf("revoked authorization permission state=%#v err=%v", state, err)
	}
}
