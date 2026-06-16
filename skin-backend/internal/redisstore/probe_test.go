package redisstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreProbeHistoryAppendTrimsByRetentionAndOrders(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	now := time.Now()
	old := redisstore.ProbeSample{EndpointID: 1, Note: "primary", CheckedAt: now.Add(-2 * time.Hour).UnixMilli(), Session: "up", Account: "up", Services: "up"}
	mid := redisstore.ProbeSample{EndpointID: 1, Note: "primary", CheckedAt: now.Add(-30 * time.Minute).UnixMilli(), Session: "up", Account: "down", Services: "up"}
	fresh := redisstore.ProbeSample{EndpointID: 2, Note: "backup", CheckedAt: now.UnixMilli(), Session: "down", Account: "up", Services: "down"}

	if err := store.AppendProbeSamples(ctx, []redisstore.ProbeSample{old, mid, fresh}, time.Hour); err != nil {
		t.Fatalf("append: %v", err)
	}
	all, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(all) != 2 || all[0].EndpointID != 1 || all[0].Account != "down" || all[1].EndpointID != 2 {
		t.Fatalf("retention should drop samples older than 1h and order ascending: %#v", all)
	}

	since := now.Add(-15 * time.Minute)
	recent, err := store.GetProbeHistory(ctx, since)
	if err != nil || len(recent) != 1 || recent[0].EndpointID != 2 || recent[0].Session != "down" {
		t.Fatalf("since filter mismatch: %#v err=%v", recent, err)
	}
}

func TestMemoryStoreProbeHistoryAppendIsAdditiveAndCanBeInvalidated(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	base := time.Now().Add(-10 * time.Minute)

	first := redisstore.ProbeSample{EndpointID: 7, CheckedAt: base.UnixMilli(), Session: "up", Account: "up", Services: "up"}
	second := redisstore.ProbeSample{EndpointID: 7, CheckedAt: base.Add(2 * time.Minute).UnixMilli(), Session: "down", Account: "up", Services: "up"}
	if err := store.AppendProbeSamples(ctx, []redisstore.ProbeSample{first}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendProbeSamples(ctx, []redisstore.ProbeSample{second}, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil || len(got) != 2 || got[0].Session != "up" || got[1].Session != "down" {
		t.Fatalf("appending should preserve existing samples in chronological order: %#v err=%v", got, err)
	}
	if err := store.InvalidateProbeHistory(ctx); err != nil {
		t.Fatal(err)
	}
	cleared, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil || len(cleared) != 0 {
		t.Fatalf("invalidate should drop all history: %#v err=%v", cleared, err)
	}
}

func TestMemoryStoreProbeHistoryAppendNoSamplesIsNoop(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	if err := store.AppendProbeSamples(ctx, nil, time.Hour); err != nil {
		t.Fatalf("nil samples should not error: %v", err)
	}
	got, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil || len(got) != 0 {
		t.Fatalf("empty store should return empty history: %#v err=%v", got, err)
	}
}

func TestMemoryStoreProbeHistoryRespectsBackingError(t *testing.T) {
	store := redisstore.NewMemoryStore()
	store.Err = errors.New("redis down")
	ctx := context.Background()
	if err := store.AppendProbeSamples(ctx, []redisstore.ProbeSample{{EndpointID: 1, CheckedAt: time.Now().UnixMilli()}}, time.Hour); err == nil {
		t.Fatalf("expected error when backing store fails")
	}
	if _, err := store.GetProbeHistory(ctx, time.Time{}); err == nil {
		t.Fatalf("expected error when backing store fails")
	}
	if err := store.InvalidateProbeHistory(ctx); err == nil {
		t.Fatalf("expected error when backing store fails")
	}
}
