package yggdrasil_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilJoinAndHasJoined(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-session@test.com", "Password123", "YggSession", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_session_profile", "YggSessionRole")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	auth, err := ygg.Authenticate(ctx, user.Email, "Password123", "client_token", false)
	if err != nil {
		t.Fatal(err)
	}
	access := auth["accessToken"].(string)
	if err := ygg.Join(ctx, access, profile.ID, "server_1", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if session, err := redis.GetYggSession(ctx, "server_1"); err != nil || session.AccessToken != access {
		t.Fatalf("join should store session in redis: %#v err=%v", session, err)
	}
	joined, status, err := ygg.HasJoined(ctx, profile.Name, "server_1")
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || joined["id"] != profile.ID || joined["name"] != profile.Name {
		t.Fatalf("HasJoined mismatch: status=%d body=%#v", status, joined)
	}
	if miss, status, err := ygg.HasJoined(ctx, "WrongName", "server_1"); err != nil || status != 204 || miss != nil {
		t.Fatalf("wrong name should miss: status=%d body=%#v err=%v", status, miss, err)
	}
	if err := redis.DeleteYggToken(ctx, access); err != nil {
		t.Fatal(err)
	}
	if miss, status, err := ygg.HasJoined(ctx, profile.Name, "server_1"); err != nil || status != 204 || miss != nil {
		t.Fatalf("missing redis token should make hasJoined miss: status=%d body=%#v err=%v", status, miss, err)
	}
	if _, err := redis.GetYggSession(ctx, "missing_server"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("missing session should be a redis cache miss, got %v", err)
	}
}

func TestYggdrasilJoinRejectsUnboundOrMismatchedProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-join-rules@test.com", "Password123", "YggJoinRules", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_join_rules_profile", "YggJoinRulesProfile")
	other := testutil.CreateProfile(t, db, user.ID, "ygg_join_rules_other", "YggJoinRulesOther")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "unbound_join", ClientToken: "client", UserID: user.ID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ygg.Join(ctx, "unbound_join", profile.ID, "server_unbound", "127.0.0.1"); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("unbound token should not join a profile, got %v", err)
	}
	if _, err := redis.GetYggSession(ctx, "server_unbound"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed unbound join must not create a session, got %v", err)
	}

	profileID := profile.ID
	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "bound_join", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ygg.Join(ctx, "bound_join", other.ID, "server_mismatch", "127.0.0.1"); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("profile mismatch should be rejected, got %v", err)
	}
	if _, err := redis.GetYggSession(ctx, "server_mismatch"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed mismatched join must not create a session, got %v", err)
	}
}

func TestYggdrasilHasJoinedRejectsBannedUserExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-banned@test.com", "Password123", "YggBanned", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_banned_profile", "YggBannedProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}
	profileID := profile.ID

	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "banned_access", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggSession(ctx, model.Session{ServerID: "server_banned", AccessToken: "banned_access", CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := db.Users.Ban(ctx, user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}

	body, status, err := ygg.HasJoined(ctx, profile.Name, "server_banned")
	if err == nil || !strings.Contains(err.Error(), "Account is banned") {
		t.Fatalf("banned user should be rejected with exact error, status=%d body=%#v err=%v", status, body, err)
	}
	if body != nil || status != 0 {
		t.Fatalf("banned hasJoined should not return a success body/status: status=%d body=%#v", status, body)
	}
}
