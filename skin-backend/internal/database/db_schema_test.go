package database_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestDBInitSchemaDefaultsAndCoreHelpers(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Init should be idempotent: %v", err)
	}
	for _, table := range []string{"users", "profiles", "tokens", "site_refresh_tokens", "sessions", "invites", "settings", "user_textures", "skin_library", "fallback_endpoints", "whitelisted_users", "verification_codes"} {
		var exists bool
		if err := db.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("InitSQL should create table %s", table)
		}
	}
	siteName, err := db.Settings.Get(ctx, "site_name", "")
	if err != nil {
		t.Fatal(err)
	}
	if siteName != "皮肤站" {
		t.Fatalf("InitSQL should seed site_name, got %q", siteName)
	}

	avatar := "avatar_hash"
	user := testutil.CreateUser(t, db, "scan@test.com", "Password123", "ScanUser", true)
	if err := db.Users.Update(ctx, user.ID, map[string]any{"avatar_hash": avatar}); err != nil {
		t.Fatal(err)
	}
	scannedUser, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scannedUser == nil || scannedUser.ID != user.ID || scannedUser.Email != user.Email || !scannedUser.IsAdmin ||
		scannedUser.PreferredLanguage != "zh_CN" || scannedUser.DisplayName != "ScanUser" || scannedUser.BannedUntil != nil ||
		scannedUser.AvatarHash == nil || *scannedUser.AvatarHash != "avatar_hash" {
		t.Fatalf("GetUserByID/scanUser mismatch: %#v", scannedUser)
	}

	skin := "skin"
	profile := testutil.CreateProfile(t, db, user.ID, "scan_profile", "ScanProfile")
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}
	scannedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if scannedProfile == nil || scannedProfile.ID != profile.ID || scannedProfile.UserID != user.ID || scannedProfile.Name != "ScanProfile" ||
		scannedProfile.TextureModel != "slim" || scannedProfile.SkinHash == nil || *scannedProfile.SkinHash != "skin" || scannedProfile.CapeHash != nil {
		t.Fatalf("GetProfileByID/scanProfile mismatch: %#v", scannedProfile)
	}
	if !database.IsNoRows(pgx.ErrNoRows) || database.IsNoRows(nil) {
		t.Fatalf("IsNoRows should match pgx.ErrNoRows only")
	}
	before := database.NowMS()
	after := database.NowMS()
	if before <= 0 || after < before {
		t.Fatalf("NowMS should be positive and monotonic enough: before=%d after=%d", before, after)
	}
}

func TestResetPublicSchemaRemovesDataAndRestoresDefaults(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	testutil.CreateUser(t, db, "reset-schema@test.com", "Password123", "ResetSchema", false)
	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("expected one user before reset: count=%d err=%v", count, err)
	}
	if err := db.ResetPublicSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := db.Users.Count(ctx); err != nil || count != 0 {
		t.Fatalf("reset should remove users: count=%d err=%v", count, err)
	}
	if siteName, err := db.Settings.Get(ctx, "site_name", ""); err != nil || siteName != "皮肤站" {
		t.Fatalf("reset should restore default settings: site_name=%q err=%v", siteName, err)
	}
}

func TestInitSQLContainsExpectedConstraintsAndIndexes(t *testing.T) {
	required := []string{
		"email TEXT UNIQUE NOT NULL",
		"name TEXT UNIQUE NOT NULL",
		"PRIMARY KEY(user_id, hash, texture_type)",
		"PRIMARY KEY(skin_hash, texture_type)",
		"UNIQUE(username, endpoint_id)",
		"idx_profiles_user_id",
		"idx_site_refresh_expires",
		"ON CONFLICT (key) DO NOTHING",
	}
	for _, fragment := range required {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing fragment %q", fragment)
		}
	}
}
