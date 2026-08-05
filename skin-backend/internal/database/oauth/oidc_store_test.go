package oauth_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestOIDCPairwiseSubjectsAreStablePerClientAndIsolatedAcrossClients(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "pairwise-subject@example.com", "Password123", "PairwiseSubject", false)
	for _, clientID := range []string{"pairwise-client-a", "pairwise-client-b"} {
		if err := db.OAuth.CreateClient(ctx, model.OAuthClient{
			ID: clientID, OwnerUserID: user.ID, Name: clientID,
			RedirectURI: "https://" + clientID + ".example/callback", ClientType: "public",
			Status: "active", CreatedAt: 1, UpdatedAt: 1,
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := db.OAuth.GetPairwiseSubject(ctx, "pairwise-client-a", user.ID); err != nil || got != "" {
		t.Fatalf("missing pairwise subject mismatch: subject=%q err=%v", got, err)
	}
	first, err := db.OAuth.CreatePairwiseSubject(ctx, "pairwise-client-a", user.ID, "subject-a", 10)
	if err != nil || first != "subject-a" {
		t.Fatalf("create first subject=%q err=%v", first, err)
	}
	stable, err := db.OAuth.CreatePairwiseSubject(ctx, "pairwise-client-a", user.ID, "replacement-must-not-win", 20)
	if err != nil || stable != "subject-a" {
		t.Fatalf("same client/user subject must remain stable: subject=%q err=%v", stable, err)
	}
	second, err := db.OAuth.CreatePairwiseSubject(ctx, "pairwise-client-b", user.ID, "subject-b", 30)
	if err != nil || second != "subject-b" || second == first {
		t.Fatalf("different clients must use isolated subjects: first=%q second=%q err=%v", first, second, err)
	}
	stored, err := db.OAuth.GetPairwiseSubject(ctx, "pairwise-client-a", user.ID)
	if err != nil || stored != "subject-a" {
		t.Fatalf("stored pairwise subject mismatch: subject=%q err=%v", stored, err)
	}
}
