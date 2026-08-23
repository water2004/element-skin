package webhook_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"element-skin/backend/internal/database"
	webhookdb "element-skin/backend/internal/database/webhook"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"
)

func TestSubscriptionSnapshotTracksEffectiveSubscriptionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-snapshot@test.com", "Password123", "WebhookSnapshot", false)
	now := database.NowMS()

	first := snapshotClient("snapshot-first", owner.ID, "active", now)
	firstEndpoint := snapshotEndpoint("snapshot-first-endpoint", first.ID, "active", now, "profile.updated", "account.created")
	if err := db.OAuth.CreateClient(ctx, first, nil, []model.WebhookEndpoint{firstEndpoint}); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"account.created": 1, "profile.updated": 1})

	second := snapshotClient("snapshot-second", owner.ID, "pending", now+1)
	secondEndpoint := snapshotEndpoint("snapshot-second-endpoint", second.ID, "active", now+1, "profile.updated")
	if err := db.OAuth.CreateClient(ctx, second, nil, []model.WebhookEndpoint{secondEndpoint}); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"account.created": 1, "profile.updated": 1})

	if updated, err := db.OAuth.UpdateClientStatus(ctx, second.ID, "active", now+2); err != nil || !updated {
		t.Fatalf("activate second client: updated=%v err=%v", updated, err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"account.created": 1, "profile.updated": 2})

	second.Status = "active"
	second.UpdatedAt = now + 3
	secondEndpoint.Status = "disabled"
	secondEndpoint.UpdatedAt = second.UpdatedAt
	if updated, err := db.OAuth.UpdateClient(ctx, second, nil, []model.WebhookEndpoint{secondEndpoint}); err != nil || !updated {
		t.Fatalf("disable second endpoint: updated=%v err=%v", updated, err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"account.created": 1, "profile.updated": 1})

	if deleted, err := db.OAuth.DeleteClient(ctx, first.ID, owner.ID); err != nil || !deleted {
		t.Fatalf("delete first client: deleted=%v err=%v", deleted, err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{})
}

func TestSubscriptionSnapshotSerializesConcurrentActivations(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-snapshot-concurrent@test.com", "Password123", "WebhookConcurrent", false)
	now := database.NowMS()
	clients := []model.OAuthClient{
		snapshotClient("snapshot-concurrent-profile", owner.ID, "pending", now),
		snapshotClient("snapshot-concurrent-account", owner.ID, "pending", now+1),
	}
	eventTypes := []string{"profile.updated", "account.updated"}
	for index, client := range clients {
		endpoint := snapshotEndpoint(fmt.Sprintf("snapshot-concurrent-endpoint-%d", index), client.ID, "active", now+int64(index), eventTypes[index])
		if err := db.OAuth.CreateClient(ctx, client, nil, []model.WebhookEndpoint{endpoint}); err != nil {
			t.Fatal(err)
		}
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{})

	start := make(chan struct{})
	errors := make(chan error, len(clients))
	var wait sync.WaitGroup
	for index := range clients {
		client := clients[index]
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			updated, err := db.OAuth.UpdateClientStatus(ctx, client.ID, "active", now+10)
			if err == nil && !updated {
				err = fmt.Errorf("client %s was not updated", client.ID)
			}
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"account.updated": 1, "profile.updated": 1})
}

func TestSubscriptionSnapshotRebuildsExactlyDuringSchemaInit(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-snapshot-init@test.com", "Password123", "WebhookInit", false)
	now := database.NowMS()
	client := snapshotClient("snapshot-init-client", owner.ID, "active", now)
	endpoint := snapshotEndpoint("snapshot-init-endpoint", client.ID, "active", now, "profile.updated")
	if err := db.OAuth.CreateClient(ctx, client, nil, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM webhook_active_event_types;
		INSERT INTO webhook_active_event_types (event_type, subscriber_count)
		VALUES ('stale.event', 9);
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"profile.updated": 1})
}

func TestSubscriptionSnapshotFailureRollsBackClientAndSnapshot(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-snapshot-rollback@test.com", "Password123", "WebhookRollback", false)
	now := database.NowMS()

	active := snapshotClient("snapshot-rollback-active", owner.ID, "active", now)
	activeEndpoint := snapshotEndpoint("snapshot-rollback-active-endpoint", active.ID, "active", now, "profile.updated")
	if err := db.OAuth.CreateClient(ctx, active, nil, []model.WebhookEndpoint{activeEndpoint}); err != nil {
		t.Fatal(err)
	}
	pending := snapshotClient("snapshot-rollback-pending", owner.ID, "pending", now+1)
	pendingEndpoint := snapshotEndpoint("snapshot-rollback-pending-endpoint", pending.ID, "active", now+1, "account.updated")
	requestedPermission := permission.MustDefinitionByCode("profile.read.owned")
	if err := db.OAuth.CreateClient(ctx, pending, []int64{int64(requestedPermission.ID)}, []model.WebhookEndpoint{pendingEndpoint}); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"profile.updated": 1})

	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION fail_webhook_snapshot_insert() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced webhook snapshot failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_webhook_snapshot_insert
		BEFORE INSERT ON webhook_active_event_types
		FOR EACH ROW EXECUTE FUNCTION fail_webhook_snapshot_insert();
	`); err != nil {
		t.Fatal(err)
	}
	updated, err := db.OAuth.UpdateClientStatus(ctx, pending.ID, "active", now+2)
	if err == nil || updated {
		t.Fatalf("snapshot failure result: updated=%v err=%v", updated, err)
	}
	stored, getErr := db.OAuth.GetClient(ctx, pending.ID)
	if getErr != nil || stored == nil || stored.Status != "pending" || stored.UpdatedAt != pending.UpdatedAt {
		t.Fatalf("failed snapshot update changed client: client=%#v err=%v", stored, getErr)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"profile.updated": 1})

	disabled, disableErr := db.OAuth.DisableInvalidClientsForOwner(ctx, owner.ID, []int64{}, []int64{}, now+3)
	var pgErr *pgconn.PgError
	if disabled != nil || !errors.As(disableErr, &pgErr) || pgErr.Code != "P0001" || pgErr.Message != "forced webhook snapshot failure" {
		t.Fatalf("dependency snapshot failure: disabled=%#v err=%v pgErr=%#v", disabled, disableErr, pgErr)
	}
	stored, getErr = db.OAuth.GetClient(ctx, pending.ID)
	if getErr != nil || stored == nil || stored.Status != "pending" || stored.UpdatedAt != pending.UpdatedAt {
		t.Fatalf("failed dependency refresh changed client: client=%#v err=%v", stored, getErr)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"profile.updated": 1})
}

func TestDisableInvalidClientsForOwnerRefreshesSubscriptionSnapshot(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-snapshot-uncredentialed@test.com", "Password123", "WebhookUncredentialed", false)
	now := database.NowMS()
	client := snapshotClient("snapshot-uncredentialed-client", owner.ID, "active", now)
	endpoint := snapshotEndpoint("snapshot-uncredentialed-endpoint", client.ID, "active", now, "profile.updated")
	requestedPermission := permission.MustDefinitionByCode("profile.read.owned")
	if err := db.OAuth.CreateClient(ctx, client, []int64{int64(requestedPermission.ID)}, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{"profile.updated": 1})

	disabled, err := db.OAuth.DisableInvalidClientsForOwner(ctx, owner.ID, []int64{}, []int64{}, now+1)
	if err != nil || len(disabled) != 1 || disabled[0].ClientID != client.ID || disabled[0].OwnerUserID != owner.ID || disabled[0].Name != client.Name || disabled[0].UpdatedAt != now+1 {
		t.Fatalf("disabled client dependencies=%#v err=%v", disabled, err)
	}
	stored, err := db.OAuth.GetClient(ctx, client.ID)
	if err != nil || stored == nil || stored.Status != "disabled" || stored.UpdatedAt != now+1 {
		t.Fatalf("disabled client=%#v err=%v", stored, err)
	}
	assertSubscriptionSnapshot(t, db, map[string]int64{})
}

func TestDisableInvalidClientsForOwnerReturnsClosedPoolError(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	db.Pool.Close()
	disabled, err := db.OAuth.DisableInvalidClientsForOwner(t.Context(), "closed-pool-owner", nil, nil, database.NowMS())
	if disabled != nil || !errors.Is(err, puddle.ErrClosedPool) {
		t.Fatalf("closed pool dependency result=%#v err=%v", disabled, err)
	}
}

func TestRefreshSubscriptionSnapshotReturnsDeleteError(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	tx, err := db.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if _, err := tx.Exec(t.Context(), `ALTER TABLE webhook_active_event_types RENAME TO unavailable_webhook_active_event_types`); err != nil {
		t.Fatal(err)
	}
	err = webhookdb.RefreshSubscriptionSnapshot(t.Context(), tx)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
		t.Fatalf("snapshot delete error=%v pgErr=%#v", err, pgErr)
	}
}

func TestSubscriptionSnapshotTracksDependencyDisableAndOwnerCascade(t *testing.T) {
	t.Run("permission dependency disable", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		owner := testutil.CreateUser(t, db, "webhook-snapshot-dependency@test.com", "Password123", "WebhookDependency", false)
		now := database.NowMS()
		client := snapshotClient("snapshot-dependency-client", owner.ID, "active", now)
		endpoint := snapshotEndpoint("snapshot-dependency-endpoint", client.ID, "active", now, "profile.updated")
		requestedPermission := permission.MustDefinitionByCode("profile.read.owned")
		if err := db.OAuth.CreateClient(ctx, client, []int64{int64(requestedPermission.ID)}, []model.WebhookEndpoint{endpoint}); err != nil {
			t.Fatal(err)
		}
		assertSubscriptionSnapshot(t, db, map[string]int64{"profile.updated": 1})

		disabled, err := db.OAuth.DisableInvalidClientsForOwnerWithCredentials(ctx, owner.ID, []int64{}, []int64{}, now+1)
		if err != nil || len(disabled) != 1 || disabled[0].ClientID != client.ID || disabled[0].OwnerUserID != owner.ID || disabled[0].Name != client.Name || disabled[0].UpdatedAt != now+1 {
			t.Fatalf("disabled client dependencies=%#v err=%v", disabled, err)
		}
		assertSubscriptionSnapshot(t, db, map[string]int64{})
	})

	t.Run("owner cascade delete", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		owner := testutil.CreateUser(t, db, "webhook-snapshot-owner-delete@test.com", "Password123", "WebhookOwnerDelete", false)
		now := database.NowMS()
		client := snapshotClient("snapshot-owner-delete-client", owner.ID, "active", now)
		endpoint := snapshotEndpoint("snapshot-owner-delete-endpoint", client.ID, "active", now, "account.updated")
		if err := db.OAuth.CreateClient(ctx, client, nil, []model.WebhookEndpoint{endpoint}); err != nil {
			t.Fatal(err)
		}
		assertSubscriptionSnapshot(t, db, map[string]int64{"account.updated": 1})

		deleted, err := db.Users.Delete(ctx, owner.ID)
		if err != nil || !deleted {
			t.Fatalf("delete webhook client owner: deleted=%v err=%v", deleted, err)
		}
		assertSubscriptionSnapshot(t, db, map[string]int64{})
	})
}

func TestProfileTriggerUsesSubscriptionSnapshot(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-snapshot-trigger@test.com", "Password123", "WebhookTrigger", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "", "BeforeSnapshot")
	now := database.NowMS()
	client := snapshotClient("snapshot-trigger-client", owner.ID, "active", now)
	endpoint := snapshotEndpoint("snapshot-trigger-endpoint", client.ID, "active", now, "profile.updated")
	if err := db.OAuth.CreateClient(ctx, client, nil, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `TRUNCATE webhook_events CASCADE`); err != nil {
		t.Fatal(err)
	}
	if updated, err := db.Profiles.UpdateName(ctx, profile.ID, "WithSnapshot"); err != nil || !updated {
		t.Fatalf("update profile with snapshot: updated=%v err=%v", updated, err)
	}
	var eventType string
	var subjectUserID string
	var data []byte
	if err := db.Pool.QueryRow(ctx, `
		SELECT event_type, subject_user_id, data
		FROM webhook_events
	`).Scan(&eventType, &subjectUserID, &data); err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	wantPayload := map[string]string{"profile_id": profile.ID, "user_id": owner.ID}
	if eventType != "profile.updated" || subjectUserID != owner.ID || !reflect.DeepEqual(payload, wantPayload) {
		t.Fatalf("snapshot event mismatch: type=%q subject=%q data=%v", eventType, subjectUserID, payload)
	}

	if updated, err := db.OAuth.UpdateClientStatus(ctx, client.ID, "disabled", now+1); err != nil || !updated {
		t.Fatalf("disable client: updated=%v err=%v", updated, err)
	}
	if updated, err := db.Profiles.UpdateName(ctx, profile.ID, "WithoutSnapshot"); err != nil || !updated {
		t.Fatalf("update profile without snapshot: updated=%v err=%v", updated, err)
	}
	var eventCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_events`).Scan(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("events after disabling subscription=%d err=%v want=1", eventCount, err)
	}
}

func snapshotClient(id, ownerID, status string, now int64) model.OAuthClient {
	return model.OAuthClient{
		ID:          id,
		OwnerUserID: ownerID,
		Name:        id,
		ClientType:  "confidential",
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func snapshotEndpoint(id, clientID, status string, now int64, eventTypes ...string) model.WebhookEndpoint {
	return model.WebhookEndpoint{
		ID:               id,
		ClientID:         clientID,
		URL:              "https://webhook.example/" + id,
		SecretCiphertext: "snapshot-ciphertext",
		Status:           status,
		EventTypes:       eventTypes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func assertSubscriptionSnapshot(t *testing.T, db *database.DB, want map[string]int64) {
	t.Helper()
	rows, err := db.Pool.Query(t.Context(), `
		SELECT event_type, subscriber_count
		FROM webhook_active_event_types
		ORDER BY event_type
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make(map[string]int64)
	for rows.Next() {
		var eventType string
		var subscriberCount int64
		if err := rows.Scan(&eventType, &subscriberCount); err != nil {
			t.Fatal(err)
		}
		got[eventType] = subscriberCount
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscription snapshot=%v want=%v", got, want)
	}
	for eventType, count := range want {
		var active bool
		if err := db.Pool.QueryRow(t.Context(), `SELECT webhook_event_has_subscribers($1)`, eventType).Scan(&active); err != nil || !active {
			t.Fatalf("subscriber lookup %q=%v err=%v want=true (count=%d)", eventType, active, err, count)
		}
	}
	var missing bool
	if err := db.Pool.QueryRow(t.Context(), `SELECT webhook_event_has_subscribers('missing.event')`).Scan(&missing); err != nil || missing {
		t.Fatalf("missing subscriber lookup=%v err=%v want=false", missing, err)
	}
}
