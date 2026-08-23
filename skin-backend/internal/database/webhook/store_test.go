package webhook_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestWebhookOutboxTriggerExpansionClaimAndCompletionLifecycleExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "webhook-store@test.com", "Password123", "WebhookStore", false)
	profileReadID := int64(permission.MustDefinitionByCode("profile.read.owned").ID)
	client := model.OAuthClient{
		ID:          "webhook-store-client",
		OwnerUserID: user.ID,
		Name:        "Webhook store client",
		RedirectURI: "",
		ClientType:  "confidential",
		SecretHash:  "client-secret-hash",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	endpoint := model.WebhookEndpoint{
		ID:               "wh_store_endpoint",
		ClientID:         client.ID,
		URL:              "https://hooks.example/events",
		SecretCiphertext: "ciphertext",
		Status:           "active",
		EventTypes:       []string{"profile.created", "profile.updated"},
		CreatedAt:        1000,
		UpdatedAt:        1000,
	}
	if err := db.OAuth.CreateClient(ctx, client, []int64{profileReadID}, []model.WebhookEndpoint{endpoint}); err != nil {
		t.Fatal(err)
	}
	storedEndpoints, err := db.Webhooks.ListEndpointsByClient(ctx, client.ID)
	if err != nil || len(storedEndpoints) != 1 || storedEndpoints[0].ID != endpoint.ID ||
		!reflect.DeepEqual(storedEndpoints[0].EventTypes, []string{"profile.created", "profile.updated"}) {
		t.Fatalf("stored webhook endpoints=%#v err=%v", storedEndpoints, err)
	}
	for _, tc := range []struct {
		name           string
		targetClientID string
		wantCount      int
	}{
		{name: "broadcast candidates", targetClientID: "", wantCount: 1},
		{name: "matching targeted client", targetClientID: client.ID, wantCount: 1},
		{name: "other targeted client", targetClientID: "other-client", wantCount: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoints, err := db.Webhooks.ListSubscribedEndpoints(ctx, "profile.created", tc.targetClientID)
			if err != nil || len(endpoints) != tc.wantCount {
				t.Fatalf("subscribed endpoints=%#v want_count=%d err=%v", endpoints, tc.wantCount, err)
			}
			if tc.wantCount == 1 && endpoints[0].ID != endpoint.ID {
				t.Fatalf("subscribed endpoint id=%q want=%q", endpoints[0].ID, endpoint.ID)
			}
		})
	}
	profile := model.Profile{ID: "webhook-store-profile", UserID: user.ID, Name: "WebhookStore", TextureModel: "default"}
	if err := db.Profiles.Create(ctx, profile); err != nil {
		t.Fatal(err)
	}
	events, err := db.Webhooks.ClaimPendingEvents(ctx, 1000, 2000, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "profile.created" || events[0].SubjectUserID != user.ID || events[0].TargetClientID != "" || events[0].CreatedAt <= 0 || events[0].ExpandedAt != nil || events[0].ExpansionLeaseUntil == nil || *events[0].ExpansionLeaseUntil != 2000 || events[0].ExpansionLeaseToken == "" || !reflect.DeepEqual(events[0].Data, map[string]any{"profile_id": profile.ID, "user_id": user.ID}) {
		t.Fatalf("created profile outbox event mismatch: %#v", events)
	}
	eventID := events[0].ID
	if eventID == "" {
		t.Fatal("outbox event id should not be empty")
	}
	storedEvent, err := db.Webhooks.GetEvent(ctx, eventID)
	if err != nil || storedEvent == nil || !reflect.DeepEqual(*storedEvent, events[0]) {
		t.Fatalf("stored webhook event=%#v want=%#v err=%v", storedEvent, events[0], err)
	}
	missingEvent, err := db.Webhooks.GetEvent(ctx, "evt_missing")
	if err != nil || missingEvent != nil {
		t.Fatalf("missing webhook event=%#v err=%v", missingEvent, err)
	}
	claimedAgain, err := db.Webhooks.ClaimPendingEvents(ctx, 1999, 3000, 10)
	if err != nil || len(claimedAgain) != 0 {
		t.Fatalf("event claimed before expansion lease expiry=%#v err=%v", claimedAgain, err)
	}
	claimedAgain, err = db.Webhooks.ClaimPendingEvents(ctx, 2000, 3000, 10)
	if err != nil || len(claimedAgain) != 1 || claimedAgain[0].ID != eventID || claimedAgain[0].ExpansionLeaseToken == events[0].ExpansionLeaseToken {
		t.Fatalf("event reclaimed at expansion lease expiry=%#v err=%v", claimedAgain, err)
	}
	completedEvents, insertedDeliveries, err := db.Webhooks.CompleteExpansions(ctx, []model.WebhookExpansion{{
		EventID:     eventID,
		LeaseToken:  events[0].ExpansionLeaseToken,
		EndpointIDs: []string{endpoint.ID},
	}}, 2000)
	if err != nil || completedEvents != 0 || insertedDeliveries != 0 {
		t.Fatalf("stale expansion completed=%d inserted=%d err=%v", completedEvents, insertedDeliveries, err)
	}
	completedEvents, insertedDeliveries, err = db.Webhooks.CompleteExpansions(ctx, []model.WebhookExpansion{{
		EventID:     eventID,
		LeaseToken:  claimedAgain[0].ExpansionLeaseToken,
		EndpointIDs: []string{endpoint.ID, endpoint.ID},
	}}, 2000)
	if err != nil || completedEvents != 1 || insertedDeliveries != 1 {
		t.Fatalf("current expansion completed=%d inserted=%d err=%v", completedEvents, insertedDeliveries, err)
	}
	completedEvents, insertedDeliveries, err = db.Webhooks.CompleteExpansions(ctx, []model.WebhookExpansion{{
		EventID:     eventID,
		LeaseToken:  claimedAgain[0].ExpansionLeaseToken,
		EndpointIDs: []string{endpoint.ID},
	}}, 2100)
	if err != nil || completedEvents != 0 || insertedDeliveries != 0 {
		t.Fatalf("repeat expansion completed=%d inserted=%d err=%v", completedEvents, insertedDeliveries, err)
	}
	var deliveryCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_deliveries WHERE event_id=$1`, eventID).Scan(&deliveryCount); err != nil {
		t.Fatal(err)
	}
	if deliveryCount != 1 {
		t.Fatalf("idempotent expansion delivery count=%d want 1", deliveryCount)
	}
	claimed, err := db.Webhooks.ClaimDueDeliveries(ctx, 2000, 2500, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].AttemptCount != 1 || claimed[0].LeaseToken == "" || claimed[0].Event.ID != eventID || claimed[0].Endpoint.ID != endpoint.ID || claimed[0].Endpoint.URL != endpoint.URL {
		t.Fatalf("claimed delivery mismatch: %#v", claimed)
	}
	claimedDeliveryAgain, err := db.Webhooks.ClaimDueDeliveries(ctx, 2499, 3000, 10)
	if err != nil || len(claimedDeliveryAgain) != 0 {
		t.Fatalf("delivery claimed before lease expiry=%#v err=%v", claimedDeliveryAgain, err)
	}
	claimedDeliveryAgain, err = db.Webhooks.ClaimDueDeliveries(ctx, 2500, 3000, 10)
	if err != nil || len(claimedDeliveryAgain) != 1 || claimedDeliveryAgain[0].AttemptCount != 2 || claimedDeliveryAgain[0].LeaseToken == claimed[0].LeaseToken {
		t.Fatalf("delivery reclaimed at lease expiry=%#v err=%v", claimedDeliveryAgain, err)
	}
	httpNoContent := http.StatusNoContent
	deliveredAtValue := int64(2600)
	completed, err := db.Webhooks.CompleteDeliveryOutcomes(ctx, []model.WebhookDeliveryOutcome{{
		DeliveryID:  claimed[0].ID,
		LeaseToken:  claimed[0].LeaseToken,
		Status:      "succeeded",
		UpdatedAt:   deliveredAtValue,
		HTTPStatus:  &httpNoContent,
		DeliveredAt: &deliveredAtValue,
	}})
	if err != nil || completed != 0 {
		t.Fatalf("stale delivery outcome completed=%d err=%v", completed, err)
	}
	completed, err = db.Webhooks.CompleteDeliveryOutcomes(ctx, []model.WebhookDeliveryOutcome{{
		DeliveryID:  claimedDeliveryAgain[0].ID,
		LeaseToken:  claimedDeliveryAgain[0].LeaseToken,
		Status:      "succeeded",
		UpdatedAt:   deliveredAtValue,
		HTTPStatus:  &httpNoContent,
		DeliveredAt: &deliveredAtValue,
	}})
	if err != nil || completed != 1 {
		t.Fatalf("current delivery outcome completed=%d err=%v", completed, err)
	}
	completed, err = db.Webhooks.CompleteDeliveryOutcomes(ctx, []model.WebhookDeliveryOutcome{{
		DeliveryID: claimedDeliveryAgain[0].ID,
		LeaseToken: claimedDeliveryAgain[0].LeaseToken,
		Status:     "succeeded",
		UpdatedAt:  2700,
	}})
	if err != nil || completed != 0 {
		t.Fatalf("repeat delivery outcome completed=%d err=%v", completed, err)
	}
	var status string
	var attemptCount, httpStatus int
	var deliveredAt int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT status, attempt_count, last_http_status, delivered_at
		FROM webhook_deliveries WHERE id=$1
	`, claimed[0].ID).Scan(&status, &attemptCount, &httpStatus, &deliveredAt); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || attemptCount != 2 || httpStatus != 204 || deliveredAt != 2600 {
		t.Fatalf("completed delivery fields=%q/%d/%d/%d", status, attemptCount, httpStatus, deliveredAt)
	}
	deleted, err := db.Webhooks.DeleteTerminalEventsBefore(ctx, 2600, 10)
	if err != nil || deleted != 0 {
		t.Fatalf("terminal cleanup at exclusive cutoff deleted=%d err=%v", deleted, err)
	}
	deleted, err = db.Webhooks.DeleteTerminalEventsBefore(ctx, 2601, 10)
	if err != nil || deleted != 1 {
		t.Fatalf("terminal cleanup deleted=%d want=1 err=%v", deleted, err)
	}
	var remainingEvents, remainingDeliveries int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_events`).Scan(&remainingEvents); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_deliveries`).Scan(&remainingDeliveries); err != nil {
		t.Fatal(err)
	}
	if remainingEvents != 0 || remainingDeliveries != 0 {
		t.Fatalf("terminal cleanup left events=%d deliveries=%d", remainingEvents, remainingDeliveries)
	}

	if _, err := db.Profiles.UpdateName(ctx, profile.ID, "WebhookStore2"); err != nil {
		t.Fatal(err)
	}
	events, err = db.Webhooks.ClaimPendingEvents(ctx, 3000, 4000, 10)
	if err != nil || len(events) != 1 || events[0].Type != "profile.updated" {
		t.Fatalf("updated profile pending events=%#v err=%v", events, err)
	}
	completedEvents, insertedDeliveries, err = db.Webhooks.CompleteExpansions(ctx, []model.WebhookExpansion{{
		EventID:     events[0].ID,
		LeaseToken:  events[0].ExpansionLeaseToken,
		EndpointIDs: []string{endpoint.ID},
	}}, 3000)
	if err != nil || completedEvents != 1 || insertedDeliveries != 1 {
		t.Fatalf("updated expansion completed=%d inserted=%d err=%v", completedEvents, insertedDeliveries, err)
	}
	claimed, err = db.Webhooks.ClaimDueDeliveries(ctx, 3000, 3500, 10)
	if err != nil || len(claimed) != 1 || claimed[0].AttemptCount != 1 {
		t.Fatalf("retry delivery claim=%#v err=%v", claimed, err)
	}
	retryStatus := 503
	failed, err := db.Webhooks.CompleteDeliveryOutcomes(ctx, []model.WebhookDeliveryOutcome{{
		DeliveryID:    claimed[0].ID,
		LeaseToken:    claimed[0].LeaseToken,
		Status:        "pending",
		NextAttemptAt: 4000,
		UpdatedAt:     3100,
		HTTPStatus:    &retryStatus,
		Detail:        "temporary failure",
	}})
	if err != nil || failed != 1 {
		t.Fatalf("fail delivery outcomes=%d err=%v", failed, err)
	}
	claimed, err = db.Webhooks.ClaimDueDeliveries(ctx, 3999, 4500, 10)
	if err != nil || len(claimed) != 0 {
		t.Fatalf("delivery claimed before retry time=%#v err=%v", claimed, err)
	}
	claimed, err = db.Webhooks.ClaimDueDeliveries(ctx, 4000, 4500, 10)
	if err != nil || len(claimed) != 1 || claimed[0].AttemptCount != 2 {
		t.Fatalf("delivery retry claim=%#v err=%v", claimed, err)
	}
}

func TestWebhookEventClaimsAreDisjointAcrossConcurrentWorkersExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO webhook_events (id,event_type,data,created_at)
		SELECT 'evt_claim_' || item::TEXT, 'profile.unknown', '{}'::JSONB, item
		FROM generate_series(1,60) AS item
	`); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	claims := make(chan []model.WebhookEvent, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for worker := range 2 {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			events, err := db.Webhooks.ClaimPendingEvents(ctx, 1000, 2000+int64(worker), 30)
			claims <- events
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(claims)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	seen := make(map[string]string, 60)
	claimSets := 0
	for events := range claims {
		claimSets++
		if len(events) != 30 {
			t.Fatalf("concurrent claim size=%d want=30", len(events))
		}
		for _, event := range events {
			if previousToken, exists := seen[event.ID]; exists {
				t.Fatalf("event %q claimed twice with tokens %q and %q", event.ID, previousToken, event.ExpansionLeaseToken)
			}
			if event.ExpansionLeaseToken == "" || event.ExpansionLeaseUntil == nil {
				t.Fatalf("event %q missing expansion lease: %#v", event.ID, event)
			}
			seen[event.ID] = event.ExpansionLeaseToken
		}
	}
	if claimSets != 2 || len(seen) != 60 {
		t.Fatalf("concurrent claims sets=%d unique_events=%d want=2/60", claimSets, len(seen))
	}
	for item := 1; item <= 60; item++ {
		if _, exists := seen[fmt.Sprintf("evt_claim_%d", item)]; !exists {
			t.Fatalf("event evt_claim_%d was not claimed", item)
		}
	}
	remaining, err := db.Webhooks.ClaimPendingEvents(ctx, 1999, 3000, 60)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("leased events reclaimed early=%#v err=%v", remaining, err)
	}
}

func TestWebhookTriggerSkipsEventsWithoutActiveSubscribersExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "webhook-no-subscriber@test.com", "Password123", "WebhookNoSubscriber", false)
	if err := db.Profiles.Create(ctx, model.Profile{ID: "webhook-no-subscriber-profile", UserID: user.ID, Name: "NoSubscriber", TextureModel: "default"}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("webhook event count without active subscription=%d want 0", count)
	}
}
