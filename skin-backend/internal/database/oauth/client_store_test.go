package oauth_test

import (
	"context"
	"reflect"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
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
	if err := db.OAuth.CreateClient(ctx, client, initialPermissions, nil); err != nil {
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
	profilePermission := initialPermissions[0]
	noticePermission := permissionIDs("notice.read.owned")[0]
	if allowed, err := db.OAuth.ClientHasPermission(ctx, client.ID, profilePermission); err != nil || !allowed {
		t.Fatalf("active client requested permission allowed=%v err=%v", allowed, err)
	}
	if allowed, err := db.OAuth.ClientHasPermission(ctx, client.ID, noticePermission); err != nil || allowed {
		t.Fatalf("unrequested client permission allowed=%v err=%v", allowed, err)
	}
	if allowed, err := db.OAuth.ClientHasPermission(ctx, "missing-client", profilePermission); err != nil || allowed {
		t.Fatalf("missing client permission allowed=%v err=%v", allowed, err)
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
	if err := db.OAuth.CreateClient(ctx, otherClient, []int64{}, nil); err != nil {
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
	updated, err := db.OAuth.UpdateClient(ctx, client, updatedPermissions, nil)
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
	if allowed, err := db.OAuth.ClientHasPermission(ctx, client.ID, updatedPermissions[0]); err != nil || allowed {
		t.Fatalf("disabled client permission allowed=%v err=%v", allowed, err)
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
	if updated, err := db.OAuth.UpdateClient(ctx, missingUpdate, updatedPermissions, nil); err != nil || updated {
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
