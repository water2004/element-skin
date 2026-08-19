package oauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServiceClientManagementReviewSecretDeleteAndAdminListExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-owner-manage@test.com", "Password123", "OAuthOwnerManage", false)
	other := testutil.CreateUser(t, db, "oauth-other-manage@test.com", "Password123", "OAuthOtherManage", false)
	admin := testutil.CreateUser(t, db, "oauth-admin-manage@test.com", "Password123", "OAuthAdminManage", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := db.Permissions.ActorForUser(ctx, other.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)

	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Managed app",
		Description:     "Original description",
		RedirectURI:     "https://managed.example/callback",
		WebsiteURL:      "https://managed.example",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "account.update.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	firstSecret := created["client_secret"].(string)
	if clientID == "" || firstSecret == "" || created["status"] != oauth.StatusPending {
		t.Fatalf("created client mismatch: %#v", created)
	}
	if _, err := svc.GetClient(ctx, otherActor, clientID); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("other user get client error mismatch: %#v", err)
	}
	gotClient, err := svc.GetClient(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if gotClient["client_id"] != clientID ||
		gotClient["name"] != "Managed app" ||
		gotClient["description"] != "Original description" ||
		gotClient["redirect_uri"] != "https://managed.example/callback" ||
		gotClient["website_url"] != "https://managed.example" ||
		gotClient["client_type"] != oauth.ClientTypeConfidential ||
		gotClient["status"] != oauth.StatusPending {
		t.Fatalf("owned client detail mismatch: %#v", gotClient)
	}

	ownedList, err := svc.ListClients(ctx, ownerActor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownedList) != 1 || ownedList[0]["client_id"] != clientID || ownedList[0]["name"] != "Managed app" {
		t.Fatalf("owned list mismatch: %#v", ownedList)
	}
	if _, err := svc.ListClients(ctx, permission.Actor{}, 10); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("list without owned permission error mismatch: %#v", err)
	}
	if _, err := svc.ListClientsForAdmin(ctx, adminActor, "weird", 10); !isHTTPError(err, 400, "status.validate.invalid") {
		t.Fatalf("admin list invalid status error mismatch: %#v", err)
	}
	pendingList, err := svc.ListClientsForAdmin(ctx, adminActor, oauth.StatusPending, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pendingList) != 1 || pendingList[0]["client_id"] != clientID || pendingList[0]["status"] != oauth.StatusPending {
		t.Fatalf("pending admin list mismatch: %#v", pendingList)
	}
	if pendingList[0]["name"] != "Managed app" ||
		pendingList[0]["description"] != "Original description" ||
		pendingList[0]["client_type"] != oauth.ClientTypeConfidential {
		t.Fatalf("pending admin summary fields mismatch: %#v", pendingList[0])
	}
	if _, ok := pendingList[0]["permissions"]; ok {
		t.Fatalf("pending admin list must not load permissions: %#v", pendingList[0])
	}
	if _, ok := pendingList[0]["redirect_uri"]; ok {
		t.Fatalf("pending admin list must not load redirect uri: %#v", pendingList[0])
	}
	allList, err := svc.ListClientsForAdmin(ctx, adminActor, "all", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(allList) != 1 || allList[0]["client_id"] != clientID || allList[0]["status"] != oauth.StatusPending {
		t.Fatalf("all admin list mismatch: %#v", allList)
	}

	updated, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
		Name:            "Managed app updated",
		Description:     "Updated description",
		RedirectURI:     "https://managed.example/new-callback",
		WebsiteURL:      "https://managed.example/docs",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, oauth.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Managed app updated" || updated["status"] != oauth.StatusPending ||
		updated["redirect_uri"] != "https://managed.example/new-callback" {
		t.Fatalf("owner update should preserve pending status and update fields: %#v", updated)
	}
	submitted, err := svc.SubmitClientForReview(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != oauth.StatusPending {
		t.Fatalf("submitted client should be pending: %#v", submitted)
	}
	if _, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusPending, ""); !isHTTPError(err, 400, "status.validate.invalid") {
		t.Fatalf("review pending status error mismatch: %#v", err)
	}
	reviewed, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, "")
	if err != nil {
		t.Fatal(err)
	}
	if reviewed["status"] != oauth.StatusActive || reviewed["client_id"] != clientID {
		t.Fatalf("reviewed client mismatch: %#v", reviewed)
	}
	rotated, err := svc.RotateClientSecret(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated["client_secret"] == "" || rotated["client_secret"] == firstSecret || rotated["status"] != oauth.StatusActive {
		t.Fatalf("rotated secret mismatch: %#v", rotated)
	}
	if err := svc.Redis.SetOAuthAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash: "managed-client-delete-access",
		ClientID:  clientID,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteClient(ctx, otherActor, clientID); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("other delete error mismatch: %#v", err)
	}
	if token, err := svc.Redis.GetOAuthAccessToken(ctx, "managed-client-delete-access"); err != nil || token.ClientID != clientID {
		t.Fatalf("failed client delete must preserve access token: token=%#v err=%v", token, err)
	}
	if err := svc.DeleteClient(ctx, ownerActor, clientID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Redis.GetOAuthAccessToken(ctx, "managed-client-delete-access"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("deleted client access token should be removed exactly, got %v", err)
	}
	if _, err := svc.GetClient(ctx, ownerActor, clientID); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("deleted client get error mismatch: %#v", err)
	}
}

func TestServiceClientManagementRejectsUnauthorizedMissingAndInvalidStateExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-owner-reject@test.com", "Password123", "OAuthOwnerReject", false)
	admin := testutil.CreateUser(t, db, "oauth-admin-reject@test.com", "Password123", "OAuthAdminReject", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)

	if _, err := svc.CreateClient(ctx, permission.Actor{}, oauth.ClientInput{}); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("create without permission mismatch: %#v", err)
	}
	if _, err := svc.ListClientsForAdmin(ctx, permission.Actor{}, "all", 10); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("admin list without permission mismatch: %#v", err)
	}
	if _, err := svc.GetClient(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("get missing client mismatch: %#v", err)
	}
	if _, err := svc.UpdateClient(ctx, ownerActor, "missing-client", oauth.ClientInput{
		Name:            "Missing",
		RedirectURI:     "https://missing.example/callback",
		PermissionCodes: []string{"account.read.self"},
	}, "active"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("update missing client mismatch: %#v", err)
	}
	if _, err := svc.SubmitClientForReview(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("submit missing client mismatch: %#v", err)
	}
	if _, err := svc.ReviewClient(ctx, permission.Actor{}, "missing-client", oauth.StatusActive, ""); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("review without permission mismatch: %#v", err)
	}
	if _, err := svc.ReviewClient(ctx, adminActor, "missing-client", oauth.StatusActive, ""); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("review missing client mismatch: %#v", err)
	}
	if _, err := svc.RotateClientSecret(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("rotate missing client mismatch: %#v", err)
	}
	if err := svc.DeleteClient(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("delete missing client mismatch: %#v", err)
	}
	if _, err := svc.ClientPermissions(ctx, adminActor, "missing-client"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("client permissions missing mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, permission.Actor{}, "missing-client", "account.read.self", "deny"); !isHTTPError(err, 403, "permission.check.denied") {
		t.Fatalf("set permission deny without revoke permission mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, "missing-client", "account.read.self", "allow"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("set permission missing client mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, "missing-client", "account.read.self"); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("clear permission missing client mismatch: %#v", err)
	}

	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Reject state app",
		RedirectURI:     "https://reject-state.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	keptStatus, err := svc.UpdateClient(ctx, adminActor, clientID, oauth.ClientInput{
		Name:            "Reject state app kept",
		RedirectURI:     "https://reject-state.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if keptStatus["status"] != oauth.StatusPending || keptStatus["name"] != "Reject state app kept" {
		t.Fatalf("admin update with empty status should preserve current status: %#v", keptStatus)
	}
	if _, err := svc.UpdateClient(ctx, adminActor, clientID, oauth.ClientInput{
		Name:            "Reject state app updated",
		RedirectURI:     "https://reject-state.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, "archived"); !isHTTPError(err, 400, "status.validate.invalid") {
		t.Fatalf("update invalid status mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "not.a.permission"); !isHTTPError(err, 400, "permission.validate.invalid") {
		t.Fatalf("clear invalid permission mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "account.read.self"); !isHTTPError(err, 404, "permission_override.resolve.not_found") {
		t.Fatalf("clear missing permission override mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "account.update.self", "deny"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "account.update.self"); err != nil {
		t.Fatalf("clear existing permission override failed: %v", err)
	}
	if err := svc.DeleteClient(ctx, adminActor, clientID); err != nil {
		t.Fatalf("admin delete client failed: %v", err)
	}
	if _, err := svc.GetClient(ctx, adminActor, clientID); !isHTTPError(err, 404, "oauth_client.resolve.not_found") {
		t.Fatalf("admin deleted client should be gone: %#v", err)
	}
}
