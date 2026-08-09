package redisstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRedisStoreStateIsAtomicSingleUseAndExpiresExactly(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()

	if _, err := store.PopState(ctx, "missing-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing state error=%v, want ErrCacheMiss", err)
	}

	state := map[string]any{
		"kind":    "oauth_state",
		"user_id": "user-1",
		"profile": map[string]any{
			"id":   "profile-1",
			"name": "PlayerOne",
		},
	}
	if err := store.SetState(ctx, "oauth-token", state, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.PopState(ctx, "oauth-token")
	if err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "oauth_state" || got["user_id"] != "user-1" {
		t.Fatalf("state scalar fields mismatch: %#v", got)
	}
	profile, ok := got["profile"].(map[string]any)
	if !ok || profile["id"] != "profile-1" || profile["name"] != "PlayerOne" {
		t.Fatalf("state nested profile mismatch: %#v", got)
	}
	if _, err := store.PopState(ctx, "oauth-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should be consumed atomically, got %v", err)
	}

	if err := store.SetState(ctx, "expires-token", map[string]any{"kind": "profile"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	server.FastForward(time.Minute)
	if _, err := store.PopState(ctx, "expires-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should expire at TTL boundary, got %v", err)
	}
}

func TestRedisStoreStateReadDeleteNullAndMalformedPayloadsExactly(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	state := map[string]any{"kind": "authorization", "attempt": float64(2)}
	if err := store.SetState(ctx, "readable-state", state, time.Minute); err != nil {
		t.Fatal(err)
	}
	read, err := store.GetState(ctx, "readable-state")
	if err != nil || !reflect.DeepEqual(read, state) {
		t.Fatalf("read state=%#v err=%v want=%#v", read, err, state)
	}
	readAgain, err := store.GetState(ctx, "readable-state")
	if err != nil || !reflect.DeepEqual(readAgain, state) {
		t.Fatalf("non-consuming state read=%#v err=%v", readAgain, err)
	}
	if err := store.DeleteState(ctx, "readable-state"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetState(ctx, "readable-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted state error=%v want ErrCacheMiss", err)
	}

	if err := store.client.Set(ctx, store.stateKey("null-get"), "null", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	nullState, err := store.GetState(ctx, "null-get")
	if err != nil || nullState == nil || len(nullState) != 0 {
		t.Fatalf("null get state=%#v err=%v", nullState, err)
	}
	if err := store.client.Set(ctx, store.stateKey("null-pop"), "null", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	nullState, err = store.PopState(ctx, "null-pop")
	if err != nil || nullState == nil || len(nullState) != 0 {
		t.Fatalf("null pop state=%#v err=%v", nullState, err)
	}
	if _, err := store.PopState(ctx, "null-pop"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("null state was not consumed: %v", err)
	}

	if err := store.client.Set(ctx, store.stateKey("malformed-state"), "{", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	malformed, err := store.PopState(ctx, "malformed-state")
	if malformed != nil || err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("malformed state=%#v err=%v", malformed, err)
	}
	if _, err := store.PopState(ctx, "malformed-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("malformed state was not consumed: %v", err)
	}
}

func TestMemoryStateDeleteAndStructuredFallbacksExactly(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if err := store.SetState(ctx, "delete-state", map[string]any{"kind": "temporary"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteState(ctx, "delete-state"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetState(ctx, "delete-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("memory deleted state error=%v want ErrCacheMiss", err)
	}
	wantErr := errors.New("memory state unavailable")
	store.Err = wantErr
	if err := store.DeleteState(ctx, "delete-state"); !errors.Is(err, wantErr) {
		t.Fatalf("memory delete dependency error=%v", err)
	}
	store.Err = nil

	type statePayload struct {
		Kind  string `json:"kind"`
		Count int    `json:"count"`
	}
	converted := memoryStateMap(statePayload{Kind: "structured", Count: 3})
	if converted["kind"] != "structured" || converted["count"] != float64(3) || len(converted) != 2 {
		t.Fatalf("structured memory state=%#v", converted)
	}
	empty := memoryStateMap(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("nil memory state=%#v", empty)
	}
}

func TestMemoryExternalAccessTokenConvertsStructuredMapsAndRejectsInvalidValuesExactly(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	key := "identity:access:map-identity"
	store.items[key] = memoryItem{value: map[string]any{
		"identity_id": "map-identity", "access_token": "map-access", "token_type": "Bearer", "expires_at": float64(12345),
	}}
	token, err := store.GetExternalAccessToken(ctx, "map-identity")
	if err != nil || token.IdentityID != "map-identity" || token.AccessToken != "map-access" ||
		token.TokenType != "Bearer" || token.ExpiresAt != 12345 {
		t.Fatalf("map access token=%#v err=%v", token, err)
	}
	store.items["identity:access:invalid-identity"] = memoryItem{value: "invalid"}
	if token, err := store.GetExternalAccessToken(ctx, "invalid-identity"); !errors.Is(err, ErrCacheMiss) || token != (ExternalAccessToken{}) {
		t.Fatalf("invalid access token=%#v err=%v", token, err)
	}
	wantErr := errors.New("external access cache unavailable")
	store.Err = wantErr
	if err := store.DeleteExternalAccessToken(ctx, "map-identity"); !errors.Is(err, wantErr) {
		t.Fatalf("external access delete error=%v", err)
	}
}
