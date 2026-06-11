package texture_test

import (
	"context"
	"errors"
	"testing"

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
