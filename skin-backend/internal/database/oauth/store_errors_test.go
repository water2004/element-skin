package oauth_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

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
		{name: "create client", call: func() error { return db.OAuth.CreateClient(ctx, client, permissionIDs("account.read.self"), nil) }},
		{name: "update client", call: func() error {
			_, err := db.OAuth.UpdateClient(ctx, client, permissionIDs("account.read.self"), nil)
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
		{name: "client has permission", call: func() error {
			_, err := db.OAuth.ClientHasPermission(ctx, client.ID, 1)
			return err
		}},
		{name: "create grant", call: func() error { return db.OAuth.CreateGrant(ctx, grant, permissionIDs("account.read.self")) }},
		{name: "revoke grant", call: func() error {
			_, err := db.OAuth.RevokeGrant(ctx, grant.ID, grant.UserID, 2000)
			return err
		}},
		{name: "revoke inactive grants", call: func() error {
			_, err := db.OAuth.RevokeInactiveGrants(ctx, 2000, 1000)
			return err
		}},
		{name: "delete revoked grants", call: func() error {
			_, err := db.OAuth.DeleteRevokedGrants(ctx, 2000)
			return err
		}},
		{name: "list grants", call: func() error {
			_, err := db.OAuth.ListGrantsByUser(ctx, grant.UserID, 10)
			return err
		}},
		{name: "admin list grants", call: func() error {
			_, err := db.OAuth.ListGrantsForAdmin(ctx, 10)
			return err
		}},
		{name: "grant permission ids", call: func() error {
			_, err := db.OAuth.GrantPermissionIDs(ctx, grant.ID)
			return err
		}},
		{name: "authorization permission state", call: func() error {
			_, err := db.OAuth.AuthorizationPermissionState(ctx, grant.UserID, grant.ClientID, 1, 2)
			return err
		}},
		{name: "create authorization code", call: func() error {
			return db.OAuth.CreateAuthorizationCode(ctx, code, permissionIDs("account.read.self"))
		}},
		{name: "consume authorization code", call: func() error {
			_, _, err := db.OAuth.ConsumeAuthorizationCode(ctx, code.CodeHash, code.ClientID, code.RedirectURI, 1500)
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
	if err := db.OAuth.CreateClient(ctx, validClient, []int64{}, nil); err != nil {
		t.Fatal(err)
	}

	duplicate := validClient
	duplicate.Name = "Duplicate should not win"
	err := db.OAuth.CreateClient(ctx, duplicate, []int64{}, nil)
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
	err = db.OAuth.CreateClient(ctx, invalidPermissionClient, []int64{9_999_999}, nil)
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
	if code, permissions, err := db.OAuth.ConsumeAuthorizationCode(ctx, codeWithMissingGrant.CodeHash, codeWithMissingGrant.ClientID, codeWithMissingGrant.RedirectURI, 1500); err != nil || code != nil || permissions != nil {
		t.Fatalf("authorization code with missing grant should roll back: code=%#v permissions=%v err=%v", code, permissions, err)
	}

	codeWithInvalidPermission := codeWithMissingGrant
	codeWithInvalidPermission.CodeHash = "code-invalid-permission"
	codeWithInvalidPermission.GrantID = validGrant.ID
	err = db.OAuth.CreateAuthorizationCode(ctx, codeWithInvalidPermission, []int64{9_999_999})
	assertPgCode(t, err, "23503")
	if code, permissions, err := db.OAuth.ConsumeAuthorizationCode(ctx, codeWithInvalidPermission.CodeHash, codeWithInvalidPermission.ClientID, codeWithInvalidPermission.RedirectURI, 1500); err != nil || code != nil || permissions != nil {
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
