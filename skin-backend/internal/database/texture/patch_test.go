package texture_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"
)

func TestUpdateForUserPatchesExactFieldsAndAppliedProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "texture-patch-owner@test.com", "Password123", "TexturePatchOwner", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "texture_patch_profile", "TexturePatchProfile")
	const hash = "texture_patch_hash"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Original Note", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, ptr(hash)); err != nil {
		t.Fatal(err)
	}
	note := "Note Only"
	if err := store.UpdateForUser(ctx, owner.ID, hash, "skin", texture.Patch{Note: &note}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, hash, "skin", note, "default", 1)
	model := "slim"
	isPublic := false
	if err := store.UpdateForUser(ctx, owner.ID, hash, "skin", texture.Patch{
		Model:    &model,
		IsPublic: &isPublic,
	}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, hash, "skin", note, model, 0)
	gotProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || gotProfile == nil || gotProfile.TextureModel != model {
		t.Fatalf("applied profile model = %#v, %v; want %q", gotProfile, err, model)
	}
	if err := store.UpdateForUser(ctx, owner.ID, "missing_hash", "skin", texture.Patch{Note: &note}); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing texture error = %v; want ErrNotFound", err)
	}
}

func TestTextureStoreSingleFieldPatchesPreserveAllOtherFieldsAndRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "texture-single-owner@test.com", "Password123", "TextureSingleOwner", false)
	collector := testutil.CreateUser(t, db, "texture-single-collector@test.com", "Password123", "TextureSingleCollector", false)

	for _, seed := range []struct {
		hash  string
		note  string
		model string
	}{
		{hash: "texture_single_note", note: "Original Note", model: "slim"},
		{hash: "texture_single_model", note: "Original Model", model: "default"},
		{hash: "texture_single_public", note: "Original Public", model: "slim"},
		{hash: "texture_single_untouched", note: "Untouched", model: "default"},
		{hash: "texture_admin_single_note", note: "Admin Original Note", model: "slim"},
		{hash: "texture_admin_single_model", note: "Admin Original Model", model: "default"},
		{hash: "texture_admin_single_public", note: "Admin Original Public", model: "slim"},
	} {
		if err := store.AddToLibrary(ctx, owner.ID, seed.hash, "skin", seed.note, true, seed.model); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(seed.hash, "texture_admin_") {
			if added, err := store.AddToWardrobe(ctx, collector.ID, seed.hash, "skin"); err != nil || !added {
				t.Fatalf("seed collector wardrobe %s: added=%v err=%v", seed.hash, added, err)
			}
		}
	}

	updatedNote := "Updated Note Only"
	if err := store.UpdateForUser(ctx, owner.ID, "texture_single_note", "skin", texture.Patch{Note: &updatedNote}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, "texture_single_note", "skin", updatedNote, "slim", 1)

	updatedModel := "slim"
	if err := store.UpdateForUser(ctx, owner.ID, "texture_single_model", "skin", texture.Patch{Model: &updatedModel}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, "texture_single_model", "skin", "Original Model", updatedModel, 1)

	private := false
	if err := store.UpdateForUser(ctx, owner.ID, "texture_single_public", "skin", texture.Patch{IsPublic: &private}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, "texture_single_public", "skin", "Original Public", "slim", 0)
	assertTextureState(t, store, owner.ID, "texture_single_untouched", "skin", "Untouched", "default", 1)

	adminNote := "Admin Updated Note Only"
	if err := store.AdminPatch(ctx, "texture_admin_single_note", "skin", texture.Patch{Note: &adminNote}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, "texture_admin_single_note", "skin", adminNote, "slim", 1)
	assertTextureState(t, store, collector.ID, "texture_admin_single_note", "skin", adminNote, "slim", 2)

	adminModel := "slim"
	if err := store.AdminPatch(ctx, "texture_admin_single_model", "skin", texture.Patch{Model: &adminModel}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, "texture_admin_single_model", "skin", "Admin Original Model", adminModel, 1)
	assertTextureState(t, store, collector.ID, "texture_admin_single_model", "skin", "Admin Original Model", adminModel, 2)

	if err := store.AdminPatch(ctx, "texture_admin_single_public", "skin", texture.Patch{IsPublic: &private}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, "texture_admin_single_public", "skin", "Admin Original Public", "slim", 0)
	assertTextureState(t, store, collector.ID, "texture_admin_single_public", "skin", "Admin Original Public", "slim", 2)
}

func TestAdminPatchUpdatesOwnerAndWardrobeWithoutChangingWardrobeMarker(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "texture-admin-patch-owner@test.com", "Password123", "TextureAdminPatchOwner", false)
	collector := testutil.CreateUser(t, db, "texture-admin-patch-collector@test.com", "Password123", "TextureAdminPatchCollector", false)
	ownerProfile := testutil.CreateProfile(t, db, owner.ID, "texture_admin_owner_profile", "TextureAdminOwnerProfile")
	collectorProfile := testutil.CreateProfile(t, db, collector.ID, "texture_admin_collector_profile", "TextureAdminCollectorProfile")
	const hash = "texture_admin_patch_hash"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := store.AddToWardrobe(ctx, collector.ID, hash, "skin"); err != nil || !added {
		t.Fatalf("AddToWardrobe = %v, %v; want true, nil", added, err)
	}
	if err := db.Profiles.UpdateSkin(ctx, ownerProfile.ID, ptr(hash)); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(ctx, collectorProfile.ID, ptr(hash)); err != nil {
		t.Fatal(err)
	}
	note := "Admin Updated"
	model := "slim"
	isPublic := false
	if err := store.AdminPatch(ctx, hash, "skin", texture.Patch{
		Note:     &note,
		Model:    &model,
		IsPublic: &isPublic,
	}); err != nil {
		t.Fatal(err)
	}
	assertTextureState(t, store, owner.ID, hash, "skin", note, model, 0)
	collectorInfo, err := store.GetInfo(ctx, collector.ID, hash, "skin")
	if err != nil || collectorInfo == nil ||
		collectorInfo["note"] != note ||
		collectorInfo["model"] != model ||
		collectorInfo["is_public"] != 2 {
		t.Fatalf("collector texture = %#v, %v; want note=%q model=%q is_public=2", collectorInfo, err, note, model)
	}
	for _, profileID := range []string{ownerProfile.ID, collectorProfile.ID} {
		got, err := db.Profiles.GetByID(ctx, profileID)
		if err != nil || got == nil || got.TextureModel != model {
			t.Fatalf("profile %q = %#v, %v; want model=%q", profileID, got, err, model)
		}
	}
	if err := store.AdminPatch(ctx, "missing_hash", "skin", texture.Patch{Note: &note}); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing library texture error = %v; want ErrNotFound", err)
	}
}

func TestUpdateForUserReturnsNotFoundAfterConcurrentLibraryDelete(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "texture-patch-delete-race@test.com", "Password123", "TexturePatchDeleteRace", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "texture_patch_delete_race_profile", "TexturePatchRace")
	const hash = "texture_patch_delete_race"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, ptr(hash)); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `
		SELECT 1, pg_backend_pid()
		FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
		FOR UPDATE
	`, hash).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	modelName := "slim"
	result := make(chan error, 1)
	go func() {
		result <- store.UpdateForUser(
			context.Background(),
			owner.ID,
			hash,
			"skin",
			texture.Patch{Model: &modelName},
		)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-result:
			t.Fatalf("texture update completed before delete lock was released: %v", err)
		default:
		}
		var waiting bool
		if err := db.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("texture update did not reach the expected library row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := <-result; !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("update after concurrent library delete error=%v; want ErrNotFound", err)
	}
	if info, err := store.GetInfo(ctx, owner.ID, hash, "skin"); err != nil || info != nil {
		t.Fatalf("concurrent delete should leave no personal texture: info=%#v err=%v", info, err)
	}
	gotProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || gotProfile == nil || gotProfile.TextureModel != "default" {
		t.Fatalf("failed concurrent update changed profile model: profile=%#v err=%v", gotProfile, err)
	}
}

func assertTextureState(t *testing.T, db interface {
	GetInfo(context.Context, string, string, string) (map[string]any, error)
}, userID, hash, textureType, note, model string, isPublic int) {
	t.Helper()
	info, err := db.GetInfo(context.Background(), userID, hash, textureType)
	if err != nil || info == nil ||
		info["note"] != note ||
		info["model"] != model ||
		info["is_public"] != isPublic {
		t.Fatalf("user texture = %#v, %v; want note=%q model=%q is_public=%d", info, err, note, model, isPublic)
	}
}

func ptr(value string) *string {
	return &value
}
