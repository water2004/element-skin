package app_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

var appNewDatabaseCounter atomic.Uint64

func TestNewRejectsWeakJWTSecret(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.JWTSecret = "short"
	if _, err := app.New(context.Background(), cfg); err == nil {
		t.Fatal("weak JWT secret should reject startup")
	}
}

func TestNewOpensDependenciesBuildsRouterAndClosesExactly(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.TestConfig()
	dbName := fmt.Sprintf("elementskin_app_new_%d_%d", os.Getpid(), appNewDatabaseCounter.Add(1))
	cfg.DatabaseDSN = createTemporaryDatabaseForAppNew(t, dbName)
	cfg.TexturesDir = t.TempDir()
	cfg.CarouselDir = t.TempDir()
	cfg.RedisKeyPrefix = cfg.RedisKeyPrefix + dbName + ":"

	application, err := app.New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/public/settings", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name"`) {
		t.Fatalf("app.New router response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	application.Close()
	dropTemporaryDatabaseForAppNew(t, dbName)
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

func TestNewWithDBBuildsWorkingRouterAndCloseReleasesRedis(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.RedisKeyPrefix = cfg.RedisKeyPrefix + "app-new-with-db:"
	application, err := app.NewWithDB(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	req := httptest.NewRequest(http.MethodGet, "/v2/public/settings", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name"`) {
		t.Fatalf("NewWithDB router response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestNewWithDBReturnsExactRedisConnectionError(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.RedisAddr = "127.0.0.1:1"
	application, err := app.NewWithDB(cfg, db)
	if application != nil || err == nil || !strings.Contains(err.Error(), "connect redis 127.0.0.1:1") {
		t.Fatalf("NewWithDB redis error mismatch: app=%#v err=%v", application, err)
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

func createTemporaryDatabaseForAppNew(t *testing.T, dbName string) string {
	t.Helper()
	adminDSN := os.Getenv("ADMIN_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), adminDSN)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		t.Fatalf("create app.New test database: %v", err)
	}
	t.Cleanup(func() {
		dropTemporaryDatabaseForAppNew(t, dbName)
	})
	return "postgresql://postgres:12345678@localhost:5432/" + dbName + "?sslmode=disable"
}

func dropTemporaryDatabaseForAppNew(t *testing.T, dbName string) {
	t.Helper()
	adminDSN := os.Getenv("ADMIN_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), adminDSN)
	if err != nil {
		t.Fatalf("connect admin database for app.New cleanup: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
		t.Fatalf("drop app.New test database: %v", err)
	}
}
