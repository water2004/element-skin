package texture_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTextureLibraryWardrobeAndPatchParsingExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-wardrobe-owner@test.com", "Password123", "WardrobeOwner", false)
	collector := testutil.CreateUser(t, db, "site-textures-wardrobe-collector@test.com", "Password123", "WardrobeCollector", false)
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_wardrobe_skin", "skin", "Wardrobe Skin", true, "default"); err != nil {
		t.Fatal(err)
	}

	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(collector.ID), "missing_wardrobe_skin", "skin"); !httpErrorIs(err, 404, "Texture not found in library") {
		t.Fatalf("missing wardrobe add mismatch: %#v", err)
	}
	if count, err := db.Textures.CountForUser(ctx, collector.ID); err != nil || count != 0 {
		t.Fatalf("missing wardrobe add must not create user rows: count=%d err=%v", count, err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(collector.ID), "texture_service_wardrobe_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateTexture(ctx, textureUserActor(collector.ID), "texture_service_wardrobe_skin", "skin", map[string]any{
		"model": "slim",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["model"] != "slim" || updated["is_public"] != 2 {
		t.Fatalf("wardrobe model patch mismatch: %#v", updated)
	}
	updated, err = svc.UpdateTexture(ctx, textureUserActor(owner.ID), "texture_service_wardrobe_skin", "skin", map[string]any{
		"is_public": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["model"] != "default" || updated["is_public"] != 1 {
		t.Fatalf("integer public patch mismatch: %#v", updated)
	}
	updated, err = svc.UpdateTexture(ctx, textureUserActor(owner.ID), "texture_service_wardrobe_skin", "skin", map[string]any{
		"is_public": float64(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["model"] != "default" || updated["is_public"] != 0 {
		t.Fatalf("float public patch mismatch: %#v", updated)
	}
	ownerInfo, err := db.Textures.GetInfo(ctx, owner.ID, "texture_service_wardrobe_skin", "skin")
	if err != nil || ownerInfo == nil || ownerInfo["model"] != "default" || ownerInfo["is_public"] != 0 {
		t.Fatalf("wardrobe patch must not mutate owner row: info=%#v err=%v", ownerInfo, err)
	}
}
