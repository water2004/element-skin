package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestAppNewWithDBUsesRealRedisAndCloseReleasesDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.RedisKeyPrefix += "app-with-db:"
	cleanupRealRedisPrefix(t, cfg)

	application, err := app.NewWithDB(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/public/settings", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("real-redis app handler status=%d body=%q", rec.Code, rec.Body.String())
	}

	application.Close()
	if err := db.Pool.Ping(context.Background()); err == nil {
		t.Fatal("App.Close should release the database pool")
	}

	// Close is intentionally idempotent so shutdown paths can safely defer it.
	application.Close()
}

func TestAppNewCleansExpiredRefreshAndServesRequests(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "app-new@test.com", "Password123", "AppNew", false)
	now := database.NowMS()
	if err := db.Tokens.AddRefresh(t.Context(), "expired-before-start", user.ID, now-1, now-2); err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(t.Context(), "future-before-start", user.ID, now+60_000, now); err != nil {
		t.Fatal(err)
	}

	cfg := testutil.TestConfig()
	cfg.DatabaseDSN = db.Pool.Config().ConnString()
	cfg.RedisKeyPrefix += "app-new:" + user.ID + ":"
	cleanupRealRedisPrefix(t, cfg)
	application, err := app.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	if row, err := db.Tokens.GetRefresh(t.Context(), "expired-before-start"); err != nil || row != nil {
		t.Fatalf("startup should delete expired refresh tokens: row=%#v err=%v", row, err)
	}
	if row, err := db.Tokens.GetRefresh(t.Context(), "future-before-start"); err != nil || row == nil {
		t.Fatalf("startup should preserve future refresh tokens: row=%#v err=%v", row, err)
	}

	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/public/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("fully constructed app handler status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func cleanupRealRedisPrefix(t *testing.T, cfg config.Config) {
	t.Helper()
	t.Cleanup(func() {
		store, err := redisstore.Open(context.Background(), cfg)
		if err != nil {
			t.Errorf("open redis for cleanup: %v", err)
			return
		}
		defer store.Close()
		if err := store.DeleteByPrefix(context.Background(), ""); err != nil {
			t.Errorf("clean app redis prefix: %v", err)
		}
	})
}
