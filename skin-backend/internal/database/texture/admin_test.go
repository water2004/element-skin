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
	if err := store.AdminDelete(ctx, "domain_texture_admin_hash", "skin", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(ctx, "domain_texture_admin_hash", "skin"); err != nil || exists {
		t.Fatalf("texture should be deleted: exists=%v err=%v", exists, err)
	}
	if err := store.AdminUpdateNote(ctx, "missing_domain_texture", "skin", "note"); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing texture should return ErrNotFound, got %v", err)
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
}
