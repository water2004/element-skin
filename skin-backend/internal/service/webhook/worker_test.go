package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestWorkerDispatchesAndSignsTargetedGrantEventExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "webhook-worker-grant@test.com", "Password123", "WebhookWorkerGrant", false)
	var gotBody []byte
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotHeaders = req.Header.Clone()
		var err error
		gotBody, err = io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read webhook body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg := testutil.TestConfig()
	box, err := util.NewSecretBox(cfg.IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	const signingSecret = "webhook-signing-secret"
	ciphertext, err := box.Encrypt(signingSecret)
	if err != nil {
		t.Fatal(err)
	}
	permissionID := int64(permission.MustDefinitionByCode("oauth_grant.read.owned").ID)
	client := model.OAuthClient{ID: "worker-grant-client", OwnerUserID: user.ID, Name: "Worker grant client", ClientType: "public", Status: "active", CreatedAt: 1000, UpdatedAt: 1000}
	endpoint := model.WebhookEndpoint{ID: "wh_worker_grant", ClientID: client.ID, URL: server.URL, SecretCiphertext: ciphertext, Status: "active", EventTypes: []string{"oauth_grant.revoked"}, CreatedAt: 1000, UpdatedAt: 1000}
	if err := db.OAuth.CreateClient(ctx, client, []int64{permissionID}, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	grant := model.OAuthGrant{ID: "worker-target-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 1100}
	if err := db.OAuth.CreateGrant(ctx, grant, []int64{permissionID}); err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 1200); err != nil || !revoked {
		t.Fatalf("revoke grant: revoked=%v err=%v", revoked, err)
	}

	fixedNow := time.UnixMilli(5000)
	worker := Worker{DB: db, Config: cfg, HTTPClient: server.Client(), Now: func() time.Time { return fixedNow }}
	if err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.DeliverBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var eventID, deliveryID, status string
	var createdAt int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT event.id, delivery.id, delivery.status, event.created_at
		FROM webhook_events AS event
		JOIN webhook_deliveries AS delivery ON delivery.event_id=event.id
		WHERE event.event_type='oauth_grant.revoked'
	`).Scan(&eventID, &deliveryID, &status, &createdAt); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" {
		t.Fatalf("delivered status=%q want succeeded", status)
	}
	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatal(err)
	}
	wantBody := map[string]any{
		"id":         eventID,
		"type":       "oauth_grant.revoked",
		"created_at": float64(createdAt),
		"data":       map[string]any{"grant_id": grant.ID, "user_id": user.ID},
	}
	if !reflect.DeepEqual(body, wantBody) {
		t.Fatalf("webhook body=%#v want=%#v", body, wantBody)
	}
	if gotHeaders.Get("Webhook-Id") != eventID || gotHeaders.Get("Webhook-Delivery") != deliveryID || gotHeaders.Get("Webhook-Timestamp") != "5000" || gotHeaders.Get("Webhook-Signature") != sign(signingSecret, "5000", gotBody) || gotHeaders.Get("Content-Type") != "application/json" {
		t.Fatalf("webhook headers mismatch: %#v", gotHeaders)
	}
}

func TestWorkerRechecksDelegatedPermissionBeforeDeliveryAndMarksStaleEventDeadExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "webhook-worker-permission@test.com", "Password123", "WebhookWorkerPermission", false)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg := testutil.TestConfig()
	box, _ := util.NewSecretBox(cfg.IdentityEncryptionKey)
	ciphertext, _ := box.Encrypt("permission-secret")
	profileReadID := int64(permission.MustDefinitionByCode("profile.read.owned").ID)
	client := model.OAuthClient{ID: "worker-permission-client", OwnerUserID: user.ID, Name: "Worker permission client", ClientType: "public", Status: "active", CreatedAt: 1000, UpdatedAt: 1000}
	endpoint := model.WebhookEndpoint{ID: "wh_worker_permission", ClientID: client.ID, URL: server.URL, SecretCiphertext: ciphertext, Status: "active", EventTypes: []string{"profile.updated"}, CreatedAt: 1000, UpdatedAt: 1000}
	if err := db.OAuth.CreateClient(ctx, client, []int64{profileReadID}, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	grant := model.OAuthGrant{ID: "worker-permission-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 1100}
	if err := db.OAuth.CreateGrant(ctx, grant, []int64{profileReadID}); err != nil {
		t.Fatal(err)
	}
	profile := model.Profile{ID: "worker-permission-profile", UserID: user.ID, Name: "WorkerPerm", TextureModel: "default"}
	if err := db.Profiles.Create(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Profiles.UpdateName(ctx, profile.ID, "WorkerPerm2"); err != nil {
		t.Fatal(err)
	}
	worker := Worker{DB: db, Config: cfg, HTTPClient: server.Client(), Now: func() time.Time { return time.UnixMilli(6000) }}
	if err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 6100); err != nil || !revoked {
		t.Fatalf("revoke delegated grant: revoked=%v err=%v", revoked, err)
	}
	if err := worker.DeliverBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var status, detail string
	var attempts int
	if err := db.Pool.QueryRow(ctx, `
		SELECT status, attempt_count, last_error
		FROM webhook_deliveries
	`).Scan(&status, &attempts, &detail); err != nil {
		t.Fatal(err)
	}
	if status != "dead" || attempts != 1 || detail != "permission no longer allows webhook event" || requests != 0 {
		t.Fatalf("stale delivery status=%q attempts=%d detail=%q requests=%d", status, attempts, detail, requests)
	}
}

func TestWorkerRetriesNonSuccessWithDeterministicBackoffExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "webhook-worker-retry@test.com", "Password123", "WebhookWorkerRetry", false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	cfg := testutil.TestConfig()
	box, _ := util.NewSecretBox(cfg.IdentityEncryptionKey)
	ciphertext, _ := box.Encrypt("retry-secret")
	permissionID := int64(permission.MustDefinitionByCode("oauth_grant.read.owned").ID)
	client := model.OAuthClient{ID: "worker-retry-client", OwnerUserID: user.ID, Name: "Worker retry client", ClientType: "public", Status: "active", CreatedAt: 1000, UpdatedAt: 1000}
	endpoint := model.WebhookEndpoint{ID: "wh_worker_retry", ClientID: client.ID, URL: server.URL, SecretCiphertext: ciphertext, Status: "active", EventTypes: []string{"oauth_grant.revoked"}, CreatedAt: 1000, UpdatedAt: 1000}
	if err := db.OAuth.CreateClient(ctx, client, []int64{permissionID}, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	grant := model.OAuthGrant{ID: "worker-retry-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 1100}
	if err := db.OAuth.CreateGrant(ctx, grant, []int64{permissionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 1200); err != nil {
		t.Fatal(err)
	}
	now := time.UnixMilli(7000)
	worker := Worker{DB: db, Config: cfg, HTTPClient: server.Client(), Now: func() time.Time { return now }}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var deliveryID, status, detail string
	var attempts, lastHTTPStatus int
	var nextAttemptAt int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT id, status, attempt_count, next_attempt_at, last_http_status, last_error
		FROM webhook_deliveries
	`).Scan(&deliveryID, &status, &attempts, &nextAttemptAt, &lastHTTPStatus, &detail); err != nil {
		t.Fatal(err)
	}
	wantNext := now.UnixMilli() + int64(retryDelay(deliveryID, 1)/time.Millisecond)
	if status != "pending" || attempts != 1 || nextAttemptAt != wantNext || lastHTTPStatus != 503 || detail != "webhook endpoint returned HTTP 503" {
		t.Fatalf("retry fields id=%q status=%q attempts=%d next=%d want_next=%d http=%d detail=%q", deliveryID, status, attempts, nextAttemptAt, wantNext, lastHTTPStatus, detail)
	}
	if _, err := db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET attempt_count=11, next_attempt_at=$2
		WHERE id=$1
	`, deliveryID, wantNext); err != nil {
		t.Fatal(err)
	}
	now = time.UnixMilli(wantNext)
	if err := worker.DeliverBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT status, attempt_count, last_http_status, last_error
		FROM webhook_deliveries WHERE id=$1
	`, deliveryID).Scan(&status, &attempts, &lastHTTPStatus, &detail); err != nil {
		t.Fatal(err)
	}
	if status != "dead" || attempts != 12 || lastHTTPStatus != 503 || detail != "webhook endpoint returned HTTP 503" {
		t.Fatalf("terminal retry status=%q attempts=%d http=%d detail=%q", status, attempts, lastHTTPStatus, detail)
	}
}

func TestWorkerUsesConfidentialApplicationPermissionAndStopsDisabledEndpointExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-worker-app-owner@test.com", "Password123", "WebhookWorkerAppOwner", false)
	target := testutil.CreateUser(t, db, "webhook-worker-app-target@test.com", "Password123", "WebhookWorkerAppTarget", false)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg := testutil.TestConfig()
	box, _ := util.NewSecretBox(cfg.IdentityEncryptionKey)
	ciphertext, _ := box.Encrypt("application-secret")
	definition := permission.MustDefinitionByCode("profile.read.any")
	client := model.OAuthClient{ID: "worker-application-client", OwnerUserID: owner.ID, Name: "Worker application client", ClientType: "confidential", Status: "active", CreatedAt: 1000, UpdatedAt: 1000}
	endpoint := model.WebhookEndpoint{ID: "wh_worker_application", ClientID: client.ID, URL: server.URL, SecretCiphertext: ciphertext, Status: "active", EventTypes: []string{"profile.created"}, CreatedAt: 1000, UpdatedAt: 1000}
	if err := db.OAuth.CreateClient(ctx, client, []int64{int64(definition.ID)}, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetPermissionOverrideForSubject(ctx, permissiondb.SubjectIDForClient(client.ID), definition, "allow", ""); err != nil {
		t.Fatal(err)
	}
	firstProfile := model.Profile{ID: "worker-application-profile-1", UserID: target.ID, Name: "WorkerApplication1", TextureModel: "default"}
	if err := db.Profiles.Create(ctx, firstProfile); err != nil {
		t.Fatal(err)
	}
	worker := Worker{DB: db, Config: cfg, HTTPClient: server.Client(), Now: func() time.Time { return time.UnixMilli(8000) }}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("application permission delivery requests=%d want=1", requests)
	}

	secondProfile := model.Profile{ID: "worker-application-profile-2", UserID: target.ID, Name: "WorkerApplication2", TextureModel: "default"}
	if err := db.Profiles.Create(ctx, secondProfile); err != nil {
		t.Fatal(err)
	}
	if err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE webhook_endpoints SET status='disabled' WHERE id=$1`, endpoint.ID); err != nil {
		t.Fatal(err)
	}
	if err := worker.DeliverBatch(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT event.data->>'profile_id', delivery.status
		FROM webhook_deliveries AS delivery
		JOIN webhook_events AS event ON event.id=delivery.event_id
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	statuses := map[string]string{}
	for rows.Next() {
		var profileID, status string
		if err := rows.Scan(&profileID, &status); err != nil {
			t.Fatal(err)
		}
		statuses[profileID] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantStatuses := map[string]string{firstProfile.ID: "succeeded", secondProfile.ID: "dead"}
	if !reflect.DeepEqual(statuses, wantStatuses) || requests != 1 {
		t.Fatalf("application delivery statuses=%v requests=%d", statuses, requests)
	}
}

func TestWorkerExpandsUnknownEventsWithoutDeliveryExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO webhook_events (id,event_type,data,created_at)
		VALUES ('evt_unknown','profile.unknown','{}'::jsonb,1000)
	`); err != nil {
		t.Fatal(err)
	}
	worker := Worker{DB: db, Config: testutil.TestConfig(), Now: func() time.Time { return time.UnixMilli(9000) }}
	if err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var expandedAt *int64
	var deliveries int
	if err := db.Pool.QueryRow(ctx, `SELECT expanded_at FROM webhook_events WHERE id='evt_unknown'`).Scan(&expandedAt); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_deliveries WHERE event_id='evt_unknown'`).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	if expandedAt == nil || *expandedAt != 9000 || deliveries != 0 {
		t.Fatalf("unknown event expanded_at=%v deliveries=%d", expandedAt, deliveries)
	}
}

func TestWorkerConstructionCancellationAndSafetyHelpersExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	worker := NewWorker(db, cfg)
	if worker.DB != db || worker.Config.IdentityEncryptionKey != cfg.IdentityEncryptionKey || worker.HTTPClient == nil || worker.Now == nil {
		t.Fatalf("new worker fields mismatch: %#v", worker)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker.Run(ctx)

	if _, err := worker.HTTPClient.Get("http://127.0.0.1:1/private"); err == nil || !strings.Contains(err.Error(), "public address") {
		t.Fatalf("safe client private address error=%v", err)
	}
	if fallback := (Worker{}).httpClient(); fallback == nil {
		t.Fatal("nil worker should construct a safe HTTP client")
	}
	before := time.Now().UnixMilli()
	gotNow := (Worker{}).nowMS()
	after := time.Now().UnixMilli()
	if gotNow < before || gotNow > after {
		t.Fatalf("worker default time=%d outside [%d,%d]", gotNow, before, after)
	}
	maxDelay := retryDelay("delivery-max", 100)
	if maxDelay < 6*time.Hour || maxDelay > 6*time.Hour+72*time.Minute {
		t.Fatalf("capped retry delay=%s", maxDelay)
	}
	longDetail := strings.Repeat("x", 600)
	if got := truncateDetail(longDetail); len(got) != 500 || got != longDetail[:500] {
		t.Fatalf("truncated detail length=%d", len(got))
	}
}

func TestPublicIPRejectsEveryPrivateWebhookAddressClassExactly(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.1.1", "224.0.0.1", "0.0.0.0", "::1", "fc00::1", "fe80::1"} {
		if publicIP(net.ParseIP(raw)) {
			t.Fatalf("private or special address %s accepted", raw)
		}
	}
	for _, raw := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !publicIP(net.ParseIP(raw)) {
			t.Fatalf("public address %s rejected", raw)
		}
	}
}
