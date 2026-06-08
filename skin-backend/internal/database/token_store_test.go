package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestTokenStoreTokensSessionsAndRefreshExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "token-store-direct@test.com", "Password123", "TokenStoreDirect", false)
	profile := testutil.CreateProfile(t, db, user.ID, "token_store_profile", "TokenStoreProfile")

	if err := db.AddToken(ctx, model.Token{AccessToken: "old_access_direct", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddToken(ctx, model.Token{AccessToken: "new_access_direct", ClientToken: "client", UserID: user.ID, CreatedAt: 20}); err != nil {
		t.Fatal(err)
	}
	if err := db.CleanupTokens(ctx, user.ID, 15, 1); err != nil {
		t.Fatal(err)
	}
	if old, err := db.GetToken(ctx, "old_access_direct"); err != nil || old != nil {
		t.Fatalf("old token should be removed: token=%#v err=%v", old, err)
	}
	if newer, err := db.GetToken(ctx, "new_access_direct"); err != nil || newer == nil || newer.ProfileID != nil {
		t.Fatalf("new token mismatch: token=%#v err=%v", newer, err)
	}

	if err := db.ReplaceSession(ctx, model.Session{ServerID: "server_direct", AccessToken: "new_access_direct", CreatedAt: 200}); err != nil {
		t.Fatal(err)
	}
	session, err := db.GetSession(ctx, "server_direct")
	if err != nil || session == nil || session.AccessToken != "new_access_direct" || session.CreatedAt != 200 {
		t.Fatalf("session mismatch: session=%#v err=%v", session, err)
	}

	if err := db.AddRefreshToken(ctx, "refresh_direct", user.ID, database.NowMS()+60_000, 100); err != nil {
		t.Fatal(err)
	}
	consumed, err := db.ConsumeRefreshToken(ctx, "refresh_direct")
	if err != nil || consumed["token_hash"] != "refresh_direct" || consumed["user_id"] != user.ID {
		t.Fatalf("refresh consume mismatch: refresh=%#v err=%v", consumed, err)
	}
	if again, err := db.ConsumeRefreshToken(ctx, "refresh_direct"); err != nil || again != nil {
		t.Fatalf("refresh token should be single-use: %#v err=%v", again, err)
	}
}
