package texture_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"
)

func TestPublicLibraryAndWardrobeCopyVisibilityRules(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "domain-texture-library-owner@test.com", "Password123", "DomainTextureLibraryOwner", false)
	other := testutil.CreateUser(t, db, "domain-texture-library-other@test.com", "Password123", "DomainTextureLibraryOther", false)
	if err := store.AddToLibrary(ctx, owner.ID, "domain_texture_library_hash", "skin", "Domain Library", true, "default"); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListPublic(ctx, 1, "skin", "Domain Library", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "domain_texture_library_hash" || items[0]["uploader_display_name"] != "DomainTextureLibraryOwner" {
		t.Fatalf("public library mismatch: %#v", page)
	}
	added, err := store.AddToWardrobe(ctx, other.ID, "domain_texture_library_hash")
	if err != nil || !added {
		t.Fatalf("wardrobe add mismatch: added=%v err=%v", added, err)
	}
	info, err := store.GetInfo(ctx, other.ID, "domain_texture_library_hash", "skin")
	if err != nil || info["note"] != "Domain Library" || info["is_public"] != 2 {
		t.Fatalf("wardrobe copy mismatch: info=%#v err=%v", info, err)
	}
	if err := store.AddToLibrary(ctx, owner.ID, "domain_private_library_hash", "skin", "Private Library", false, "default"); err != nil {
		t.Fatal(err)
	}
	added, err = store.AddToWardrobe(ctx, other.ID, "domain_private_library_hash")
	if err != nil || added {
		t.Fatalf("private library texture should not be wardrobe-addable: added=%v err=%v", added, err)
	}
}
