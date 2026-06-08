package testutil

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

func TestTestConfigExactDefaults(t *testing.T) {
	t.Setenv("TEST_DATABASE_DSN", "postgresql://example/test")
	cfg := TestConfig()
	if cfg.DatabaseDSN != "postgresql://example/test" || cfg.JWTSecret != "abcdefghijklmnopqrstuvwxyz123456" ||
		cfg.SiteURL != "http://test" || cfg.APIURL != "http://localhost:8000" {
		t.Fatalf("TestConfig mismatch: %#v", cfg)
	}
	if !strings.HasSuffix(cfg.PrivateKeyPath, "private.pem") || !strings.HasSuffix(cfg.PublicKeyPath, "public.pem") {
		t.Fatalf("TestConfig should point at Yggdrasil test keys: %#v", cfg)
	}
}

func TestNewTestAppCreateHelpersExactState(t *testing.T) {
	db, handler := NewTestApp(t)
	if handler == nil {
		t.Fatal("NewTestApp should return a handler")
	}
	ctx := context.Background()
	user := CreateUser(t, db, "", "Password123", "", true)
	emailLocal := strings.TrimSuffix(user.Email, "@example.com")
	displaySuffix := strings.TrimPrefix(user.DisplayName, "User_")
	if len(emailLocal) != 8 || !strings.HasSuffix(user.Email, "@example.com") || !strings.HasPrefix(user.DisplayName, "User_") || len(displaySuffix) != 8 || !user.IsAdmin {
		t.Fatalf("CreateUser generated fields mismatch: %#v", user)
	}
	stored, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Email != user.Email || !util.VerifyPassword("Password123", stored.Password) {
		t.Fatalf("CreateUser did not persist expected user: %#v", stored)
	}
	profile := CreateProfile(t, db, user.ID, "", "GeneratedProfile")
	if profile.UserID != user.ID || profile.Name != "GeneratedProfile" || profile.TextureModel != "default" || len(profile.ID) != 32 {
		t.Fatalf("CreateProfile generated fields mismatch: %#v", profile)
	}
	if ok, err := db.Profiles.VerifyOwnership(ctx, user.ID, profile.ID); err != nil || !ok {
		t.Fatalf("CreateProfile should persist ownership: ok=%v err=%v", ok, err)
	}
}

func TestEnsureTestDatabaseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbName := "elementskin_go_test_idempotent"
	t.Cleanup(func() { dropTestDatabase(t, context.Background(), dbName) })
	ensureTestDatabase(t, ctx, dbName)
	ensureTestDatabase(t, ctx, dbName)
	cfg := TestConfig()
	cfg.DatabaseDSN = "postgresql://postgres:12345678@localhost:5432/" + dbName + "?sslmode=disable"
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
}
