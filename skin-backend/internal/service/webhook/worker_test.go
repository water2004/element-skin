package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	if err := db.OAuth.CreateClientWithEndpoints(ctx, client, []int64{permissionID}, []model.WebhookEndpoint{endpoint}); err != nil {
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
	if err := db.OAuth.CreateClientWithEndpoints(ctx, client, []int64{profileReadID}, []model.WebhookEndpoint{endpoint}); err != nil {
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
	if err := db.OAuth.CreateClientWithEndpoints(ctx, client, []int64{permissionID}, []model.WebhookEndpoint{endpoint}); err != nil {
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
