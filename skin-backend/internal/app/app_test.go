package app_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestNewRejectsWeakJWTSecret(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.JWTSecret = "short"
	if _, err := app.New(context.Background(), cfg); err == nil {
		t.Fatal("weak JWT secret should reject startup")
	}
}

func TestRefreshCleanupLoopRemovesExpiredThenCancels(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "cleanup@example.com", "Password123!", "CleanupUser", false)
	now := database.NowMS()
	if err := db.Tokens.AddRefresh(context.Background(), "hash_old", user.ID, now-10_000, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(context.Background(), "hash_new", user.ID, now+7*24*3600*1000, now); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.RunRefreshCleanupLoop(ctx, db.Tokens, 10*time.Millisecond)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		row, err := db.Tokens.GetRefresh(context.Background(), "hash_old")
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup loop did not stop after cancellation")
	}
	if row, err := db.Tokens.GetRefresh(context.Background(), "hash_old"); err != nil || row != nil {
		t.Fatalf("expired refresh token should be removed: row=%#v err=%v", row, err)
	}
	if row, err := db.Tokens.GetRefresh(context.Background(), "hash_new"); err != nil || row == nil {
		t.Fatalf("future refresh token should be retained: row=%#v err=%v", row, err)
	}
}

type flakyRefreshCleaner struct {
	calls atomic.Int64
}

func (f *flakyRefreshCleaner) DeleteExpiredRefresh(context.Context, int64) error {
	f.calls.Add(1)
	return errors.New("boom")
}

func TestRefreshCleanupLoopSurvivesCleanupError(t *testing.T) {
	cleaner := &flakyRefreshCleaner{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.RunRefreshCleanupLoop(ctx, cleaner, 10*time.Millisecond)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && cleaner.calls.Load() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup loop did not stop after cancellation")
	}
	if cleaner.calls.Load() < 2 {
		t.Fatalf("cleanup loop should continue after errors, calls=%d", cleaner.calls.Load())
	}
}

func TestNewWithDBAndRedisClosesRedisWhenSignerInitializationFails(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = t.TempDir() + "/missing-private.pem"
	cache := &closeTrackingStore{Store: redisstore.NewMemoryStore()}

	application, err := app.NewWithDBAndRedis(cfg, nil, cache)
	if err == nil || application != nil {
		t.Fatalf("missing signing key should fail app construction: app=%#v err=%v", application, err)
	}
	if !cache.closed {
		t.Fatal("failed app construction must close the already-open Redis store")
	}
}

func TestAppCloseReleasesDatabaseAndRedis(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := &closeTrackingStore{Store: redisstore.NewMemoryStore()}
	application, err := app.NewWithDBAndRedis(cfg, db, cache)
	if err != nil {
		t.Fatal(err)
	}

	application.Close()

	if !cache.closed {
		t.Fatal("App.Close must close Redis")
	}
	if err := db.Pool.Ping(context.Background()); err == nil {
		t.Fatal("App.Close must close the database pool")
	}
}

type closeTrackingStore struct {
	redisstore.Store
	closed bool
}

func (s *closeTrackingStore) Close() error {
	s.closed = true
	return s.Store.Close()
}
