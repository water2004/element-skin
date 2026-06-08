package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTextureLibraryStorePublicListAndWardrobeExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "texture-library-owner@test.com", "Password123", "TextureLibraryOwner", false)
	other := testutil.CreateUser(t, db, "texture-library-other@test.com", "Password123", "TextureLibraryOther", false)
	if err := db.AddTextureToLibrary(ctx, owner.ID, "texture_library_hash", "skin", "Library Texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	public, err := db.ListPublicLibrary(ctx, 1, "skin", "Library", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "texture_library_hash" || items[0]["uploader_display_name"] != "TextureLibraryOwner" || public["has_next"] != false {
		t.Fatalf("public library mismatch: %#v", public)
	}
	added, err := db.AddTextureToWardrobe(ctx, other.ID, "texture_library_hash")
	if err != nil || !added {
		t.Fatalf("wardrobe add mismatch: added=%v err=%v", added, err)
	}
	info, err := db.GetTextureInfo(ctx, other.ID, "texture_library_hash", "skin")
	if err != nil || info["note"] != "Library Texture" || info["is_public"] != 2 {
		t.Fatalf("wardrobe texture mismatch: info=%#v err=%v", info, err)
	}
}
