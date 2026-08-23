package webhook_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestPriorityWebhookTriggersEmitOnlyMeaningfulResourceChangesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-p0-owner@test.com", "Password123", "WebhookP0Owner", false)
	permissionCodes := []string{
		"account.read.any",
		"oauth_grant.read.owned",
		"official_whitelist.read.any",
		"permission.read.any",
	}
	permissionIDs := make([]int64, 0, len(permissionCodes))
	for _, code := range permissionCodes {
		permissionIDs = append(permissionIDs, int64(permission.MustDefinitionByCode(code).ID))
	}
	client := model.OAuthClient{
		ID: "webhook-p0-client", OwnerUserID: owner.ID, Name: "Webhook P0 client",
		ClientType: "confidential", SecretHash: "secret", Status: "active", CreatedAt: 1000, UpdatedAt: 1000,
	}
	endpoint := model.WebhookEndpoint{
		ID: "wh_p0", ClientID: client.ID, URL: "https://hooks.example/p0", SecretCiphertext: "ciphertext",
		Status: "active", CreatedAt: 1000, UpdatedAt: 1000,
		EventTypes: []string{
			"account.created", "account.updated", "account.deleted", "oauth_grant.revoked",
			"official_whitelist.added", "official_whitelist.removed", "permission.updated",
		},
	}
	if err := db.OAuth.CreateClient(ctx, client, permissionIDs, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}

	target := testutil.CreateUser(t, db, "webhook-p0-target@test.com", "Password123", "WebhookP0Target", false)
	assertWebhookEventsExactly(t, db, map[string][]map[string]any{
		"account.created":    {{"user_id": target.ID}},
		"permission.updated": {{"user_id": target.ID}},
	})

	if err := db.Users.Update(ctx, target.ID, map[string]any{"display_name": target.DisplayName}); err != nil {
		t.Fatal(err)
	}
	if err := db.Users.UpdatePassword(ctx, target.ID, "new-password-hash"); err != nil {
		t.Fatal(err)
	}
	if err := db.Users.Update(ctx, target.ID, map[string]any{
		"display_name": "WebhookP0TargetUpdated", "preferred_language": "en_US",
	}); err != nil {
		t.Fatal(err)
	}
	assertWebhookEventsExactly(t, db, map[string][]map[string]any{
		"account.created":    {{"user_id": target.ID}},
		"account.updated":    {{"user_id": target.ID}},
		"permission.updated": {{"user_id": target.ID}},
	})

	if err := db.Permissions.GrantRole(ctx, target.ID, permission.RoleAdmin, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.GrantRole(ctx, target.ID, permission.RoleAdmin, permissiondb.SubjectIDForUser(owner.ID)); err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.Permissions.RevokeRole(ctx, target.ID, permission.RoleAdmin); err != nil || !revoked {
		t.Fatalf("revoke role revoked=%v err=%v", revoked, err)
	}
	profileRead := permission.MustDefinitionByCode("profile.read.owned")
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, target.ID, profileRead, "allow", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, target.ID, profileRead, "allow", permissiondb.SubjectIDForUser(owner.ID)); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, target.ID, profileRead, "deny", ""); err != nil {
		t.Fatal(err)
	}
	if cleared, err := db.Permissions.ClearSubjectPermissionOverride(ctx, target.ID, profileRead); err != nil || !cleared {
		t.Fatalf("clear override cleared=%v err=%v", cleared, err)
	}
	assertWebhookEventCountExactly(t, db, "permission.updated", 6)

	var fallbackEndpointID int
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO fallback_endpoints (
			priority,session_url,account_url,services_url,cache_ttl,
			enable_profile,enable_hasjoined,enable_whitelist,note
		) VALUES (1,'https://fallback.example/session','https://fallback.example/account',
			'https://fallback.example/services',60,TRUE,TRUE,TRUE,'webhook test')
		RETURNING id
	`).Scan(&fallbackEndpointID); err != nil {
		t.Fatal(err)
	}
	if err := db.Fallbacks.AddWhitelistUser(ctx, "WebhookPlayer", fallbackEndpointID); err != nil {
		t.Fatal(err)
	}
	if err := db.Fallbacks.AddWhitelistUser(ctx, "WebhookPlayer", fallbackEndpointID); err != nil {
		t.Fatal(err)
	}
	if err := db.Fallbacks.RemoveWhitelistUser(ctx, "WebhookPlayer", fallbackEndpointID); err != nil {
		t.Fatal(err)
	}
	assertWebhookEventsForTypeExactly(t, db, "official_whitelist.added", []map[string]any{{
		"endpoint_id": float64(fallbackEndpointID), "username": "WebhookPlayer",
	}})
	assertWebhookEventsForTypeExactly(t, db, "official_whitelist.removed", []map[string]any{{
		"endpoint_id": float64(fallbackEndpointID), "username": "WebhookPlayer",
	}})

	grant := model.OAuthGrant{
		ID: "webhook-p0-grant", UserID: target.ID, SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID: client.ID, Status: "active", CreatedAt: 2000,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)}); err != nil {
		t.Fatal(err)
	}
	deleted, err := db.Users.Delete(ctx, target.ID)
	if err != nil || !deleted {
		t.Fatalf("delete target deleted=%v err=%v", deleted, err)
	}
	assertWebhookEventsForTypeExactly(t, db, "oauth_grant.revoked", []map[string]any{{
		"grant_id": grant.ID, "user_id": target.ID,
	}})
	assertWebhookEventsForTypeExactly(t, db, "account.deleted", []map[string]any{{"user_id": target.ID}})
	assertWebhookEventCountExactly(t, db, "permission.updated", 6)
}

func TestPriorityWebhookTriggerFailuresLeaveNoEventsOrBusinessRowsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-p0-failure-owner@test.com", "Password123", "WebhookP0FailureOwner", false)
	target := testutil.CreateUser(t, db, "webhook-p0-failure-target@test.com", "Password123", "WebhookP0FailureTarget", false)
	permissionIDs := []int64{
		int64(permission.MustDefinitionByCode("account.read.any").ID),
		int64(permission.MustDefinitionByCode("official_whitelist.read.any").ID),
		int64(permission.MustDefinitionByCode("permission.read.any").ID),
	}
	client := model.OAuthClient{
		ID: "webhook-p0-failure-client", OwnerUserID: owner.ID, Name: "Webhook P0 failure client",
		ClientType: "confidential", SecretHash: "secret", Status: "active", CreatedAt: 1000, UpdatedAt: 1000,
	}
	endpoint := model.WebhookEndpoint{
		ID: "wh_p0_failure", ClientID: client.ID, URL: "https://hooks.example/p0-failure", SecretCiphertext: "ciphertext",
		Status: "active", CreatedAt: 1000, UpdatedAt: 1000,
		EventTypes: []string{"account.created", "official_whitelist.added", "permission.updated"},
	}
	if err := db.OAuth.CreateClient(ctx, client, permissionIDs, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	duplicate := model.User{
		ID: "webhook-p0-duplicate", Email: target.Email, Password: "hash",
		PreferredLanguage: "zh_CN", DisplayName: "WebhookP0Duplicate",
	}
	if err := db.Users.Create(ctx, duplicate); err == nil {
		t.Fatal("duplicate account create should fail")
	}
	if err := db.Permissions.GrantRole(ctx, target.ID, "missing-webhook-role", ""); err == nil {
		t.Fatal("missing permission role grant should fail")
	}
	if err := db.Fallbacks.AddWhitelistUser(ctx, "MissingEndpointPlayer", 999999); err == nil {
		t.Fatal("missing whitelist endpoint add should fail")
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET display_name='RolledBackWebhookName' WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var events, duplicateUsers, missingWhitelistRows int
	var displayName string
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, duplicate.ID).Scan(&duplicateUsers); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM whitelisted_users WHERE username='MissingEndpointPlayer'`).Scan(&missingWhitelistRows); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT display_name FROM users WHERE id=$1`, target.ID).Scan(&displayName); err != nil {
		t.Fatal(err)
	}
	if events != 0 || duplicateUsers != 0 || missingWhitelistRows != 0 || displayName != target.DisplayName {
		t.Fatalf("failed mutations left events=%d duplicate_users=%d whitelist_rows=%d display_name=%q", events, duplicateUsers, missingWhitelistRows, displayName)
	}
}

func assertWebhookEventsExactly(t *testing.T, db *database.DB, want map[string][]map[string]any) {
	t.Helper()
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM webhook_events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	wantCount := 0
	for eventType, events := range want {
		wantCount += len(events)
		assertWebhookEventsForTypeExactly(t, db, eventType, events)
	}
	if count != wantCount {
		t.Fatalf("webhook event total=%d want=%d", count, wantCount)
	}
}

func assertWebhookEventCountExactly(t *testing.T, db *database.DB, eventType string, want int) {
	t.Helper()
	var got int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM webhook_events WHERE event_type=$1`, eventType).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("webhook event %q count=%d want=%d", eventType, got, want)
	}
}

func assertWebhookEventsForTypeExactly(t *testing.T, db *database.DB, eventType string, want []map[string]any) {
	t.Helper()
	rows, err := db.Pool.Query(context.Background(), `
		SELECT data FROM webhook_events WHERE event_type=$1 ORDER BY created_at,id
	`, eventType)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		got = append(got, data)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("webhook event %q data=%#v want=%#v", eventType, got, want)
	}
}
