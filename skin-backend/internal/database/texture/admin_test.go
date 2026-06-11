package texture_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"
)

func TestAdminTextureUpdateListDeleteAndMissingSentinel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-texture-admin@test.com", "Password123", "DomainTextureAdmin", false)
	if err := store.AddToLibrary(ctx, user.ID, "domain_texture_admin_hash", "skin", "Domain Admin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdateNote(ctx, "domain_texture_admin_hash", "skin", "Domain Admin Updated"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdateModel(ctx, "domain_texture_admin_hash", "skin", "default"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdatePublic(ctx, "domain_texture_admin_hash", "skin", false); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListAll(ctx, 1, nil, "", "Domain Admin Updated", "skin")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "Domain Admin Updated" || items[0]["model"] != "default" || items[0]["is_public"] != false {
		t.Fatalf("admin list mismatch: %#v", page)
	}
	if err := store.AdminDelete(ctx, "domain_texture_admin_hash", "skin", "", false); err == nil || err.Error() != "per-user deletion requires user_id" {
		t.Fatalf("expected per-user deletion validation, got %v", err)
	}
	if err := store.AdminDelete(ctx, "missing_domain_texture", "skin", user.ID, false); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("per-user delete missing texture should return ErrNotFound, got %v", err)
	}
	if err := store.AdminDelete(ctx, "domain_texture_admin_hash", "skin", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(ctx, "domain_texture_admin_hash", "skin"); err != nil || exists {
		t.Fatalf("texture should be deleted: exists=%v err=%v", exists, err)
	}
	if err := store.AdminUpdateNote(ctx, "missing_domain_texture", "skin", "note"); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing texture should return ErrNotFound, got %v", err)
	}
	if err := store.AdminUpdateModel(ctx, "missing_domain_texture", "skin", "slim"); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing model update should return ErrNotFound, got %v", err)
	}
	if err := store.AdminUpdatePublic(ctx, "missing_domain_texture", "skin", true); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing public update should return ErrNotFound, got %v", err)
	}
}

func TestAdminTextureUpdatesAreScopedByTextureType(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-texture-scoped@test.com", "Password123", "DomainTextureScoped", false)
	if err := store.AddToLibrary(ctx, user.ID, "same_hash", "skin", "Skin Note", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddToLibrary(ctx, user.ID, "same_hash", "cape", "Cape Note", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdateNote(ctx, "same_hash", "cape", "Cape Updated"); err != nil {
		t.Fatal(err)
	}
	skinInfo, err := store.GetInfo(ctx, user.ID, "same_hash", "skin")
	if err != nil {
		t.Fatal(err)
	}
	capeInfo, err := store.GetInfo(ctx, user.ID, "same_hash", "cape")
	if err != nil {
		t.Fatal(err)
	}
	if skinInfo["note"] != "Skin Note" || capeInfo["note"] != "Cape Updated" {
		t.Fatalf("admin update should affect only selected type: skin=%#v cape=%#v", skinInfo, capeInfo)
	}
	if err := store.AdminDelete(ctx, "same_hash", "cape", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(ctx, "same_hash", "skin"); err != nil || !exists {
		t.Fatalf("force deleting cape should keep same-hash skin: exists=%v err=%v", exists, err)
	}
	if exists, err := store.Exists(ctx, "same_hash", "cape"); err != nil || exists {
		t.Fatalf("cape should be deleted only: exists=%v err=%v", exists, err)
	}
	if exists, err := store.ExistsHash(ctx, "same_hash"); err != nil || !exists {
		t.Fatalf("same hash should still exist through skin row: exists=%v err=%v", exists, err)
	}
	if exists, err := store.ExistsHash(ctx, "missing_hash"); err != nil || exists {
		t.Fatalf("missing hash should not exist: exists=%v err=%v", exists, err)
	}
}

func TestAdminPerUserDeleteUpdatesUsageCount(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "domain-texture-admin-count-owner@test.com", "Password123", "AdminCountOwner", false)
	other := testutil.CreateUser(t, db, "domain-texture-admin-count-other@test.com", "Password123", "AdminCountOther", false)
	if err := store.AddToLibrary(ctx, owner.ID, "admin_count_hash", "skin", "Admin Count", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := store.AddToWardrobe(ctx, other.ID, "admin_count_hash", "skin"); err != nil || !added {
		t.Fatalf("wardrobe add mismatch: added=%v err=%v", added, err)
	}
	if err := store.AdminDelete(ctx, "admin_count_hash", "skin", other.ID, false); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListPublic(ctx, texture.PublicListOptions{Limit: 1, Sort: texture.PublicLibrarySortMostUsed})
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "admin_count_hash" || items[0]["usage_count"] != int64(1) {
		t.Fatalf("per-user admin delete should update usage_count: %#v", page)
	}
	if ok, err := store.VerifyOwnership(ctx, other.ID, "admin_count_hash", "skin"); err != nil || ok {
		t.Fatalf("other row should be removed: ok=%v err=%v", ok, err)
	}
}

func TestAdminTextureListPaginatesWithCursor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-texture-admin-page@test.com", "Password123", "AdminPageOwner", false)
	for _, item := range []struct {
		hash string
		name string
	}{
		{"admin_page_old", "Admin Page Old"},
		{"admin_page_new", "Admin Page New"},
	} {
		if err := store.AddToLibrary(ctx, user.ID, item.hash, "skin", item.name, true, "default"); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.ListAll(ctx, 1, nil, "", "Admin Page", "skin")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := first["items"].([]map[string]any)
	nextKey := first["next_key"].(map[string]any)
	if len(firstItems) != 1 || first["has_next"] != true || nextKey["last_skin_hash"] == "" {
		t.Fatalf("first admin texture page mismatch: %#v", first)
	}
	lastCreated := nextKey["last_created_at"].(int64)
	second, err := store.ListAll(ctx, 1, &lastCreated, nextKey["last_skin_hash"].(string), "Admin Page", "skin")
	if err != nil {
		t.Fatal(err)
	}
	secondItems := second["items"].([]map[string]any)
	if len(secondItems) != 1 || secondItems[0]["hash"] == firstItems[0]["hash"] || second["has_next"] != false {
		t.Fatalf("second admin texture page should advance cursor: first=%#v second=%#v", first, second)
	}
}
