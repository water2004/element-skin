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
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	mailsvc "element-skin/backend/internal/service/mail"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

const testDBName = "elementskin_go_test"

var dbCounter uint64

func Pointer[T any](value T) *T {
	return &value
}

func TestConfig() config.Config {
	cfg := config.Config{
		DatabaseHost:     "localhost",
		DatabasePort:     "5432",
		DatabaseUser:     "postgres",
		DatabasePassword: "12345678",
		DatabaseName:     testDBName,
		DatabaseSSLMode:  "disable",
		MaxConnections:   10,
		JWTSecret:        "abcdefghijklmnopqrstuvwxyz123456",
		JWTExpireDays:    7,
		AccessMinutes:    30,
		SiteURL:          "http://test",
		APIURL:           "http://localhost:8000",
		ServerHost:       "127.0.0.1",
		ServerPort:       "8000",
		TexturesDir:      "textures",
		CarouselDir:      "carousel",
		RedisHost:        "127.0.0.1",
		RedisPort:        "6379",
		RedisAddr:        "127.0.0.1:6379",
		RedisDB:          0,
		RedisKeyPrefix:   "elementskin:test:",
		PublicCacheTTL:   60,
		AuthCacheTTL:     30,
		CORSOrigins:      []string{"http://test"},
		CORSCredentials:  true,
	}
	cfg.DatabaseDSN = os.Getenv("TEST_DATABASE_DSN")
	if cfg.DatabaseDSN == "" {
		cfg.DatabaseDSN = "postgresql://postgres:12345678@localhost:5432/" + testDBName + "?sslmode=disable"
	}
	cfg.PrivateKeyPath = filepath.Join(repoRoot(), "private.pem")
	cfg.PublicKeyPath = filepath.Join(repoRoot(), "public.pem")
	cfg.RedisAddr = os.Getenv("REDIS_TEST_ADDR")
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "127.0.0.1:6379"
	}
	cfg.RedisPassword = os.Getenv("REDIS_TEST_PASSWORD")
	cfg.RedisKeyPrefix = fmt.Sprintf("elementskin:test:%d:", os.Getpid())
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
	db, handler, _ := NewTestAppWithRedisTB(t)
	return db, handler
}

func NewTestAppTB(t testing.TB) (*database.DB, http.Handler) {
	db, handler, _ := newTestAppTB(t, nil)
	return db, handler
}

func NewTestAppWithMaxConnectionsTB(t testing.TB, maxConnections int32) (*database.DB, http.Handler) {
	db, handler, _ := NewTestAppWithMaxConnectionsAndRedisTB(t, maxConnections)
	return db, handler
}

func NewTestAppWithMaxConnectionsAndRedisTB(t testing.TB, maxConnections int32) (*database.DB, http.Handler, redisstore.Store) {
	return newTestAppTB(t, func(cfg *config.Config) {
		if maxConnections > 0 {
			cfg.MaxConnections = maxConnections
		}
	})
}

func NewTestAppWithRedisTB(t testing.TB) (*database.DB, http.Handler, redisstore.Store) {
	return newTestAppTB(t, nil)
}

func newTestAppTB(t testing.TB, configure func(*config.Config)) (*database.DB, http.Handler, redisstore.Store) {
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
	var redis redisstore.Store
	if usingIntegrationRedis() {
		redis = NewRedisStoreTB(t, cfg.RedisKeyPrefix+dbName+":")
	} else {
		redis = NewMemoryRedis()
	}
	application, err := app.NewWithDBAndRedis(cfg, db, redis, testMailSender{})
	if err != nil {
		t.Fatalf("build test app: %v", err)
	}
	return db, application.Handler(), redis
}

type testMailSender struct{}

func (testMailSender) SendVerificationCode(context.Context, string, string, string) error {
	return nil
}

var _ mailsvc.Sender = testMailSender{}

func NewRedisStoreTB(t testing.TB, prefix string) redisstore.Store {
	t.Helper()
	cfg := TestConfig()
	if prefix != "" {
		cfg.RedisKeyPrefix = prefix
	}
	store, err := redisstore.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open redis test store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.DeleteByPrefix(context.Background(), ""); err != nil {
			t.Fatalf("cleanup redis test keys: %v", err)
		}
		_ = store.Close()
	})
	return store
}

func NewMemoryRedis() redisstore.Store {
	return redisstore.NewMemoryStore()
}

func usingIntegrationRedis() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	base := filepath.Base(cwd)
	parent := filepath.Base(filepath.Dir(cwd))
	return (base == "integration" && parent == "internal") || (base == "loadtest" && parent == "cmd")
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

func CreateUser(t testing.TB, db *database.DB, email, password, username string, isAdmin bool, protectedSubject ...bool) model.User {
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
	protected := false
	if len(protectedSubject) > 0 {
		protected = protectedSubject[0]
	}
	user := model.User{
		ID: randomID(t), Email: email, Password: hash,
		PreferredLanguage: "zh_CN", DisplayName: username,
	}
	if err := db.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Permissions.EnsureUserSubject(context.Background(), user.ID); err != nil {
		t.Fatalf("create user permission subject: %v", err)
	}
	if isAdmin {
		if err := db.Permissions.GrantRole(context.Background(), user.ID, permission.RoleAdmin, ""); err != nil {
			t.Fatalf("grant admin role: %v", err)
		}
	}
	if protected {
		if !isAdmin {
			if err := db.Permissions.GrantRole(context.Background(), user.ID, permission.RoleAdmin, ""); err != nil {
				t.Fatalf("grant protected manager admin role: %v", err)
			}
		}
		if _, err := db.Pool.Exec(context.Background(), `
			UPDATE permission_subjects SET protected=TRUE WHERE id=$1
		`, "user:"+user.ID); err != nil {
			t.Fatalf("mark protected subject: %v", err)
		}
		if err := db.Permissions.SetSubjectPermissionOverride(context.Background(), user.ID, permission.MustDefinitionByCode("permission_protected.manage.any"), "allow", ""); err != nil {
			t.Fatalf("grant protected manager permission: %v", err)
		}
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
