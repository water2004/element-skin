package testutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

const testDBName = "elementskin_go_test"

var dbCounter uint64

func TestConfig() config.Config {
	cfg := config.Defaults()
	cfg.DatabaseDSN = os.Getenv("TEST_DATABASE_DSN")
	if cfg.DatabaseDSN == "" {
		cfg.DatabaseDSN = "postgresql://postgres:12345678@localhost:5432/" + testDBName + "?sslmode=disable"
	}
	cfg.JWTSecret = "abcdefghijklmnopqrstuvwxyz123456"
	cfg.SiteURL = "http://test"
	cfg.APIURL = "http://localhost:8000"
	cfg.PrivateKeyPath = filepath.Join(repoRoot(), "private.pem")
	cfg.PublicKeyPath = filepath.Join(repoRoot(), "public.pem")
	return cfg
}

func repoRoot() string {
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return "."
}

func NewTestApp(t *testing.T) (*database.DB, http.Handler) {
	return NewTestAppTB(t)
}

func NewTestAppTB(t testing.TB) (*database.DB, http.Handler) {
	return newTestAppTB(t, nil)
}

func NewTestAppWithMaxConnectionsTB(t testing.TB, maxConnections int32) (*database.DB, http.Handler) {
	return newTestAppTB(t, func(cfg *config.Config) {
		if maxConnections > 0 {
			cfg.MaxConnections = maxConnections
		}
	})
}

func newTestAppTB(t testing.TB, configure func(*config.Config)) (*database.DB, http.Handler) {
	t.Helper()
	ctx := context.Background()
	cfg := TestConfig()
	cfg.TexturesDir = t.TempDir()
	cfg.CarouselDir = t.TempDir()
	dbName := fmt.Sprintf("%s_%d_%d", testDBName, os.Getpid(), atomic.AddUint64(&dbCounter, 1))
	cfg.DatabaseDSN = "postgresql://postgres:12345678@localhost:5432/" + dbName + "?sslmode=disable"
	if configure != nil {
		configure(&cfg)
	}
	ensureTestDatabase(t, ctx, dbName)
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.ResetPublicSchema(ctx); err != nil {
		db.Close()
		t.Fatalf("reset test schema: %v", err)
	}
	t.Cleanup(func() { dropTestDatabase(t, context.Background(), dbName) })
	t.Cleanup(db.Close)
	application, err := app.NewWithDB(cfg, db)
	if err != nil {
		t.Fatalf("build test app: %v", err)
	}
	return db, application.Handler()
}

func ensureTestDatabase(t testing.TB, ctx context.Context, dbName string) {
	t.Helper()
	adminDSN := os.Getenv("ADMIN_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	defer conn.Close(ctx)
	var exists int
	err = conn.QueryRow(ctx, "SELECT 1 FROM pg_database WHERE datname=$1", dbName).Scan(&exists)
	if database.IsNoRows(err) {
		_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	}
	if err != nil {
		t.Fatalf("ensure test database: %v", err)
	}
}

func dropTestDatabase(t testing.TB, ctx context.Context, dbName string) {
	t.Helper()
	adminDSN := os.Getenv("ADMIN_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("connect admin database for cleanup: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
		t.Fatalf("drop test database: %v", err)
	}
}

func CreateUser(t testing.TB, db *database.DB, email, password, username string, isAdmin bool) model.User {
	t.Helper()
	if email == "" {
		email = randomID(t)[:8] + "@example.com"
	}
	if username == "" {
		username = "User_" + randomID(t)[:8]
	}
	hash, err := util.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{
		ID: randomID(t), Email: email, Password: hash,
		IsAdmin: isAdmin, PreferredLanguage: "zh_CN", DisplayName: username,
	}
	if err := db.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func CreateProfile(t testing.TB, db *database.DB, userID, id, name string) model.Profile {
	t.Helper()
	if id == "" {
		id = randomID(t)
	}
	p := model.Profile{ID: id, UserID: userID, Name: name, TextureModel: "default"}
	if err := db.Profiles.Create(context.Background(), p); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return p
}

func randomID(t testing.TB) string {
	t.Helper()
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		t.Fatalf("generate uuid: %v", err)
	}
	return id
}
