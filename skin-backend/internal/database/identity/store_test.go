package identity_test

import (
	"context"
	"reflect"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestCredentialAuthorizationLifecyclePersistsExactStructuredState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "credential-state@test.com", "Password123", "CredentialState", false)
	provider := model.IdentityProvider{
		ID: "credential-state-provider", Name: "Credential State", IssuerURL: "https://identity.example",
		AuthorizationEndpoint: "https://identity.example/authorize", TokenEndpoint: "https://identity.example/token",
		JWKSURI: "https://identity.example/jwks", ClientID: "client", Scopes: []string{"openid"},
		Adapter: "generic_oidc", Enabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	external := model.ExternalIdentity{
		ID: "credential-state-identity", UserID: user.ID, ProviderID: provider.ID,
		Subject: "subject", CreatedAt: 2, UpdatedAt: 2,
	}
	if err := db.Identities.CreateIdentity(ctx, external, model.ExternalIdentityCredential{
		IdentityID: external.ID, RefreshTokenCiphertext: "ciphertext",
		GrantedScopes: []string{"openid"}, UpdatedAt: 2,
	}); err != nil {
		t.Fatal(err)
	}
	views, err := db.Identities.ListIdentityViewsByUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Identity.ID != external.ID ||
		views[0].Provider.ID != provider.ID || views[0].Provider.Name != provider.Name ||
		views[0].Credential.IdentityID != external.ID ||
		views[0].Credential.RefreshTokenCiphertext != "ciphertext" ||
		!reflect.DeepEqual(views[0].Credential.GrantedScopes, []string{"openid"}) ||
		views[0].Credential.AuthorizationStatus != model.ExternalIdentityAuthorizationActive {
		t.Fatalf("identity view mismatch: %#v", views)
	}

	credential, err := db.Identities.GetCredential(ctx, external.ID)
	if err != nil || credential == nil || credential.AuthorizationStatus != model.ExternalIdentityAuthorizationActive || credential.LastRefreshAt != nil || credential.LastRefreshErrorAt != nil {
		t.Fatalf("initial credential=%#v err=%v", credential, err)
	}
	failed, err := db.Identities.MarkCredentialRefreshFailed(ctx, external.ID, 10)
	if err != nil || !failed {
		t.Fatalf("mark transient refresh failure updated=%v err=%v", failed, err)
	}
	credential, err = db.Identities.GetCredential(ctx, external.ID)
	if err != nil || credential == nil || credential.AuthorizationStatus != model.ExternalIdentityAuthorizationActive || credential.LastRefreshErrorAt == nil || *credential.LastRefreshErrorAt != 10 || credential.UpdatedAt != 10 {
		t.Fatalf("transient failure credential=%#v err=%v", credential, err)
	}

	refreshedAt := int64(20)
	credential.RefreshTokenCiphertext = "rotated-ciphertext"
	credential.GrantedScopes = []string{"email", "openid"}
	credential.AuthorizationStatus = model.ExternalIdentityAuthorizationActive
	credential.LastRefreshAt = &refreshedAt
	credential.LastRefreshErrorAt = nil
	credential.UpdatedAt = refreshedAt
	if err := db.Identities.UpdateCredential(ctx, *credential); err != nil {
		t.Fatal(err)
	}
	credential, err = db.Identities.GetCredential(ctx, external.ID)
	if err != nil || credential == nil || credential.RefreshTokenCiphertext != "rotated-ciphertext" || !reflect.DeepEqual(credential.GrantedScopes, []string{"email", "openid"}) || credential.LastRefreshAt == nil || *credential.LastRefreshAt != refreshedAt || credential.LastRefreshErrorAt != nil {
		t.Fatalf("successful refresh credential=%#v err=%v", credential, err)
	}

	rejected, err := db.Identities.MarkCredentialRefreshRejected(ctx, external.ID, 30)
	if err != nil || !rejected {
		t.Fatalf("mark rejected refresh updated=%v err=%v", rejected, err)
	}
	credential, err = db.Identities.GetCredential(ctx, external.ID)
	if err != nil || credential == nil || credential.AuthorizationStatus != model.ExternalIdentityAuthorizationReauthorizationRequired || credential.LastRefreshAt == nil || *credential.LastRefreshAt != 20 || credential.LastRefreshErrorAt == nil || *credential.LastRefreshErrorAt != 30 || credential.UpdatedAt != 30 {
		t.Fatalf("rejected refresh credential=%#v err=%v", credential, err)
	}

	if updated, err := db.Identities.MarkCredentialRefreshRejected(ctx, "missing", 40); err != nil || updated {
		t.Fatalf("missing rejected refresh updated=%v err=%v", updated, err)
	}
	if updated, err := db.Identities.MarkCredentialRefreshFailed(ctx, "missing", 40); err != nil || updated {
		t.Fatalf("missing transient refresh updated=%v err=%v", updated, err)
	}
}
