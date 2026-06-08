package util

import (
	"testing"
	"time"
)

func TestInMemoryStateStorePopIsOneShotAndSweepsExpired(t *testing.T) {
	store := NewInMemoryStateStore()
	store.Put("k", map[string]any{"user_id": "u1"}, time.Minute)
	got := store.Pop("k").(map[string]any)
	if got["user_id"] != "u1" || store.Pop("k") != nil || store.Pop("missing") != nil {
		t.Fatalf("state store pop semantics mismatch: got=%#v len=%d", got, store.Len())
	}
	store.Put("old", "v", 0)
	time.Sleep(10 * time.Millisecond)
	if store.Pop("old") != nil {
		t.Fatal("expired item should return nil")
	}
	store.Put("expired", "v", 0)
	time.Sleep(10 * time.Millisecond)
	store.Put("new", "v2", time.Minute)
	if store.Len() != 1 || store.Pop("new") != "v2" {
		t.Fatalf("put should sweep expired items, len=%d", store.Len())
	}
}
