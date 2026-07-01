package oauth_test

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClientLifecyclePreservesExactFieldsAndPermissions(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client@test.com", "pw", "OAuthClient", false)
	other := testutil.CreateUser(t, db, "oauth-client-other@test.com", "pw", "OAuthClientOther", false)
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
	otherClient := model.OAuthClient{
		ID:          "client-2",
		OwnerUserID: other.ID,
		Name:        "Second client",
		RedirectURI: "https://second.example/callback",
		ClientType:  "public",
		Status:      "pending",
		CreatedAt:   1500,
		UpdatedAt:   1500,
	}
	if err := db.OAuth.CreateClient(ctx, otherClient, []int64{}); err != nil {
		t.Fatal(err)
	}
	allClients, err := db.OAuth.ListClients(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(allClients, []model.OAuthClient{otherClient, client}) {
		t.Fatalf("all client order mismatch:\n got=%#v\nwant=%#v", allClients, []model.OAuthClient{otherClient, client})
	}
	limitedClients, err := db.OAuth.ListClients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(limitedClients, []model.OAuthClient{otherClient}) {
		t.Fatalf("limited client order mismatch:\n got=%#v\nwant=%#v", limitedClients, []model.OAuthClient{otherClient})
	}
	pendingClients, err := db.OAuth.ListClientsByStatus(ctx, "pending", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pendingClients, []model.OAuthClient{otherClient}) {
		t.Fatalf("pending client list mismatch:\n got=%#v\nwant=%#v", pendingClients, []model.OAuthClient{otherClient})
	}
	allByStatus, err := db.OAuth.ListClientsByStatus(ctx, "all", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(allByStatus, allClients) {
		t.Fatalf("status=all client list mismatch:\n got=%#v\nwant=%#v", allByStatus, allClients)
	}
	if updated, err := db.OAuth.UpdateClientStatus(ctx, otherClient.ID, "disabled", 1600); err != nil || !updated {
		t.Fatalf("UpdateClientStatus should update pending client: updated=%v err=%v", updated, err)
	}
	otherClient.Status = "disabled"
	otherClient.UpdatedAt = 1600
	gotOther, err := db.OAuth.GetClient(ctx, otherClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotOther, &otherClient) {
		t.Fatalf("updated status client mismatch:\n got=%#v\nwant=%#v", gotOther, &otherClient)
	}
	if updated, err := db.OAuth.UpdateClientStatus(ctx, "missing-client", "active", 1700); err != nil || updated {
		t.Fatalf("UpdateClientStatus should miss unknown client: updated=%v err=%v", updated, err)
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
	missingPermissions, err := db.OAuth.ClientPermissionIDs(ctx, "missing-client")
	if err != nil {
		t.Fatal(err)
	}
	if len(missingPermissions) != 0 {
		t.Fatalf("missing client permissions should be empty: %v", missingPermissions)
	}
	emptyOwnerList, err := db.OAuth.ListClientsByOwner(ctx, "missing-owner", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyOwnerList) != 0 {
		t.Fatalf("missing owner client list should be empty: %#v", emptyOwnerList)
	}
	emptyStatusList, err := db.OAuth.ListClientsByStatus(ctx, "active", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyStatusList) != 0 {
		t.Fatalf("zero-limit active client list should be empty: %#v", emptyStatusList)
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
	if rotated, err := db.OAuth.RotateClientSecret(ctx, "missing-client", "secret-hash-3", 3100); err != nil || rotated {
		t.Fatalf("RotateClientSecret should miss unknown client: rotated=%v err=%v", rotated, err)
	}
	missingUpdate := client
	missingUpdate.ID = "missing-client"
	if updated, err := db.OAuth.UpdateClient(ctx, missingUpdate, updatedPermissions); err != nil || updated {
		t.Fatalf("UpdateClient should miss unknown client: updated=%v err=%v", updated, err)
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
	if deleted, err := db.OAuth.DeleteClient(ctx, otherClient.ID, ""); err != nil || !deleted {
		t.Fatalf("admin DeleteClient should delete by empty owner: deleted=%v err=%v", deleted, err)
	}
}

func TestStoreClosedPoolReturnsExactDependencyErrorsForEveryOAuthTable(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	client := model.OAuthClient{
		ID:          "closed-client",
		OwnerUserID: "closed-owner",
		Name:        "Closed client",
		RedirectURI: "https://closed.example/callback",
		ClientType:  "confidential",
		SecretHash:  "secret",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	refresh := model.OAuthToken{
		TokenHash: "closed-refresh-new",
		ClientID:  client.ID,
		UserID:    "closed-user",
		GrantID:   "closed-grant",
		ExpiresAt: 2000,
		CreatedAt: 1000,
	}
	device := model.OAuthDeviceCode{
		DeviceCodeHash: "closed-device",
		UserCodeHash:   "closed-user-code",
		ClientID:       client.ID,
		Status:         "pending",
		ExpiresAt:      2000,
		CreatedAt:      1000,
	}
	code := model.OAuthAuthorizationCode{
		CodeHash:            "closed-code",
		ClientID:            client.ID,
		UserID:              "closed-user",
		GrantID:             "closed-grant",
		RedirectURI:         client.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           2000,
		CreatedAt:           1000,
	}
	grant := model.OAuthGrant{
		ID:        "closed-grant",
		UserID:    "closed-user",
		SubjectID: "user:closed-user",
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 1000,
	}
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "create client", call: func() error { return db.OAuth.CreateClient(ctx, client, permissionIDs("account.read.self")) }},
		{name: "update client", call: func() error {
			_, err := db.OAuth.UpdateClient(ctx, client, permissionIDs("account.read.self"))
			return err
		}},
		{name: "rotate client secret", call: func() error {
			_, err := db.OAuth.RotateClientSecret(ctx, client.ID, "new-secret", 2000)
			return err
		}},
		{name: "delete client", call: func() error {
			_, err := db.OAuth.DeleteClient(ctx, client.ID, "")
			return err
		}},
		{name: "get client", call: func() error {
			_, err := db.OAuth.GetClient(ctx, client.ID)
			return err
		}},
		{name: "list owner clients", call: func() error {
			_, err := db.OAuth.ListClientsByOwner(ctx, client.OwnerUserID, 10)
			return err
		}},
		{name: "list clients", call: func() error {
			_, err := db.OAuth.ListClients(ctx, 10)
			return err
		}},
		{name: "list clients by status", call: func() error {
			_, err := db.OAuth.ListClientsByStatus(ctx, "active", 10)
			return err
		}},
		{name: "update client status", call: func() error {
			_, err := db.OAuth.UpdateClientStatus(ctx, client.ID, "disabled", 2000)
			return err
		}},
		{name: "client permission ids", call: func() error {
			_, err := db.OAuth.ClientPermissionIDs(ctx, client.ID)
			return err
		}},
		{name: "create grant", call: func() error { return db.OAuth.CreateGrant(ctx, grant, permissionIDs("account.read.self")) }},
		{name: "revoke grant", call: func() error {
			_, err := db.OAuth.RevokeGrant(ctx, grant.ID, grant.UserID, 2000)
			return err
		}},
		{name: "list grants", call: func() error {
			_, err := db.OAuth.ListGrantsByUser(ctx, grant.UserID, 10)
			return err
		}},
		{name: "grant permission ids", call: func() error {
			_, err := db.OAuth.GrantPermissionIDs(ctx, grant.ID)
			return err
		}},
		{name: "create authorization code", call: func() error {
			return db.OAuth.CreateAuthorizationCode(ctx, code, permissionIDs("account.read.self"))
		}},
		{name: "consume authorization code", call: func() error {
			_, _, err := db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, 1500)
			return err
		}},
		{name: "create refresh token", call: func() error { return db.OAuth.CreateRefreshToken(ctx, refresh) }},
		{name: "get refresh token", call: func() error {
			_, err := db.OAuth.GetRefreshToken(ctx, refresh.TokenHash)
			return err
		}},
		{name: "revoke refresh token", call: func() error {
			_, err := db.OAuth.RevokeRefreshToken(ctx, refresh.TokenHash, 1500)
			return err
		}},
		{name: "rotate refresh token", call: func() error {
			_, err := db.OAuth.RotateRefreshToken(ctx, refresh.TokenHash, refresh, 1500)
			return err
		}},
		{name: "create device code", call: func() error {
			return db.OAuth.CreateDeviceCode(ctx, device, permissionIDs("account.read.self"))
		}},
		{name: "get device by user code", call: func() error {
			_, _, err := db.OAuth.GetDeviceCodeByUserCodeHash(ctx, device.UserCodeHash)
			return err
		}},
		{name: "get device by device code", call: func() error {
			_, _, err := db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, device.DeviceCodeHash)
			return err
		}},
		{name: "approve device code", call: func() error {
			_, err := db.OAuth.ApproveDeviceCode(ctx, device.UserCodeHash, "closed-user", "user:closed-user", 1500)
			return err
		}},
		{name: "deny device code", call: func() error {
			_, err := db.OAuth.DenyDeviceCode(ctx, device.UserCodeHash, 1500)
			return err
		}},
		{name: "mark device polled", call: func() error {
			return db.OAuth.MarkDeviceCodePolled(ctx, device.DeviceCodeHash, 1500)
		}},
		{name: "consume approved device code", call: func() error {
			_, _, err := db.OAuth.ConsumeApprovedDeviceCode(ctx, device.DeviceCodeHash, 1500)
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil || !strings.Contains(err.Error(), "closed pool") {
				t.Fatalf("%s error mismatch: %v", tc.name, err)
			}
		})
	}
}

func TestStoreRollsBackOAuthWritesOnExactForeignKeyFailures(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-rollback@test.com", "pw", "OAuthRollback", false)
	validClient := model.OAuthClient{
		ID:          "rollback-client",
		OwnerUserID: user.ID,
		Name:        "Rollback client",
		RedirectURI: "https://rollback.example/callback",
		ClientType:  "confidential",
		SecretHash:  "secret",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	if err := db.OAuth.CreateClient(ctx, validClient, []int64{}); err != nil {
		t.Fatal(err)
	}

	duplicate := validClient
	duplicate.Name = "Duplicate should not win"
	err := db.OAuth.CreateClient(ctx, duplicate, []int64{})
	assertPgCode(t, err, "23505")
	stored, err := db.OAuth.GetClient(ctx, validClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, &validClient) {
		t.Fatalf("duplicate client failure mutated row:\n got=%#v\nwant=%#v", stored, &validClient)
	}

	invalidPermissionClient := model.OAuthClient{
		ID:          "invalid-permission-client",
		OwnerUserID: user.ID,
		Name:        "Invalid permission client",
		RedirectURI: "https://invalid.example/callback",
		ClientType:  "public",
		Status:      "pending",
		CreatedAt:   1100,
		UpdatedAt:   1100,
	}
	err = db.OAuth.CreateClient(ctx, invalidPermissionClient, []int64{9_999_999})
	assertPgCode(t, err, "23503")
	if got, err := db.OAuth.GetClient(ctx, invalidPermissionClient.ID); err != nil || got != nil {
		t.Fatalf("client with invalid permission should roll back: client=%#v err=%v", got, err)
	}
	assertPermissionSubjectAbsent(t, db, permissiondb.SubjectIDForClient(invalidPermissionClient.ID))

	grantWithMissingClient := model.OAuthGrant{
		ID:        "grant-missing-client",
		UserID:    user.ID,
		SubjectID: permissiondb.SubjectIDForUser(user.ID),
		ClientID:  "missing-client",
		Status:    "active",
		CreatedAt: 1200,
	}
	err = db.OAuth.CreateGrant(ctx, grantWithMissingClient, []int64{})
	assertPgCode(t, err, "23503")
	if grants, err := db.OAuth.ListGrantsByUser(ctx, user.ID, 10); err != nil || len(grants) != 0 {
		t.Fatalf("grant with missing client should roll back: grants=%#v err=%v", grants, err)
	}

	grantWithInvalidPermission := grantWithMissingClient
	grantWithInvalidPermission.ID = "grant-invalid-permission"
	grantWithInvalidPermission.ClientID = validClient.ID
	err = db.OAuth.CreateGrant(ctx, grantWithInvalidPermission, []int64{9_999_999})
	assertPgCode(t, err, "23503")
	if permissions, err := db.OAuth.GrantPermissionIDs(ctx, grantWithInvalidPermission.ID); err != nil || len(permissions) != 0 {
		t.Fatalf("invalid grant permission should roll back permissions: permissions=%v err=%v", permissions, err)
	}

	validGrant := model.OAuthGrant{
		ID:        "valid-grant",
		UserID:    user.ID,
		SubjectID: permissiondb.SubjectIDForUser(user.ID),
		ClientID:  validClient.ID,
		Status:    "active",
		CreatedAt: 1300,
	}
	if err := db.OAuth.CreateGrant(ctx, validGrant, permissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}

	codeWithMissingGrant := model.OAuthAuthorizationCode{
		CodeHash:            "code-missing-grant",
		ClientID:            validClient.ID,
		UserID:              user.ID,
		GrantID:             "missing-grant",
		RedirectURI:         validClient.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           5000,
		CreatedAt:           1400,
	}
	err = db.OAuth.CreateAuthorizationCode(ctx, codeWithMissingGrant, []int64{})
	assertPgCode(t, err, "23503")
	if code, permissions, err := db.OAuth.ConsumeAuthorizationCode(ctx, codeWithMissingGrant.CodeHash, 1500); err != nil || code != nil || permissions != nil {
		t.Fatalf("authorization code with missing grant should roll back: code=%#v permissions=%v err=%v", code, permissions, err)
	}

	codeWithInvalidPermission := codeWithMissingGrant
	codeWithInvalidPermission.CodeHash = "code-invalid-permission"
	codeWithInvalidPermission.GrantID = validGrant.ID
	err = db.OAuth.CreateAuthorizationCode(ctx, codeWithInvalidPermission, []int64{9_999_999})
	assertPgCode(t, err, "23503")
	if code, permissions, err := db.OAuth.ConsumeAuthorizationCode(ctx, codeWithInvalidPermission.CodeHash, 1500); err != nil || code != nil || permissions != nil {
		t.Fatalf("authorization code with invalid permission should roll back: code=%#v permissions=%v err=%v", code, permissions, err)
	}

	refreshWithMissingGrant := model.OAuthToken{
		TokenHash: "refresh-missing-grant",
		ClientID:  validClient.ID,
		UserID:    user.ID,
		GrantID:   "missing-grant",
		ExpiresAt: 6000,
		CreatedAt: 1500,
	}
	err = db.OAuth.CreateRefreshToken(ctx, refreshWithMissingGrant)
	assertPgCode(t, err, "23503")
	if refresh, err := db.OAuth.GetRefreshToken(ctx, refreshWithMissingGrant.TokenHash); err != nil || refresh != nil {
		t.Fatalf("refresh token with missing grant should roll back: token=%#v err=%v", refresh, err)
	}

	deviceWithMissingClient := model.OAuthDeviceCode{
		DeviceCodeHash: "device-missing-client",
		UserCodeHash:   "user-code-missing-client",
		ClientID:       "missing-client",
		Status:         "pending",
		ExpiresAt:      6000,
		CreatedAt:      1600,
	}
	err = db.OAuth.CreateDeviceCode(ctx, deviceWithMissingClient, []int64{})
	assertPgCode(t, err, "23503")
	if device, permissions, err := db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, deviceWithMissingClient.DeviceCodeHash); err != nil || device != nil || permissions != nil {
		t.Fatalf("device code with missing client should roll back: device=%#v permissions=%v err=%v", device, permissions, err)
	}

	deviceWithInvalidPermission := deviceWithMissingClient
	deviceWithInvalidPermission.DeviceCodeHash = "device-invalid-permission"
	deviceWithInvalidPermission.UserCodeHash = "user-code-invalid-permission"
	deviceWithInvalidPermission.ClientID = validClient.ID
	err = db.OAuth.CreateDeviceCode(ctx, deviceWithInvalidPermission, []int64{9_999_999})
	assertPgCode(t, err, "23503")
	if device, permissions, err := db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, deviceWithInvalidPermission.DeviceCodeHash); err != nil || device != nil || permissions != nil {
		t.Fatalf("device code with invalid permission should roll back: device=%#v permissions=%v err=%v", device, permissions, err)
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
	if ok, err := db.OAuth.ApproveDeviceCode(ctx, code.UserCodeHash, user.ID, subjectID, 1250); err != nil || ok {
		t.Fatalf("ApproveDeviceCode should reject non-pending code: ok=%v err=%v", ok, err)
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
	if err := db.OAuth.MarkDeviceCodePolled(ctx, code.DeviceCodeHash, 1260); err != nil {
		t.Fatal(err)
	}
	got, _, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, code.DeviceCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	lastPolledAt := int64(1260)
	wantApproved.LastPolledAt = &lastPolledAt
	if !reflect.DeepEqual(got, &wantApproved) {
		t.Fatalf("polled device code mismatch:\n got=%#v\nwant=%#v", got, &wantApproved)
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
	if ok, err := db.OAuth.DenyDeviceCode(ctx, denied.UserCodeHash, 1510); err != nil || ok {
		t.Fatalf("DenyDeviceCode should reject non-pending code: ok=%v err=%v", ok, err)
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
	missingCode, missingPermissions, err := db.OAuth.GetDeviceCodeByUserCodeHash(ctx, "missing-user-code")
	if err != nil {
		t.Fatal(err)
	}
	if missingCode != nil || missingPermissions != nil {
		t.Fatalf("missing device code should return nils: code=%#v permissions=%v", missingCode, missingPermissions)
	}
	missingCode, missingPermissions, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, "missing-device-code")
	if err != nil {
		t.Fatal(err)
	}
	if missingCode != nil || missingPermissions != nil {
		t.Fatalf("missing device hash should return nils: code=%#v permissions=%v", missingCode, missingPermissions)
	}
	if err := db.OAuth.MarkDeviceCodePolled(ctx, "missing-device-code", 1600); err != nil {
		t.Fatal(err)
	}
	expired := code
	expired.DeviceCodeHash = "device-hash-expired"
	expired.UserCodeHash = "user-hash-expired"
	expired.ExpiresAt = 1000
	if err := db.OAuth.CreateDeviceCode(ctx, expired, permissions[:1]); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.OAuth.ApproveDeviceCode(ctx, expired.UserCodeHash, user.ID, subjectID, 1200); err != nil || ok {
		t.Fatalf("ApproveDeviceCode should reject expired pending code: ok=%v err=%v", ok, err)
	}
	if ok, err := db.OAuth.DenyDeviceCode(ctx, expired.UserCodeHash, 1200); err != nil || ok {
		t.Fatalf("DenyDeviceCode should reject expired pending code: ok=%v err=%v", ok, err)
	}
	got, gotPermissions, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, expired.DeviceCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &expired) || !reflect.DeepEqual(gotPermissions, permissions[:1]) {
		t.Fatalf("expired device code should remain pending: got=%#v perms=%v", got, gotPermissions)
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

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("PostgreSQL error code mismatch: err=%v code=%q want %q", err, pgErrCode(pgErr), code)
	}
}

func pgErrCode(err *pgconn.PgError) string {
	if err == nil {
		return ""
	}
	return err.Code
}

func assertPermissionSubjectAbsent(t *testing.T, db *database.DB, subjectID string) {
	t.Helper()
	var count int
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM permission_subjects WHERE id=$1
	`, subjectID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("permission subject %q count=%d, want 0", subjectID, count)
	}
}
