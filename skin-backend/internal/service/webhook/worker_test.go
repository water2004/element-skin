package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	corewebhook "element-skin/backend/internal/webhook"
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
	if _, err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.DeliverBatch(ctx); err != nil {
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
	if _, err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.OAuth.RevokeGrant(ctx, grant.ID, user.ID, 6100); err != nil || !revoked {
		t.Fatalf("revoke delegated grant: revoked=%v err=%v", revoked, err)
	}
	if _, err := worker.DeliverBatch(ctx); err != nil {
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
	if _, err := worker.RunOnce(ctx); err != nil {
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
	if _, err := worker.DeliverBatch(ctx); err != nil {
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
	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("application permission delivery requests=%d want=1", requests)
	}

	secondProfile := model.Profile{ID: "worker-application-profile-2", UserID: target.ID, Name: "WorkerApplication2", TextureModel: "default"}
	if err := db.Profiles.Create(ctx, secondProfile); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.DispatchBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE webhook_endpoints SET status='disabled' WHERE id=$1`, endpoint.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.DeliverBatch(ctx); err != nil {
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

func TestWorkerDeliversApplicationOnlyEventToConfidentialClientAndRejectsPublicClientExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "webhook-worker-app-only-owner@test.com", "Password123", "WebhookWorkerAppOnlyOwner", false)
	target := testutil.CreateUser(t, db, "webhook-worker-app-only-target@test.com", "Password123", "WebhookWorkerAppOnlyTarget", false)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg := testutil.TestConfig()
	box, err := util.NewSecretBox(cfg.IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt("application-only-secret")
	if err != nil {
		t.Fatal(err)
	}
	definition := permission.MustDefinitionByCode("permission.read.any")
	clients := []model.OAuthClient{
		{ID: "worker-app-only-confidential", OwnerUserID: owner.ID, Name: "Confidential app-only", ClientType: "confidential", SecretHash: "secret", Status: "active", CreatedAt: 1000, UpdatedAt: 1000},
		{ID: "worker-app-only-public", OwnerUserID: owner.ID, Name: "Public app-only", ClientType: "public", Status: "active", CreatedAt: 1001, UpdatedAt: 1001},
	}
	for _, client := range clients {
		endpoint := model.WebhookEndpoint{
			ID: "wh_" + client.ID, ClientID: client.ID, URL: server.URL, SecretCiphertext: ciphertext,
			Status: "active", EventTypes: []string{"permission.updated"}, CreatedAt: client.CreatedAt, UpdatedAt: client.UpdatedAt,
		}
		if err := db.OAuth.CreateClient(ctx, client, []int64{int64(definition.ID)}, []model.WebhookEndpoint{endpoint}); err != nil {
			t.Fatal(err)
		}
		if err := db.Permissions.SetPermissionOverrideForSubject(ctx, permissiondb.SubjectIDForClient(client.ID), definition, "allow", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Permissions.GrantRole(ctx, target.ID, permission.RoleAdmin, ""); err != nil {
		t.Fatal(err)
	}
	worker := Worker{DB: db, Config: cfg, HTTPClient: server.Client(), Now: func() time.Time { return time.UnixMilli(8500) }}
	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT endpoint.client_id,delivery.status,delivery.last_error
		FROM webhook_deliveries AS delivery
		JOIN webhook_endpoints AS endpoint ON endpoint.id=delivery.endpoint_id
		ORDER BY endpoint.client_id
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	statuses := map[string][2]string{}
	for rows.Next() {
		var clientID, status, detail string
		if err := rows.Scan(&clientID, &status, &detail); err != nil {
			t.Fatal(err)
		}
		statuses[clientID] = [2]string{status, detail}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"worker-app-only-confidential": {"succeeded", ""},
	}
	if requests != 1 || !reflect.DeepEqual(statuses, want) {
		t.Fatalf("application-only requests=%d statuses=%#v want=1/%#v", requests, statuses, want)
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
	if _, err := worker.DispatchBatch(ctx); err != nil {
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

func TestWorkerDrainsActiveExpansionQueueWithoutWaitingForPollIntervalExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	if _, err := db.Pool.Exec(t.Context(), `
		INSERT INTO webhook_events (id,event_type,data,created_at)
		SELECT 'evt_drain_' || item::TEXT, 'profile.unknown', '{}'::JSONB, item
		FROM generate_series(1,201) AS item
	`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	worker := Worker{
		DB:             db,
		Config:         testutil.TestConfig(),
		Now:            func() time.Time { return time.UnixMilli(10000) },
		PollInterval:   time.Hour,
		ActiveInterval: time.Millisecond,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var expanded, deliveries int
			if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_events WHERE expanded_at=10000`).Scan(&expanded); err != nil {
				cancel()
				<-done
				t.Fatal(err)
			}
			if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM webhook_deliveries`).Scan(&deliveries); err != nil {
				cancel()
				<-done
				t.Fatal(err)
			}
			if expanded == 201 {
				cancel()
				<-done
				if deliveries != 0 {
					t.Fatalf("unknown event deliveries=%d want=0", deliveries)
				}
				return
			}
			if expanded > 201 || deliveries != 0 {
				cancel()
				<-done
				t.Fatalf("drain state expanded=%d deliveries=%d", expanded, deliveries)
			}
		case <-ctx.Done():
			<-done
			t.Fatalf("worker waited for poll interval with active queue: expanded queue did not drain: %v", ctx.Err())
		}
	}
}

func TestBatchAuthorizationCacheCoalescesOnlyMatchingConcurrentChecksExactly(t *testing.T) {
	cache := newBatchAuthorizationCache()
	key := authorizationCacheKey{
		EventType:        "profile.updated",
		SubjectUserID:    "user-1",
		EndpointClientID: "client-1",
	}
	const callers = 20
	var calls atomic.Int32
	release := make(chan struct{})
	results := make(chan struct {
		allowed bool
		err     error
	}, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			allowed, err := cache.resolve(t.Context(), key, func() (bool, error) {
				calls.Add(1)
				<-release
				return true, nil
			})
			results <- struct {
				allowed bool
				err     error
			}{allowed: allowed, err: err}
		}()
	}
	ready.Wait()
	close(release)
	for range callers {
		result := <-results
		if !result.allowed || result.err != nil {
			t.Fatalf("coalesced authorization result allowed=%v err=%v", result.allowed, result.err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("matching authorization loads=%d want=1", got)
	}

	differentKey := key
	differentKey.SubjectUserID = "user-2"
	allowed, err := cache.resolve(t.Context(), differentKey, func() (bool, error) {
		calls.Add(1)
		return false, nil
	})
	if allowed || err != nil || calls.Load() != 2 {
		t.Fatalf("different authorization result allowed=%v err=%v loads=%d want false,nil,2", allowed, err, calls.Load())
	}

	wantErr := errors.New("authorization unavailable")
	errorKey := key
	errorKey.SubjectUserID = "user-3"
	var errorCalls int
	for index := range 2 {
		allowed, err = cache.resolve(t.Context(), errorKey, func() (bool, error) {
			errorCalls++
			return false, wantErr
		})
		if allowed || !errors.Is(err, wantErr) {
			t.Fatalf("cached error result %d allowed=%v err=%v", index, allowed, err)
		}
	}
	if errorCalls != 1 {
		t.Fatalf("matching failed authorization loads=%d want=1", errorCalls)
	}
}

func TestWorkerConstructionCancellationAndSafetyHelpersExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	worker := NewWorker(db, cfg)
	if worker.DB != db || worker.Config.IdentityEncryptionKey != cfg.IdentityEncryptionKey || worker.HTTPClient == nil || worker.Now == nil {
		t.Fatalf("new worker fields mismatch: %#v", worker)
	}
	if worker.ActiveWorkInterval() != 3*time.Second {
		t.Fatalf("default active work interval=%s want=3s", worker.ActiveWorkInterval())
	}
	if interval := (Worker{}).ActiveWorkInterval(); interval != defaultActiveInterval {
		t.Fatalf("zero-value active work interval=%s want=%s", interval, defaultActiveInterval)
	}
	worker.ActiveInterval = 75 * time.Millisecond
	if worker.ActiveWorkInterval() != 75*time.Millisecond {
		t.Fatalf("custom active work interval=%s want=75ms", worker.ActiveWorkInterval())
	}
	if worker.pollInterval() != 500*time.Millisecond {
		t.Fatalf("default poll interval=%s want=500ms", worker.pollInterval())
	}
	worker.PollInterval = 90 * time.Millisecond
	if worker.pollInterval() != 90*time.Millisecond {
		t.Fatalf("custom poll interval=%s want=90ms", worker.pollInterval())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker.Run(ctx)
	if worker.waitAfterWork(ctx, make(chan time.Time)) {
		t.Fatal("canceled worker wait should stop")
	}
	timerWorker := worker
	timerWorker.ActiveInterval = time.Millisecond
	if !timerWorker.waitAfterWork(t.Context(), make(chan time.Time)) {
		t.Fatal("active worker wait should resume after timer")
	}

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
	if got, want := retryDelay("delivery-zero", 0), retryDelay("delivery-zero", 1); got != want {
		t.Fatalf("zero-attempt retry delay=%s want first-attempt %s", got, want)
	}
	longDetail := strings.Repeat("x", 600)
	if got := truncateDetail(longDetail); len(got) != 500 || got != longDetail[:500] {
		t.Fatalf("truncated detail length=%d", len(got))
	}

	now := time.UnixMilli(20 * int64(24*time.Hour/time.Millisecond))
	oldExpandedAt := now.Add(-8 * 24 * time.Hour).UnixMilli()
	recentExpandedAt := now.Add(-6 * 24 * time.Hour).UnixMilli()
	if _, err := db.Pool.Exec(t.Context(), `
		INSERT INTO webhook_events (id,event_type,data,created_at,expanded_at)
		VALUES
			('evt_cleanup_old','profile.unknown','{}'::jsonb,$1,$1),
			('evt_cleanup_recent','profile.unknown','{}'::jsonb,$2,$2)
	`, oldExpandedAt, recentExpandedAt); err != nil {
		t.Fatal(err)
	}
	cleanupWorker := Worker{DB: db, Config: cfg, Now: func() time.Time { return now }}
	cleanupWorker.ActiveInterval = time.Millisecond
	cleanup := make(chan time.Time, 1)
	cleanup <- now
	if !cleanupWorker.waitAfterWork(t.Context(), cleanup) {
		t.Fatal("cleanup wait should resume after active interval")
	}
	var remaining []string
	rows, err := db.Pool.Query(t.Context(), `SELECT id FROM webhook_events ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		remaining = append(remaining, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(remaining, []string{"evt_cleanup_recent"}) {
		t.Fatalf("cleanup remaining events=%v", remaining)
	}
	safeClient := newSafeHTTPClient()
	transport, ok := safeClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("safe transport type=%T", safeClient.Transport)
	}
	if _, err := transport.DialContext(t.Context(), "tcp", "missing-port"); err == nil ||
		!strings.Contains(err.Error(), "missing port") {
		t.Fatalf("safe transport malformed address error=%v", err)
	}
	if err := safeClient.CheckRedirect(httptest.NewRequest(http.MethodGet, "https://example.com", nil), nil); err == nil || err.Error() != "webhook redirects are not followed" {
		t.Fatalf("safe redirect error=%v", err)
	}

	targeted, ok := corewebhook.DefinitionByType("oauth_grant.revoked")
	if !ok {
		t.Fatal("oauth_grant.revoked definition is missing")
	}
	allowed, err := worker.endpointAuthorized(t.Context(), model.WebhookEvent{TargetClientID: "target-client"}, model.WebhookEndpoint{ClientID: "other-client"}, targeted)
	if err != nil || allowed {
		t.Fatalf("mismatched targeted client authorization allowed=%v err=%v", allowed, err)
	}
	allowed, err = worker.endpointAuthorized(t.Context(), model.WebhookEvent{}, model.WebhookEndpoint{}, corewebhook.Definition{})
	if err != nil || allowed {
		t.Fatalf("event without permission authorization allowed=%v err=%v", allowed, err)
	}

	cancelCache := newBatchAuthorizationCache()
	cancelKey := authorizationCacheKey{EventType: "profile.updated", SubjectUserID: "cancel-user", EndpointClientID: "cancel-client"}
	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		_, resolveErr := cancelCache.resolve(context.Background(), cancelKey, func() (bool, error) {
			close(loadStarted)
			<-releaseLoad
			return true, nil
		})
		firstResult <- resolveErr
	}()
	<-loadStarted
	canceledContext, cancelResolve := context.WithCancel(context.Background())
	cancelResolve()
	secondLoadCalls := 0
	allowed, err = cancelCache.resolve(canceledContext, cancelKey, func() (bool, error) {
		secondLoadCalls++
		return false, nil
	})
	if !errors.Is(err, context.Canceled) || allowed || secondLoadCalls != 0 {
		t.Fatalf("canceled cached authorization allowed=%v err=%v second_loads=%d", allowed, err, secondLoadCalls)
	}
	close(releaseLoad)
	if err := <-firstResult; err != nil {
		t.Fatalf("first cached authorization load error=%v", err)
	}

	db.Close()
	cleanupWorker.cleanup(t.Context())
}
