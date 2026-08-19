package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestHomepageMediaUploadFailsWhenRedisInvalidateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, &homepageInvalidateFailRedis{Store: testutil.NewMemoryRedis()}, nil)

	req := multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64))
	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis fail upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("redis fail upload body mismatch: %q", rec.Body.String())
	}
	// Verify DB record was cleaned up after Redis failure.
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items after Redis invalidate failure, got %d", len(items))
	}
	// Verify file was cleaned up.
	entries, err := os.ReadDir(cfg.CarouselDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files after Redis invalidate failure, got %d", len(entries))
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("redis fail panorama upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err = db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no panorama items after Redis invalidate failure, got %#v", items)
	}
	entries, err = os.ReadDir(cfg.CarouselDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no panorama files after Redis invalidate failure, got %d", len(entries))
	}
}

func TestHomepageMediaUploadDatabaseFailureCleansFilesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
	if _, err := db.Pool.Exec(context.Background(), `ALTER TABLE homepage_media ADD CONSTRAINT reject_homepage_media_insert CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64)))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("image database failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil || len(items) != 0 {
		t.Fatalf("failed image upload must leave no rows: items=%#v err=%v", items, err)
	}
	entries, err := os.ReadDir(cfg.CarouselDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed image upload must remove file: entries=%#v err=%v", entries, err)
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("panorama database failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err = db.HomepageMedia.List(context.Background(), false)
	if err != nil || len(items) != 0 {
		t.Fatalf("failed panorama upload must leave no rows: items=%#v err=%v", items, err)
	}
	entries, err = os.ReadDir(cfg.CarouselDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed panorama upload must remove directory: entries=%#v err=%v", entries, err)
	}
}

func TestHomepageMediaRoutesReturnExactErrorsForClosedDatabaseAndFilesystemFailures(t *testing.T) {
	t.Run("closed database", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cfg.CarouselDir = t.TempDir()
		h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
		db.Close()

		req := httptest.NewRequest(http.MethodGet, "/v2/admin/homepage-media", nil)
		req = withAdminActor(req, "admin-test-user")
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("list closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("image upload closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if entries, err := os.ReadDir(cfg.CarouselDir); err != nil || len(entries) != 0 {
			t.Fatalf("image upload closed database should leave no files: entries=%d err=%v", len(entries), err)
		}

		rec = httptest.NewRecorder()
		h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("panorama upload closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if entries, err := os.ReadDir(cfg.CarouselDir); err != nil || len(entries) != 0 {
			t.Fatalf("panorama upload closed database should leave no files: entries=%d err=%v", len(entries), err)
		}

		req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/missing", strings.NewReader(`{"title":"x"}`))
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("id", "missing")
		rec = httptest.NewRecorder()
		h.PatchHomepageMedia(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"homepage_media\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
			t.Fatalf("patch closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/reorder", strings.NewReader(`{"ids":["missing"]}`))
		req = withAdminActor(req, "admin-test-user")
		rec = httptest.NewRecorder()
		h.ReorderHomepageMedia(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"homepage_media\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
			t.Fatalf("reorder closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete, "/v2/admin/homepage-media/missing", nil)
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("id", "missing")
		rec = httptest.NewRecorder()
		h.DeleteHomepageMedia(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"error\":{\"object\":\"homepage_media\",\"operation\":\"resolve\",\"reason\":\"not_found\"}}\n" {
			t.Fatalf("delete closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("carousel dir is existing file", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cfg.CarouselDir = filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(cfg.CarouselDir, []byte("blocks mkdir"), 0o644); err != nil {
			t.Fatal(err)
		}
		h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)

		rec := httptest.NewRecorder()
		h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("image mkdir failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		items, err := db.HomepageMedia.List(context.Background(), false)
		if err != nil || len(items) != 0 {
			t.Fatalf("image mkdir failure should leave no rows: items=%#v err=%v", items, err)
		}

		rec = httptest.NewRecorder()
		h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
			t.Fatalf("panorama mkdir failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		items, err = db.HomepageMedia.List(context.Background(), false)
		if err != nil || len(items) != 0 {
			t.Fatalf("panorama mkdir failure should leave no rows: items=%#v err=%v", items, err)
		}
	})
}

func TestHomepageMediaMutationRedisFailuresKeepExactPersistedSideEffects(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	healthy := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, healthy, nil)

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "first.png", pngBytes(t, 8, 8)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("first upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	first := decodeMedia(t, rec.Body.Bytes())
	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v2/admin/homepage-media/image", "file", "second.png", pngBytes(t, 8, 8)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("second upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	second := decodeMedia(t, rec.Body.Bytes())

	failing := admin.NewWithRedis(cfg, db, &homepageInvalidateFailRedis{Store: testutil.NewMemoryRedis()}, nil)
	req := httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/"+first.ID, strings.NewReader(`{"title":"Patched before cache failure"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", first.ID)
	rec = httptest.NewRecorder()
	failing.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("patch invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	row, err := db.HomepageMedia.Get(context.Background(), first.ID)
	if err != nil || row.Title != "Patched before cache failure" {
		t.Fatalf("patch should persist before invalidate failure: row=%#v err=%v", row, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v2/admin/homepage-media/reorder", strings.NewReader(`{"ids":["`+second.ID+`","`+first.ID+`"]}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	failing.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("reorder invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[0].SortOrder != 0 || items[1].ID != first.ID || items[1].SortOrder != 1 {
		t.Fatalf("reorder should persist before invalidate failure: %#v", items)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v2/admin/homepage-media/"+first.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", first.ID)
	rec = httptest.NewRecorder()
	failing.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("delete invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := db.HomepageMedia.Get(context.Background(), first.ID); err == nil {
		t.Fatal("delete should remove DB row before invalidate failure")
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, first.StoragePath)); !os.IsNotExist(err) {
		t.Fatalf("delete should remove file before invalidate failure, stat err=%v", err)
	}
}
