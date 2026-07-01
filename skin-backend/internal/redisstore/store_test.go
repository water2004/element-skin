package redisstore

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestRedisStoreHelpersPreserveStableKeysAndModels(t *testing.T) {
	defaultStore := New(nil, "")
	if defaultStore.key("settings", "site_name") != "elementskin:settings:site_name" {
		t.Fatalf("default prefix key mismatch: %q", defaultStore.key("settings", "site_name"))
	}
	if err := defaultStore.Close(); err != nil {
		t.Fatalf("nil client close should be safe: %v", err)
	}

	customStore := New(nil, "tenant")
	if customStore.prefix != "tenant:" || customStore.key("public", "settings") != "tenant:public:settings" {
		t.Fatalf("custom prefix should be normalized exactly: prefix=%q key=%q", customStore.prefix, customStore.key("public", "settings"))
	}
	alreadyNormalized := New(nil, "tenant:")
	if alreadyNormalized.prefix != "tenant:" {
		t.Fatalf("already-normalized prefix should not gain another colon: %q", alreadyNormalized.prefix)
	}

	bannedUntil := time.Now().Add(time.Minute).UnixMilli()
	user := AuthUserFromModel(model.User{ID: "u1", BannedUntil: &bannedUntil})
	if user.ID != "u1" || user.BannedUntil == nil || !user.Banned(time.Now()) {
		t.Fatalf("auth user model conversion mismatch: %#v", user)
	}
	expiredBan := time.Now().Add(-time.Minute).UnixMilli()
	user.BannedUntil = &expiredBan
	if user.Banned(time.Now()) {
		t.Fatalf("expired ban should not be active: %#v", user)
	}

	profileID := "p1"
	token := model.Token{AccessToken: "access", ClientToken: "client", UserID: "u1", ProfileID: &profileID, CreatedAt: 123}
	roundTripToken := yggTokenFromModel(token).model()
	if roundTripToken.AccessToken != token.AccessToken || roundTripToken.ClientToken != token.ClientToken ||
		roundTripToken.UserID != token.UserID || roundTripToken.ProfileID == nil || *roundTripToken.ProfileID != profileID ||
		roundTripToken.CreatedAt != token.CreatedAt {
		t.Fatalf("ygg token conversion mismatch: %#v", roundTripToken)
	}

	ip := "203.0.113.10"
	session := model.Session{ServerID: "server", AccessToken: "access", IP: &ip, CreatedAt: 456}
	roundTripSession := yggSessionFromModel(session).model()
	if roundTripSession.ServerID != session.ServerID || roundTripSession.AccessToken != session.AccessToken ||
		roundTripSession.IP == nil || *roundTripSession.IP != ip || roundTripSession.CreatedAt != session.CreatedAt {
		t.Fatalf("ygg session conversion mismatch: %#v", roundTripSession)
	}
}

func TestMemoryStoreYggDeleteAndTrimExactLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	profileID := "p1"
	tokens := []model.Token{
		{AccessToken: "old", ClientToken: "client", UserID: "u1", ProfileID: &profileID, CreatedAt: 1},
		{AccessToken: "new", ClientToken: "client", UserID: "u1", ProfileID: &profileID, CreatedAt: 2},
	}
	for _, token := range tokens {
		if err := store.SetYggToken(ctx, token, time.Minute); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.DeleteYggToken(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, "old"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted token should miss, got %v", err)
	}
	remaining, err := store.GetYggToken(ctx, "new")
	if err != nil || remaining.AccessToken != "new" || remaining.UserID != "u1" {
		t.Fatalf("other token should remain after single delete: %#v err=%v", remaining, err)
	}
	if err := store.DeleteYggToken(ctx, "missing"); err != nil {
		t.Fatalf("deleting a missing token should be idempotent: %v", err)
	}

	replaced, err := store.ReplaceYggToken(ctx, "missing", model.Token{AccessToken: "replacement", UserID: "u1", CreatedAt: 3}, time.Minute)
	if err != nil || replaced {
		t.Fatalf("replacing a missing token should return false without error: replaced=%v err=%v", replaced, err)
	}
	if err := store.TrimYggTokensByUser(ctx, "u1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, "new"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("trim keep=0 should delete all user tokens, got %v", err)
	}
	if err := store.TrimYggTokensByUser(ctx, "missing-user", 2); err != nil {
		t.Fatalf("trimming a missing user should be a no-op: %v", err)
	}
}

func TestMemoryStoreYggIndexMutationsPreserveOriginalTTL(t *testing.T) {
	ctx := context.Background()
	start := time.UnixMilli(10_000)

	deleteStore := NewMemoryStore()
	deleteNow := start
	deleteStore.now = func() time.Time { return deleteNow }
	for i, access := range []string{"delete-old", "delete-new"} {
		if err := deleteStore.SetYggToken(ctx, model.Token{
			AccessToken: access,
			UserID:      "delete-user",
			CreatedAt:   int64(i + 1),
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	deleteIndexKey := deleteStore.yggUserTokensKey("delete-user")
	deleteExpiry := deleteStore.items[deleteIndexKey].expiresAt
	deleteNow = deleteNow.Add(30 * time.Second)
	if err := deleteStore.DeleteYggToken(ctx, "delete-old"); err != nil {
		t.Fatal(err)
	}
	if got := deleteStore.items[deleteIndexKey].expiresAt; !got.Equal(deleteExpiry) {
		t.Fatalf("single-token delete changed index expiry: got=%v want=%v", got, deleteExpiry)
	}
	deleteNow = deleteExpiry
	if _, err := deleteStore.yggTokenIndex("delete-user"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted-token index should expire at original boundary, got %v", err)
	}

	trimStore := NewMemoryStore()
	trimNow := start
	trimStore.now = func() time.Time { return trimNow }
	for i, access := range []string{"trim-1", "trim-2", "trim-3"} {
		if err := trimStore.SetYggToken(ctx, model.Token{
			AccessToken: access,
			UserID:      "trim-user",
			CreatedAt:   int64(i + 1),
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	trimIndexKey := trimStore.yggUserTokensKey("trim-user")
	trimExpiry := trimStore.items[trimIndexKey].expiresAt
	trimNow = trimNow.Add(30 * time.Second)
	if err := trimStore.TrimYggTokensByUser(ctx, "trim-user", 1); err != nil {
		t.Fatal(err)
	}
	if got := trimStore.items[trimIndexKey].expiresAt; !got.Equal(trimExpiry) {
		t.Fatalf("token trim changed index expiry: got=%v want=%v", got, trimExpiry)
	}
	trimNow = trimExpiry
	if _, err := trimStore.yggTokenIndex("trim-user"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("trimmed-token index should expire at original boundary, got %v", err)
	}
}

func TestMemoryStoreOAuthAccessTokenReadsStoredStructAndRawMapExactly(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	structToken := OAuthAccessToken{
		TokenHash:     "struct-token",
		ClientID:      "client-struct",
		UserID:        "user-struct",
		GrantID:       "grant-struct",
		PermissionIDs: []int64{7, 8},
		ExpiresAt:     1700,
		CreatedAt:     1100,
	}
	store.items["oauth:access:"+structToken.TokenHash] = memoryItem{value: structToken}
	gotStruct, err := store.GetOAuthAccessToken(ctx, structToken.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotStruct, structToken) {
		t.Fatalf("stored struct token mismatch:\n got=%#v\nwant=%#v", gotStruct, structToken)
	}

	store.items["oauth:access:raw-token"] = memoryItem{value: map[string]any{
		"token_hash":     "raw-token",
		"client_id":      42,
		"user_id":        "raw-user",
		"grant_id":       "raw-grant",
		"permission_ids": []any{float64(10), int64(11), int(12), "bad"},
		"expires_at":     int64(2600),
		"created_at":     int(1200),
	}}
	gotRaw, err := store.GetOAuthAccessToken(ctx, "raw-token")
	if err != nil {
		t.Fatal(err)
	}
	wantRaw := OAuthAccessToken{
		TokenHash:     "raw-token",
		ClientID:      "",
		UserID:        "raw-user",
		GrantID:       "raw-grant",
		PermissionIDs: []int64{10, 11, 12, 0},
		ExpiresAt:     2600,
		CreatedAt:     1200,
	}
	if !reflect.DeepEqual(gotRaw, wantRaw) {
		t.Fatalf("raw map token mismatch:\n got=%#v\nwant=%#v", gotRaw, wantRaw)
	}
}

func TestMemoryStorePrefixTTLAndErrorContracts(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	now := time.UnixMilli(1_000)
	store.now = func() time.Time { return now }

	if err := store.SetSetting(ctx, "site_name", "A", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "one", Type: "image", StoragePath: "one.png"}}, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteByPrefix(ctx, "settings:"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("prefixed delete should remove matching settings key, got %v", err)
	}
	if homepageMedia, err := store.GetPublicHomepageMedia(ctx); err != nil || len(homepageMedia) != 1 || homepageMedia[0].StoragePath != "one.png" {
		t.Fatalf("prefixed delete should not remove unrelated homepage media cache: %#v err=%v", homepageMedia, err)
	}

	if err := store.SetVerificationCode(ctx, "ttl@example.com", "register", "12345678", time.Second); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := store.GetVerificationCode(ctx, "ttl@example.com", "register"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("verification code should expire at ttl boundary, got %v", err)
	}

	if err := store.SetState(ctx, "state-token", map[string]any{"kind": "oauth_state", "user_id": "u1"}, time.Second); err != nil {
		t.Fatal(err)
	}
	state, err := store.PopState(ctx, "state-token")
	if err != nil || state["kind"] != "oauth_state" || state["user_id"] != "u1" {
		t.Fatalf("state pop mismatch: state=%#v err=%v", state, err)
	}
	if _, err := store.PopState(ctx, "state-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should be single-use, got %v", err)
	}
	if err := store.SetState(ctx, "expired-state", map[string]any{"kind": "profile"}, time.Second); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := store.PopState(ctx, "expired-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should expire at ttl boundary, got %v", err)
	}

	boom := errors.New("cache unavailable")
	store.Err = boom
	if err := store.SetSetting(ctx, "site_name", "B", time.Minute); !errors.Is(err, boom) {
		t.Fatalf("set should propagate store error, got %v", err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, boom) {
		t.Fatalf("get should propagate store error, got %v", err)
	}
	if err := store.InvalidateAuthUser(ctx, "u1"); !errors.Is(err, boom) {
		t.Fatalf("invalidate auth should propagate store error, got %v", err)
	}
	if err := store.DeleteFallbackRequest(ctx, "endpoint", "request"); !errors.Is(err, boom) {
		t.Fatalf("delete fallback guard should propagate store error, got %v", err)
	}
	if err := store.SetState(ctx, "state-error", map[string]any{"kind": "oauth_state"}, time.Minute); !errors.Is(err, boom) {
		t.Fatalf("state set should propagate store error, got %v", err)
	}
	if _, err := store.PopState(ctx, "state-error"); !errors.Is(err, boom) {
		t.Fatalf("state pop should propagate store error, got %v", err)
	}
	if _, err := store.HitRateLimit(ctx, "login", 1, time.Minute); !errors.Is(err, boom) {
		t.Fatalf("rate limit should propagate store error, got %v", err)
	}
}
