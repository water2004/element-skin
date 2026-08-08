package webhook_test

import (
	"context"
	"reflect"
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
	events, err := db.Webhooks.ListPendingEvents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "profile.created" || events[0].SubjectUserID != user.ID || events[0].TargetClientID != "" || events[0].CreatedAt <= 0 || events[0].ExpandedAt != nil || !reflect.DeepEqual(events[0].Data, map[string]any{"profile_id": profile.ID, "user_id": user.ID}) {
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
	if err := db.Webhooks.CompleteExpansion(ctx, eventID, []string{endpoint.ID}, 2000); err != nil {
		t.Fatal(err)
	}
	if err := db.Webhooks.CompleteExpansion(ctx, eventID, []string{endpoint.ID}, 2100); err != nil {
		t.Fatal(err)
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
	if len(claimed) != 1 || claimed[0].AttemptCount != 1 || claimed[0].Event.ID != eventID || claimed[0].Endpoint.ID != endpoint.ID || claimed[0].Endpoint.URL != endpoint.URL {
		t.Fatalf("claimed delivery mismatch: %#v", claimed)
	}
	completed, err := db.Webhooks.CompleteDelivery(ctx, claimed[0].ID, 2200, 204)
	if err != nil || !completed {
		t.Fatalf("complete delivery: completed=%v err=%v", completed, err)
	}
	completed, err = db.Webhooks.CompleteDelivery(ctx, claimed[0].ID, 2300, 204)
	if err != nil || completed {
		t.Fatalf("repeat completion: completed=%v err=%v", completed, err)
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
	if status != "succeeded" || attemptCount != 1 || httpStatus != 204 || deliveredAt != 2200 {
		t.Fatalf("completed delivery fields=%q/%d/%d/%d", status, attemptCount, httpStatus, deliveredAt)
	}
	deleted, err := db.Webhooks.DeleteTerminalEventsBefore(ctx, 2200, 10)
	if err != nil || deleted != 0 {
		t.Fatalf("terminal cleanup at exclusive cutoff deleted=%d err=%v", deleted, err)
	}
	deleted, err = db.Webhooks.DeleteTerminalEventsBefore(ctx, 2201, 10)
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
	events, err = db.Webhooks.ListPendingEvents(ctx, 10)
	if err != nil || len(events) != 1 || events[0].Type != "profile.updated" {
		t.Fatalf("updated profile pending events=%#v err=%v", events, err)
	}
	if err := db.Webhooks.CompleteExpansion(ctx, events[0].ID, []string{endpoint.ID}, 3000); err != nil {
		t.Fatal(err)
	}
	claimed, err = db.Webhooks.ClaimDueDeliveries(ctx, 3000, 3500, 10)
	if err != nil || len(claimed) != 1 || claimed[0].AttemptCount != 1 {
		t.Fatalf("retry delivery claim=%#v err=%v", claimed, err)
	}
	retryStatus := 503
	failed, err := db.Webhooks.FailDelivery(ctx, claimed[0].ID, "pending", 4000, 3100, &retryStatus, "temporary failure")
	if err != nil || !failed {
		t.Fatalf("fail delivery: failed=%v err=%v", failed, err)
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
