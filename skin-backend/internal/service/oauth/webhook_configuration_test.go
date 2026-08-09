package oauth_test

import (
	"context"
	"reflect"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceOAuthAppWebhookConfigurationIsOptionalPermissionBoundAndSecretSafeExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-webhook-config@test.com", "Password123", "OAuthWebhookConfig", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := testutil.TestConfig()
	svc := oauth.Service{DB: db, Redis: redisstore.NewMemoryStore(), Config: cfg}
	enabled := true
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Webhook configured app",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"profile.read.owned", "texture.read.owned"},
		WebhookEndpoints: []oauth.WebhookEndpointInput{{
			URL:        "https://hooks.example/profile",
			EventTypes: []string{"profile.updated", "profile.created"},
			Enabled:    &enabled,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created["redirect_uri"] != "" || created["client_secret"] == "" {
		t.Fatalf("client without OAuth redirect mismatch: %#v", created)
	}
	createdEndpoints := created["webhook_endpoints"].([]map[string]any)
	if len(createdEndpoints) != 1 {
		t.Fatalf("created webhook endpoints=%#v", createdEndpoints)
	}
	firstEndpoint := createdEndpoints[0]
	endpointID := firstEndpoint["id"].(string)
	signingSecret := firstEndpoint["signing_secret"].(string)
	if endpointID == "" || signingSecret == "" || firstEndpoint["status"] != "active" || !reflect.DeepEqual(firstEndpoint["events"], []string{"profile.created", "profile.updated"}) {
		t.Fatalf("created webhook endpoint mismatch: %#v", firstEndpoint)
	}
	stored, err := db.Webhooks.ListEndpointsByClient(ctx, created["client_id"].(string))
	if err != nil || len(stored) != 1 || stored[0].SecretCiphertext == "" || stored[0].SecretCiphertext == signingSecret {
		t.Fatalf("stored webhook endpoint mismatch: %#v err=%v", stored, err)
	}
	box, err := util.NewSecretBox(cfg.IdentityEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := box.Decrypt(stored[0].SecretCiphertext)
	if err != nil || decrypted != signingSecret {
		t.Fatalf("decrypted signing secret=%q err=%v", decrypted, err)
	}
	firstCiphertext := stored[0].SecretCiphertext

	got, err := svc.GetClient(ctx, actor, created["client_id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	gotEndpoints := got["webhook_endpoints"].([]map[string]any)
	if len(gotEndpoints) != 1 {
		t.Fatalf("get webhook endpoints=%#v", gotEndpoints)
	}
	if _, exposed := gotEndpoints[0]["signing_secret"]; exposed {
		t.Fatalf("get app exposed webhook signing secret: %#v", gotEndpoints[0])
	}

	disabled := false
	updated, err := svc.UpdateClient(ctx, actor, created["client_id"].(string), oauth.ClientInput{
		Name:            "Webhook configured app updated",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"profile.read.owned", "texture.read.owned"},
		WebhookEndpoints: []oauth.WebhookEndpointInput{
			{ID: endpointID, URL: "https://hooks.example/profile-v2", EventTypes: []string{"profile.updated"}, Enabled: &disabled},
			{URL: "https://hooks.example/texture", EventTypes: []string{"texture.deleted", "texture.created"}, Enabled: &enabled},
		},
	}, oauth.StatusPending)
	if err != nil {
		t.Fatal(err)
	}
	updatedEndpoints := updated["webhook_endpoints"].([]map[string]any)
	if len(updatedEndpoints) != 2 || updatedEndpoints[0]["id"] != endpointID || updatedEndpoints[0]["enabled"] != false || updatedEndpoints[0]["url"] != "https://hooks.example/profile-v2" {
		t.Fatalf("updated webhook endpoints mismatch: %#v", updatedEndpoints)
	}
	if _, exposed := updatedEndpoints[0]["signing_secret"]; exposed {
		t.Fatalf("existing endpoint secret should not be returned: %#v", updatedEndpoints[0])
	}
	if updatedEndpoints[1]["signing_secret"] == "" {
		t.Fatalf("new endpoint signing secret missing: %#v", updatedEndpoints[1])
	}
	stored, err = db.Webhooks.ListEndpointsByClient(ctx, created["client_id"].(string))
	if err != nil || len(stored) != 2 || stored[0].SecretCiphertext != firstCiphertext || stored[0].Status != "disabled" {
		t.Fatalf("updated stored endpoints=%#v err=%v", stored, err)
	}

	_, err = svc.UpdateClient(ctx, actor, created["client_id"].(string), oauth.ClientInput{
		Name:            "Must not partially update",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"texture.read.owned"},
		WebhookEndpoints: []oauth.WebhookEndpointInput{{
			ID:         endpointID,
			URL:        "https://hooks.example/profile-v3",
			EventTypes: []string{"profile.updated"},
			Enabled:    &enabled,
		}},
	}, oauth.StatusPending)
	assertHTTPError(t, err, 400, "webhook event exceeds client permission limit")
	unchanged, err := svc.GetClient(ctx, actor, created["client_id"].(string))
	if err != nil || unchanged["name"] != "Webhook configured app updated" || !reflect.DeepEqual(unchanged["permissions"], []string{"profile.read.owned", "texture.read.owned"}) {
		t.Fatalf("failed webhook update changed client: %#v err=%v", unchanged, err)
	}

	withoutEndpoint, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "App without webhook or redirect",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil || withoutEndpoint["redirect_uri"] != "" || len(withoutEndpoint["webhook_endpoints"].([]map[string]any)) != 0 {
		t.Fatalf("optional webhook application=%#v err=%v", withoutEndpoint, err)
	}
}

func TestServiceOAuthAppWebhookConfigurationRejectsUnsafeDuplicateAndUnauthorizedInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-webhook-invalid@test.com", "Password123", "OAuthWebhookInvalid", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := oauth.Service{DB: db, Redis: redisstore.NewMemoryStore(), Config: testutil.TestConfig()}
	enabled := true
	cases := []struct {
		name      string
		endpoints []oauth.WebhookEndpointInput
		detail    string
	}{
		{name: "private IP", endpoints: []oauth.WebhookEndpointInput{{URL: "https://127.0.0.1/hook", EventTypes: []string{"profile.updated"}, Enabled: &enabled}}, detail: "invalid webhook url"},
		{name: "localhost", endpoints: []oauth.WebhookEndpointInput{{URL: "https://api.localhost/hook", EventTypes: []string{"profile.updated"}, Enabled: &enabled}}, detail: "invalid webhook url"},
		{name: "plain HTTP", endpoints: []oauth.WebhookEndpointInput{{URL: "http://hooks.example/hook", EventTypes: []string{"profile.updated"}, Enabled: &enabled}}, detail: "invalid webhook url"},
		{name: "missing events", endpoints: []oauth.WebhookEndpointInput{{URL: "https://hooks.example/missing", Enabled: &enabled}}, detail: "webhook events are required"},
		{name: "unknown event", endpoints: []oauth.WebhookEndpointInput{{URL: "https://hooks.example/unknown", EventTypes: []string{"profile.renamed"}, Enabled: &enabled}}, detail: "invalid webhook event"},
		{name: "permission mismatch", endpoints: []oauth.WebhookEndpointInput{{URL: "https://hooks.example/mismatch", EventTypes: []string{"texture.updated"}, Enabled: &enabled}}, detail: "webhook event exceeds client permission limit"},
		{name: "duplicate URL", endpoints: []oauth.WebhookEndpointInput{{URL: "https://hooks.example/duplicate", EventTypes: []string{"profile.created"}, Enabled: &enabled}, {URL: "https://hooks.example/duplicate", EventTypes: []string{"profile.updated"}, Enabled: &enabled}}, detail: "duplicate webhook url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
				Name:             "Invalid webhook " + tc.name,
				ClientType:       oauth.ClientTypePublic,
				PermissionCodes:  []string{"profile.read.owned"},
				WebhookEndpoints: tc.endpoints,
			})
			assertHTTPError(t, err, 400, tc.detail)
		})
	}
	var clientCount, endpointCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM delegated_clients`).Scan(&clientCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_endpoints`).Scan(&endpointCount); err != nil {
		t.Fatal(err)
	}
	if clientCount != 0 || endpointCount != 0 {
		t.Fatalf("invalid webhook requests persisted clients=%d endpoints=%d", clientCount, endpointCount)
	}
}

func TestServiceOAuthAppWebhookConfigurationRestrictsApplicationEventsToConfidentialClientsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "oauth-webhook-app-only@test.com", "Password123", "OAuthWebhookAppOnly", true)
	actor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := oauth.Service{DB: db, Redis: redisstore.NewMemoryStore(), Config: testutil.TestConfig()}
	enabled := true
	input := oauth.ClientInput{
		Name:            "Application webhook",
		PermissionCodes: []string{"permission.read.any"},
		WebhookEndpoints: []oauth.WebhookEndpointInput{{
			URL:        "https://hooks.example/permissions",
			EventTypes: []string{"permission.updated"},
			Enabled:    &enabled,
		}},
	}
	input.ClientType = oauth.ClientTypePublic
	if _, err := svc.CreateClient(ctx, actor, input); err == nil {
		t.Fatal("public client application-only webhook should be rejected")
	} else {
		assertHTTPError(t, err, 400, "webhook event exceeds client permission limit")
	}
	input.ClientType = oauth.ClientTypeConfidential
	created, err := svc.CreateClient(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	endpoints := created["webhook_endpoints"].([]map[string]any)
	if len(endpoints) != 1 || !reflect.DeepEqual(endpoints[0]["events"], []string{"permission.updated"}) {
		t.Fatalf("confidential application webhook endpoints=%#v", endpoints)
	}
	var clients, endpointsCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM delegated_clients`).Scan(&clients); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_endpoints`).Scan(&endpointsCount); err != nil {
		t.Fatal(err)
	}
	if clients != 1 || endpointsCount != 1 {
		t.Fatalf("application webhook rows clients=%d endpoints=%d want=1/1", clients, endpointsCount)
	}
}
