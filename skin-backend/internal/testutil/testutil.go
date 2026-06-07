package testutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	return cfg
}

func NewTestApp(t *testing.T) (*database.DB, http.Handler) {
	t.Helper()
	ctx := context.Background()
	cfg := TestConfig()
	cfg.TexturesDir = t.TempDir()
	cfg.CarouselDir = t.TempDir()
	dbName := fmt.Sprintf("%s_%d_%d", testDBName, os.Getpid(), atomic.AddUint64(&dbCounter, 1))
	cfg.DatabaseDSN = "postgresql://postgres:12345678@localhost:5432/" + dbName + "?sslmode=disable"
	ensureTestDatabase(t, ctx, dbName)
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.ResetPublicSchema(ctx); err != nil {
		db.Close()
		t.Fatalf("reset test schema: %v", err)
	}
	t.Cleanup(db.Close)
	return db, app.NewWithDB(cfg, db).Handler()
}

func ensureTestDatabase(t *testing.T, ctx context.Context, dbName string) {
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

func CreateUser(t *testing.T, db *database.DB, email, password, username string, isAdmin bool) model.User {
	t.Helper()
	if email == "" {
		email = util.RandomUUIDNoDash()[:8] + "@example.com"
	}
	if username == "" {
		username = "User_" + util.RandomUUIDNoDash()[:8]
	}
	hash, err := util.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{
		ID: util.RandomUUIDNoDash(), Email: email, Password: hash,
		IsAdmin: isAdmin, PreferredLanguage: "zh_CN", DisplayName: username,
	}
	if err := db.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func CreateProfile(t *testing.T, db *database.DB, userID, id, name string) model.Profile {
	t.Helper()
	if id == "" {
		id = util.RandomUUIDNoDash()
	}
	p := model.Profile{ID: id, UserID: userID, Name: name, TextureModel: "default"}
	if err := db.CreateProfile(context.Background(), p); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return p
}
