package texture_test

import (
	"context"
	"errors"
	"sync"
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
	if err := store.AdminDelete(ctx, "domain_texture_admin_hash", "skin", "", false); !errors.Is(err, texture.ErrUserIDRequired) {
		t.Fatalf("expected per-user deletion validation, got %v", err)
	}
	if err := store.AdminDelete(ctx, "missing_domain_texture", "skin", user.ID, false); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("per-user delete missing texture should return ErrNotFound, got %v", err)
	}
	if err := store.AdminDelete(ctx, "missing_domain_texture", "skin", "", true); err != nil {
		t.Fatalf("force delete missing texture should be idempotent, got %v", err)
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

func TestAdminTextureStoreMethodsReturnExactClosedPoolErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	db.Close()

	if page, err := store.ListAll(ctx, 1, nil, "", "", "skin"); page != nil || err == nil || err.Error() != "closed pool" {
		t.Fatalf("ListAll closed pool = page=%#v err=%v; want nil and closed pool", page, err)
	}
	if err := store.AdminUpdatePublic(ctx, "closed-hash", "skin", true); err == nil || err.Error() != "closed pool" {
		t.Fatalf("AdminUpdatePublic closed pool error=%v; want closed pool", err)
	}
	if err := store.AdminUpdateNote(ctx, "closed-hash", "skin", "Closed"); err == nil || err.Error() != "closed pool" {
		t.Fatalf("AdminUpdateNote closed pool error=%v; want closed pool", err)
	}
	if err := store.AdminUpdateModel(ctx, "closed-hash", "skin", "slim"); err == nil || err.Error() != "closed pool" {
		t.Fatalf("AdminUpdateModel closed pool error=%v; want closed pool", err)
	}
	if exists, err := store.Exists(ctx, "closed-hash", "skin"); exists || err == nil || err.Error() != "closed pool" {
		t.Fatalf("Exists closed pool = exists=%v err=%v; want false and closed pool", exists, err)
	}
	if exists, err := store.ExistsHash(ctx, "closed-hash"); exists || err == nil || err.Error() != "closed pool" {
		t.Fatalf("ExistsHash closed pool = exists=%v err=%v; want false and closed pool", exists, err)
	}
	if err := store.AdminDelete(ctx, "closed-hash", "skin", "closed-user", false); err == nil || err.Error() != "closed pool" {
		t.Fatalf("AdminDelete closed pool error=%v; want closed pool", err)
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

func TestConcurrentAdminPerUserDeletesKeepExactUsageCount(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "admin-delete-concurrent-owner@test.com", "Password123", "AdminDeleteConcurrentOwner", false)
	first := testutil.CreateUser(t, db, "admin-delete-concurrent-first@test.com", "Password123", "AdminDeleteConcurrentFirst", false)
	second := testutil.CreateUser(t, db, "admin-delete-concurrent-second@test.com", "Password123", "AdminDeleteConcurrentSecond", false)
	const hash = "admin_delete_concurrent_usage"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Admin Concurrent Usage", true, "default"); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{first.ID, second.ID} {
		if added, err := store.AddToWardrobe(ctx, userID, hash, "skin"); err != nil || !added {
			t.Fatalf("add wardrobe for %q = %v, %v; want true, nil", userID, added, err)
		}
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_concurrent_admin_texture_delete() RETURNS trigger AS $$
		BEGIN
			IF OLD.hash = 'admin_delete_concurrent_usage' THEN
				PERFORM pg_sleep(0.2);
			END IF;
			RETURN OLD;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_concurrent_admin_texture_delete
		BEFORE DELETE ON user_textures
		FOR EACH ROW EXECUTE FUNCTION delay_concurrent_admin_texture_delete();
	`); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, userID := range []string{first.ID, second.ID} {
		userID := userID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- store.AdminDelete(context.Background(), hash, "skin", userID, false)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent admin per-user delete failed: %v", err)
		}
	}
	for _, userID := range []string{first.ID, second.ID} {
		if owned, err := store.VerifyOwnership(ctx, userID, hash, "skin"); err != nil || owned {
			t.Fatalf("admin-deleted user %q ownership=%v err=%v; want false, nil", userID, owned, err)
		}
	}
	var usage int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT usage_count FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
	`, hash).Scan(&usage); err != nil {
		t.Fatal(err)
	}
	if usage != 1 {
		t.Fatalf("usage_count after concurrent admin deletes=%d; want exact owner-only count 1", usage)
	}
}
