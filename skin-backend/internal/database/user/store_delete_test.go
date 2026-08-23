package user_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDeleteRollsBackProfilesTokensAndTexturesWhenUserDeleteFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "domain-delete-rollback@test.com", "Password123", "DomainDeleteRollback", false)
	owner := testutil.CreateUser(t, db, "domain-delete-owner@test.com", "Password123", "DomainDeleteOwner", false)
	other := testutil.CreateUser(t, db, "domain-delete-other@test.com", "Password123", "DomainDeleteOther", false)
	profile := testutil.CreateProfile(t, db, target.ID, "domain_delete_rollback", "DomainDeleteRollbackProfile")
	if err := db.Tokens.AddRefresh(ctx, "delete_rollback_refresh", target.ID, 1000, 10); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_rollback_owned", "skin", "Owned", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, other.ID, "delete_rollback_owned", "skin"); err != nil || !added {
		t.Fatalf("add owned texture to other wardrobe: added=%v err=%v", added, err)
	}
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_rollback_shared", "skin", "Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, target.ID, "delete_rollback_shared", "skin"); err != nil || !added {
		t.Fatalf("add shared texture to target wardrobe: added=%v err=%v", added, err)
	}
	ownedClient := model.OAuthClient{
		ID: "delete-rollback-owned-client", OwnerUserID: target.ID, Name: "Delete rollback owned client",
		RedirectURI: "https://owned.example/callback", ClientType: "public", Status: "active", CreatedAt: 20, UpdatedAt: 20,
	}
	if err := db.OAuth.CreateClient(ctx, ownedClient, nil, nil); err != nil {
		t.Fatal(err)
	}
	otherClient := model.OAuthClient{
		ID: "delete-rollback-other-client", OwnerUserID: owner.ID, Name: "Delete rollback other client",
		RedirectURI: "https://other.example/callback", ClientType: "public", Status: "active", CreatedAt: 30, UpdatedAt: 30,
	}
	if err := db.OAuth.CreateClient(ctx, otherClient, nil, nil); err != nil {
		t.Fatal(err)
	}
	targetID := target.ID
	if err := db.OAuth.CreateDeviceCode(ctx, model.OAuthDeviceCode{
		DeviceCodeHash: "delete-rollback-device", UserCodeHash: "delete-rollback-user-code",
		ClientID: otherClient.ID, UserID: &targetID, Status: "approved", ExpiresAt: 10_000, CreatedAt: 40,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `CREATE TABLE user_delete_guards (user_id TEXT PRIMARY KEY REFERENCES users(id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO user_delete_guards (user_id) VALUES ($1)`, target.ID); err != nil {
		t.Fatal(err)
	}

	deleted, err := store.Delete(ctx, target.ID)
	var pgErr *pgconn.PgError
	if deleted || !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("guarded delete = %v, %#v; want false and PostgreSQL 23503", deleted, err)
	}
	if got, err := store.GetByID(ctx, target.ID); err != nil || got == nil {
		t.Fatalf("failed delete must preserve user: user=%#v err=%v", got, err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got == nil || got.UserID != target.ID {
		t.Fatalf("failed delete must preserve profile: profile=%#v err=%v", got, err)
	}
	if got, err := db.Tokens.GetRefresh(ctx, "delete_rollback_refresh"); err != nil || got == nil || got["user_id"] != target.ID {
		t.Fatalf("failed delete must preserve refresh token: token=%#v err=%v", got, err)
	}
	if got, err := db.OAuth.GetClient(ctx, ownedClient.ID); err != nil || got == nil || got.OwnerUserID != target.ID {
		t.Fatalf("failed delete must preserve owned OAuth client: client=%#v err=%v", got, err)
	}
	if got, _, err := db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, "delete-rollback-device"); err != nil || got == nil || got.UserID == nil || *got.UserID != target.ID {
		t.Fatalf("failed delete must preserve delegated device code: code=%#v err=%v", got, err)
	}
	for _, check := range []struct {
		userID string
		hash   string
	}{
		{target.ID, "delete_rollback_owned"},
		{other.ID, "delete_rollback_owned"},
		{target.ID, "delete_rollback_shared"},
	} {
		if got, err := db.Textures.GetInfo(ctx, check.userID, check.hash, "skin"); err != nil || got == nil {
			t.Fatalf("failed delete must preserve texture user=%q hash=%q: texture=%#v err=%v", check.userID, check.hash, got, err)
		}
	}
	for _, check := range []struct {
		hash  string
		usage int64
	}{
		{"delete_rollback_owned", 2},
		{"delete_rollback_shared", 2},
	} {
		var usage int64
		if err := db.Pool.QueryRow(ctx,
			`SELECT usage_count FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`,
			check.hash,
		).Scan(&usage); err != nil {
			t.Fatal(err)
		}
		if usage != check.usage {
			t.Fatalf("failed delete changed usage for %q: got=%d want=%d", check.hash, usage, check.usage)
		}
	}
}

func TestDeleteRemovesOwnedTexturesAndRecountsOnlySharedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "domain-delete-state@test.com", "Password123", "DomainDeleteState", false)
	owner := testutil.CreateUser(t, db, "domain-delete-state-owner@test.com", "Password123", "DomainDeleteStateOwner", false)
	collector := testutil.CreateUser(t, db, "domain-delete-state-collector@test.com", "Password123", "DomainDeleteStateCollector", false)

	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_state_owned", "skin", "Owned", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, collector.ID, "delete_state_owned", "skin"); err != nil || !added {
		t.Fatalf("collector add owned texture: added=%v err=%v", added, err)
	}
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_state_shared", "skin", "Shared", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, target.ID, "delete_state_shared", "skin"); err != nil || !added {
		t.Fatalf("target add shared texture: added=%v err=%v", added, err)
	}
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_state_unrelated", "cape", "Unrelated", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, collector.ID, "delete_state_unrelated", "cape"); err != nil || !added {
		t.Fatalf("collector add unrelated texture: added=%v err=%v", added, err)
	}

	deleted, err := store.Delete(ctx, target.ID)
	if err != nil || !deleted {
		t.Fatalf("Delete = %v, %v; want true, nil", deleted, err)
	}
	if exists, err := db.Textures.Exists(ctx, "delete_state_owned", "skin"); err != nil || exists {
		t.Fatalf("owned texture exists=%v err=%v; want false, nil", exists, err)
	}
	for _, userID := range []string{target.ID, collector.ID} {
		if info, err := db.Textures.GetInfo(ctx, userID, "delete_state_owned", "skin"); err != nil || info != nil {
			t.Fatalf("owned texture reference for %q=%#v err=%v; want nil, nil", userID, info, err)
		}
	}
	if info, err := db.Textures.GetInfo(ctx, target.ID, "delete_state_shared", "skin"); err != nil || info != nil {
		t.Fatalf("deleted user's shared texture reference=%#v err=%v; want nil, nil", info, err)
	}
	if info, err := db.Textures.GetInfo(ctx, owner.ID, "delete_state_shared", "skin"); err != nil || info == nil {
		t.Fatalf("shared texture owner reference=%#v err=%v; want existing row", info, err)
	}
	for _, check := range []struct {
		hash        string
		textureType string
		wantUsage   int64
	}{
		{"delete_state_shared", "skin", 1},
		{"delete_state_unrelated", "cape", 2},
	} {
		var usage int64
		if err := db.Pool.QueryRow(ctx, `
			SELECT usage_count
			FROM skin_library
			WHERE skin_hash=$1 AND texture_type=$2
		`, check.hash, check.textureType).Scan(&usage); err != nil {
			t.Fatal(err)
		}
		if usage != check.wantUsage {
			t.Fatalf("%s/%s usage_count=%d; want %d", check.hash, check.textureType, usage, check.wantUsage)
		}
	}
}

func TestDeleteMissingUserDoesNotRemoveOrphanedLibraryTexture(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	const missingUserID = "missing-texture-uploader"
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO skin_library (
			skin_hash, texture_type, is_public, uploader, model, name, created_at, usage_count
		) VALUES ('orphaned_hash', 'skin', 1, $1, 'default', 'Orphaned', 1234, 0)
	`, missingUserID); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.Delete(ctx, missingUserID)
	if err != nil || deleted {
		t.Fatalf("Delete missing user = %v, %v; want false, nil", deleted, err)
	}
	var uploader, name string
	var createdAt, usageCount int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT uploader, name, created_at, usage_count
		FROM skin_library
		WHERE skin_hash='orphaned_hash' AND texture_type='skin'
	`).Scan(&uploader, &name, &createdAt, &usageCount); err != nil {
		t.Fatalf("orphaned library row should remain: %v", err)
	}
	if uploader != missingUserID || name != "Orphaned" || createdAt != 1234 || usageCount != 0 {
		t.Fatalf("orphaned library row changed: uploader=%q name=%q created_at=%d usage_count=%d",
			uploader, name, createdAt, usageCount)
	}
}
