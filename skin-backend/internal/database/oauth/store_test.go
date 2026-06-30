package oauth_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestClientLifecyclePreservesExactFieldsAndPermissions(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client@test.com", "pw", "OAuthClient", false)
	initialPermissions := permissionIDs("profile.read.owned", "texture.read.owned")
	updatedPermissions := permissionIDs("profile.read.owned", "notice.read.owned")

	client := model.OAuthClient{
		ID:          "client-1",
		OwnerUserID: user.ID,
		Name:        "First client",
		Description: "Initial description",
		RedirectURI: "https://app.example/callback",
		WebsiteURL:  "https://app.example",
		ClientType:  "confidential",
		SecretHash:  "secret-hash-1",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	if err := db.OAuth.CreateClient(ctx, client, initialPermissions); err != nil {
		t.Fatal(err)
	}
	var subjectKind, subjectStatus string
	if err := db.Pool.QueryRow(ctx, `
		SELECT kind, status
		FROM permission_subjects
		WHERE id=$1
	`, permissiondb.SubjectIDForClient(client.ID)).Scan(&subjectKind, &subjectStatus); err != nil {
		t.Fatal(err)
	}
	if subjectKind != "client" || subjectStatus != "active" {
		t.Fatalf("client subject mismatch: kind=%q status=%q", subjectKind, subjectStatus)
	}
	got, err := db.OAuth.GetClient(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &client) {
		t.Fatalf("client mismatch:\n got=%#v\nwant=%#v", got, &client)
	}
	gotPermissions, err := db.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotPermissions, initialPermissions) {
		t.Fatalf("initial permissions=%v want=%v", gotPermissions, initialPermissions)
	}
	list, err := db.OAuth.ListClientsByOwner(ctx, user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list, []model.OAuthClient{client}) {
		t.Fatalf("list clients mismatch:\n got=%#v\nwant=%#v", list, []model.OAuthClient{client})
	}

	client.Name = "Updated client"
	client.Description = "Updated description"
	client.RedirectURI = "https://app.example/oauth/callback"
	client.WebsiteURL = "https://docs.example"
	client.ClientType = "public"
	client.Status = "disabled"
	client.UpdatedAt = 2000
	updated, err := db.OAuth.UpdateClient(ctx, client, updatedPermissions)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("UpdateClient should update existing client")
	}
	got, err = db.OAuth.GetClient(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &client) {
		t.Fatalf("updated client mismatch:\n got=%#v\nwant=%#v", got, &client)
	}
	gotPermissions, err = db.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotPermissions, updatedPermissions) {
		t.Fatalf("updated permissions=%v want=%v", gotPermissions, updatedPermissions)
	}

	rotated, err := db.OAuth.RotateClientSecret(ctx, client.ID, "secret-hash-2", 3000)
	if err != nil {
		t.Fatal(err)
	}
	if !rotated {
		t.Fatal("RotateClientSecret should update existing client")
	}
	got, err = db.OAuth.GetClient(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecretHash != "secret-hash-2" || got.UpdatedAt != 3000 {
		t.Fatalf("rotated secret fields mismatch: secret=%q updated_at=%d", got.SecretHash, got.UpdatedAt)
	}
	if deleted, err := db.OAuth.DeleteClient(ctx, client.ID, "other-user"); err != nil || deleted {
		t.Fatalf("DeleteClient with owner mismatch should be false: deleted=%v err=%v", deleted, err)
	}
	if deleted, err := db.OAuth.DeleteClient(ctx, client.ID, user.ID); err != nil || !deleted {
		t.Fatalf("DeleteClient with owner should be true: deleted=%v err=%v", deleted, err)
	}
	if got, err = db.OAuth.GetClient(ctx, client.ID); err != nil || got != nil {
		t.Fatalf("deleted client should be nil: client=%#v err=%v", got, err)
	}
}

func TestClientAccessTokenLifecyclePreservesExactFieldsAndPermissions(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-token@test.com", "pw", "OAuthClientToken", false)
	clientPermissions := permissionIDs("minecraft_profile.read.public", "minecraft_session.hasjoined.server")
	client := model.OAuthClient{
		ID:          "app-token-client",
		OwnerUserID: user.ID,
		Name:        "App-only client",
		Description: "App-only token test",
		RedirectURI: "https://app.example/callback",
		WebsiteURL:  "https://app.example",
		ClientType:  "confidential",
		SecretHash:  "secret-hash",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	if err := db.OAuth.CreateClient(ctx, client, clientPermissions); err != nil {
		t.Fatal(err)
	}

	token := model.OAuthClientAccessToken{
		TokenHash: "client-access-1",
		ClientID:  client.ID,
		ExpiresAt: 5000,
		CreatedAt: 1100,
	}
	tokenPermissions := permissionIDs("minecraft_session.hasjoined.server")
	if err := db.OAuth.CreateClientAccessToken(ctx, token, tokenPermissions); err != nil {
		t.Fatal(err)
	}
	got, gotPermissions, err := db.OAuth.GetClientAccessToken(ctx, token.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &token) {
		t.Fatalf("client access token mismatch:\n got=%#v\nwant=%#v", got, &token)
	}
	if !reflect.DeepEqual(gotPermissions, tokenPermissions) {
		t.Fatalf("client token permissions=%v want=%v", gotPermissions, tokenPermissions)
	}

	revokedAt := int64(1200)
	revoked, err := db.OAuth.RevokeClientAccessToken(ctx, token.TokenHash, revokedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("RevokeClientAccessToken should revoke active client access token")
	}
	got, gotPermissions, err = db.OAuth.GetClientAccessToken(ctx, token.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	want := token
	want.RevokedAt = &revokedAt
	if !reflect.DeepEqual(got, &want) {
		t.Fatalf("revoked client access token mismatch:\n got=%#v\nwant=%#v", got, &want)
	}
	if !reflect.DeepEqual(gotPermissions, tokenPermissions) {
		t.Fatalf("revoked client token permissions=%v want=%v", gotPermissions, tokenPermissions)
	}
	revoked, err = db.OAuth.RevokeClientAccessToken(ctx, token.TokenHash, 1300)
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("RevokeClientAccessToken should reject already revoked client access token")
	}
	missing, missingPermissions, err := db.OAuth.GetClientAccessToken(ctx, "missing-token")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil || missingPermissions != nil {
		t.Fatalf("missing client token should return nils: token=%#v permissions=%v", missing, missingPermissions)
	}
}

func TestDeviceCodeLifecyclePreservesExactFieldsPermissionsAndStates(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device@test.com", "pw", "OAuthDevice", false)
	client := model.OAuthClient{
		ID:          "device-client",
		OwnerUserID: user.ID,
		Name:        "Device client",
		RedirectURI: "https://device.example/callback",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	permissions := permissionIDs("account.read.self", "minecraft_profile.read.public")
	if err := db.OAuth.CreateClient(ctx, client, permissions); err != nil {
		t.Fatal(err)
	}
	subjectID := permissiondb.SubjectIDForUser(user.ID)
	code := model.OAuthDeviceCode{
		DeviceCodeHash: "device-hash-1",
		UserCodeHash:   "user-hash-1",
		ClientID:       client.ID,
		Status:         "pending",
		ExpiresAt:      5000,
		CreatedAt:      1100,
	}
	if err := db.OAuth.CreateDeviceCode(ctx, code, permissions); err != nil {
		t.Fatal(err)
	}
	got, gotPermissions, err := db.OAuth.GetDeviceCodeByUserCodeHash(ctx, code.UserCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &code) {
		t.Fatalf("device code by user code mismatch:\n got=%#v\nwant=%#v", got, &code)
	}
	if !reflect.DeepEqual(gotPermissions, permissions) {
		t.Fatalf("device code permissions=%v want=%v", gotPermissions, permissions)
	}
	if ok, err := db.OAuth.ApproveDeviceCode(ctx, code.UserCodeHash, user.ID, subjectID, 1200); err != nil || !ok {
		t.Fatalf("ApproveDeviceCode should approve pending code: ok=%v err=%v", ok, err)
	}
	got, _, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, code.DeviceCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	approvedAt := int64(1200)
	wantApproved := code
	wantApproved.UserID = &user.ID
	wantApproved.SubjectID = &subjectID
	wantApproved.Status = "approved"
	wantApproved.ApprovedAt = &approvedAt
	if !reflect.DeepEqual(got, &wantApproved) {
		t.Fatalf("approved device code mismatch:\n got=%#v\nwant=%#v", got, &wantApproved)
	}
	consumed, consumedPermissions, err := db.OAuth.ConsumeApprovedDeviceCode(ctx, code.DeviceCodeHash, 1300)
	if err != nil {
		t.Fatal(err)
	}
	consumedAt := int64(1300)
	wantConsumed := wantApproved
	wantConsumed.Status = "consumed"
	wantConsumed.ConsumedAt = &consumedAt
	if !reflect.DeepEqual(consumed, &wantConsumed) {
		t.Fatalf("consumed device code mismatch:\n got=%#v\nwant=%#v", consumed, &wantConsumed)
	}
	if !reflect.DeepEqual(consumedPermissions, permissions) {
		t.Fatalf("consumed permissions=%v want=%v", consumedPermissions, permissions)
	}
	replay, replayPermissions, err := db.OAuth.ConsumeApprovedDeviceCode(ctx, code.DeviceCodeHash, 1400)
	if err != nil {
		t.Fatal(err)
	}
	if replay != nil || replayPermissions != nil {
		t.Fatalf("device code replay should return nils: code=%#v permissions=%v", replay, replayPermissions)
	}

	denied := code
	denied.DeviceCodeHash = "device-hash-denied"
	denied.UserCodeHash = "user-hash-denied"
	if err := db.OAuth.CreateDeviceCode(ctx, denied, permissions[:1]); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.OAuth.DenyDeviceCode(ctx, denied.UserCodeHash, 1500); err != nil || !ok {
		t.Fatalf("DenyDeviceCode should deny pending code: ok=%v err=%v", ok, err)
	}
	got, gotPermissions, err = db.OAuth.GetDeviceCodeByUserCodeHash(ctx, denied.UserCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	deniedAt := int64(1500)
	wantDenied := denied
	wantDenied.Status = "denied"
	wantDenied.DeniedAt = &deniedAt
	if !reflect.DeepEqual(got, &wantDenied) {
		t.Fatalf("denied device code mismatch:\n got=%#v\nwant=%#v", got, &wantDenied)
	}
	if !reflect.DeepEqual(gotPermissions, permissions[:1]) {
		t.Fatalf("denied permissions=%v want=%v", gotPermissions, permissions[:1])
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
	if err := db.OAuth.CreateClient(ctx, client, clientPermissions); err != nil {
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
	gotCode, gotCodePermissions, err := db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, consumedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotCode, &wantCode) {
		t.Fatalf("consumed code mismatch:\n got=%#v\nwant=%#v", gotCode, &wantCode)
	}
	if !reflect.DeepEqual(gotCodePermissions, grantPermissions) {
		t.Fatalf("code permissions=%v want=%v", gotCodePermissions, grantPermissions)
	}
	gotCode, gotCodePermissions, err = db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, 1400)
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
	gotCode, gotCodePermissions, err = db.OAuth.ConsumeAuthorizationCode(ctx, expiredCode.CodeHash, 1600)
	if err != nil {
		t.Fatal(err)
	}
	if gotCode != nil || gotCodePermissions != nil {
		t.Fatalf("expired authorization code should return nils: code=%#v permissions=%v", gotCode, gotCodePermissions)
	}

	access := model.OAuthToken{TokenHash: "access-1", ClientID: client.ID, UserID: user.ID, GrantID: grant.ID, ExpiresAt: 9000, CreatedAt: 2000}
	refresh := model.OAuthToken{TokenHash: "refresh-1", ClientID: client.ID, UserID: user.ID, GrantID: grant.ID, ExpiresAt: 19000, CreatedAt: 2000}
	if err := db.OAuth.CreateTokens(ctx, access, refresh); err != nil {
		t.Fatal(err)
	}
	gotAccess, err := db.OAuth.GetAccessToken(ctx, access.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotAccess, &access) {
		t.Fatalf("access token mismatch:\n got=%#v\nwant=%#v", gotAccess, &access)
	}
	gotRefresh, err := db.OAuth.GetRefreshToken(ctx, refresh.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotRefresh, &refresh) {
		t.Fatalf("refresh token mismatch:\n got=%#v\nwant=%#v", gotRefresh, &refresh)
	}

	newAccess := model.OAuthToken{TokenHash: "access-2", ClientID: client.ID, UserID: user.ID, GrantID: grant.ID, ExpiresAt: 10000, CreatedAt: 3000}
	newRefresh := model.OAuthToken{TokenHash: "refresh-2", ClientID: client.ID, UserID: user.ID, GrantID: grant.ID, ExpiresAt: 20000, CreatedAt: 3000}
	rotated, err := db.OAuth.RotateRefreshToken(ctx, refresh.TokenHash, newAccess, newRefresh, 3100)
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
	rotated, err = db.OAuth.RotateRefreshToken(ctx, refresh.TokenHash, model.OAuthToken{TokenHash: "access-3"}, model.OAuthToken{TokenHash: "refresh-3"}, 3200)
	if err != nil {
		t.Fatal(err)
	}
	if rotated {
		t.Fatal("RotateRefreshToken should reject reused refresh token")
	}
	revoked, err := db.OAuth.RevokeAccessToken(ctx, access.TokenHash, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("RevokeAccessToken should revoke active access token")
	}
	revoked, err = db.OAuth.RevokeAccessToken(ctx, access.TokenHash, 4100)
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("RevokeAccessToken should reject already revoked access token")
	}
	if revoked, err = db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 5000); err != nil || !revoked {
		t.Fatalf("RevokeGrant should revoke active grant: revoked=%v err=%v", revoked, err)
	}
	if revoked, err = db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 5100); err != nil || revoked {
		t.Fatalf("RevokeGrant should reject already revoked grant: revoked=%v err=%v", revoked, err)
	}
}

func permissionIDs(codes ...string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(core.MustDefinitionByCode(code).ID))
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}
