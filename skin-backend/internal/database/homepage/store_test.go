package homepage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/homepage"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestMigrateHomepageMediaFilesCreatesHomepageMediaOnceInFilenameOrder(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	dir := t.TempDir()
	for _, name := range []string{"zeta.webp", "notes.txt", "alpha.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.MigrateHomepageMediaFiles(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].StoragePath != "alpha.png" || items[0].SortOrder != 0 ||
		items[1].StoragePath != "zeta.webp" || items[1].SortOrder != 1 {
		t.Fatalf("migrated homepage media mismatch: %#v", items)
	}
	if err := os.WriteFile(filepath.Join(dir, "later.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateHomepageMediaFiles(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	again, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 2 {
		t.Fatalf("migration must not append when table is already initialized: %#v", again)
	}
}

func TestNextSortOrderReturnsZeroForEmptyTableAndMaxPlusOne(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := homepage.Store{Pool: db.Pool}
	next, err := store.NextSortOrder(ctx)
	if err != nil || next != 0 {
		t.Fatalf("NextSortOrder empty table mismatch: got=%d err=%v", next, err)
	}
	now := database.NowMS()
	if err := store.Create(ctx, testMedia("first", 0, now)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, testMedia("second", 1, now)); err != nil {
		t.Fatal(err)
	}
	next, err = store.NextSortOrder(ctx)
	if err != nil || next != 2 {
		t.Fatalf("NextSortOrder after two items mismatch: got=%d err=%v", next, err)
	}
}

func TestHomepageStorePatchReorderAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	now := database.NowMS()
	item := testMedia("one", 0, now)
	if err := db.HomepageMedia.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	item2 := testMedia("two", 1, now)
	if err := db.HomepageMedia.Create(context.Background(), item2); err != nil {
		t.Fatal(err)
	}
	title := "Updated"
	enabled := false
	duration := 7000
	patched, err := db.HomepageMedia.Patch(context.Background(), "one", homepagePatch(title, enabled, duration, now+1))
	if err != nil {
		t.Fatal(err)
	}
	if patched.Title != title || patched.Enabled || patched.DurationMS != duration || patched.UpdatedAt != now+1 {
		t.Fatalf("patch mismatch: %#v", patched)
	}
	if err := db.HomepageMedia.Reorder(context.Background(), []string{"two", "one"}, now+2); err != nil {
		t.Fatal(err)
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != "two" || items[0].SortOrder != 0 || items[1].ID != "one" || items[1].SortOrder != 1 {
		t.Fatalf("reorder mismatch: %#v", items)
	}
	deleted, err := db.HomepageMedia.Delete(context.Background(), "two")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.ID != "two" {
		t.Fatalf("deleted item mismatch: %#v", deleted)
	}
}

func TestHomepageStoreListGetPatchDefaultsAndReorderRollback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	now := database.NowMS()
	enabled := testMedia("enabled", 0, now)
	disabled := testMedia("disabled", 1, now)
	disabled.Enabled = false
	if err := db.HomepageMedia.Create(ctx, enabled); err != nil {
		t.Fatal(err)
	}
	if err := db.HomepageMedia.Create(ctx, disabled); err != nil {
		t.Fatal(err)
	}

	enabledOnly, err := db.HomepageMedia.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(enabledOnly) != 1 || enabledOnly[0].ID != enabled.ID || enabledOnly[0].Enabled != true {
		t.Fatalf("enabled-only list mismatch: %#v", enabledOnly)
	}
	got, err := db.HomepageMedia.Get(ctx, disabled.ID)
	if err != nil || got.ID != disabled.ID || got.StoragePath != disabled.StoragePath || got.Enabled != false {
		t.Fatalf("Get disabled media mismatch: got=%#v err=%v", got, err)
	}
	if missing, err := db.HomepageMedia.Get(ctx, "missing-homepage-media"); !errors.Is(err, pgx.ErrNoRows) || missing.ID != "" {
		t.Fatalf("missing Get should return zero media and pgx.ErrNoRows: media=%#v err=%v", missing, err)
	}

	patched, err := db.HomepageMedia.Patch(ctx, enabled.ID, homepage.Patch{UpdatedAt: now + 1})
	if err != nil {
		t.Fatal(err)
	}
	if patched.ID != enabled.ID || patched.Title != enabled.Title || patched.DurationMS != enabled.DurationMS ||
		patched.YawSpeedDPS != enabled.YawSpeedDPS || patched.UpdatedAt != now+1 {
		t.Fatalf("empty patch should only update timestamp: %#v want base=%#v updated_at=%d", patched, enabled, now+1)
	}

	err = db.HomepageMedia.Reorder(ctx, []string{enabled.ID, "missing-homepage-media"}, now+2)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reorder with missing id error=%v; want pgx.ErrNoRows", err)
	}
	items, err := db.HomepageMedia.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != enabled.ID || items[0].SortOrder != 0 ||
		items[1].ID != disabled.ID || items[1].SortOrder != 1 {
		t.Fatalf("failed reorder should roll back all sort orders: %#v", items)
	}
}

func TestHomepageStoreIncrementalPatchAndReorderPreserveEveryUnsubmittedField(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	createdAt := int64(1000)
	first := model.HomepageMedia{
		ID: "first", Type: "image", Title: "First", StoragePath: "first.png",
		OverlayOpacityLight: 0.11, OverlayOpacityDark: 0.21, StartYaw: 1, StartPitch: 2,
		YawSpeedDPS: 3, PitchSpeedDPS: 4, SortOrder: 0, Enabled: true, DurationMS: 7000,
		CreatedAt: createdAt, UpdatedAt: 1100,
	}
	second := model.HomepageMedia{
		ID: "second", Type: "panorama", Title: "Second", StoragePath: "second/panorama",
		OverlayOpacityLight: 0.12, OverlayOpacityDark: 0.22, StartYaw: 11, StartPitch: 12,
		YawSpeedDPS: 13, PitchSpeedDPS: 14, SortOrder: 1, Enabled: false, DurationMS: 8000,
		CreatedAt: createdAt + 1, UpdatedAt: 1200,
	}
	third := model.HomepageMedia{
		ID: "third", Type: "image", Title: "Third", StoragePath: "third.webp",
		OverlayOpacityLight: 0.13, OverlayOpacityDark: 0.23, StartYaw: 21, StartPitch: 22,
		YawSpeedDPS: 23, PitchSpeedDPS: 24, SortOrder: 2, Enabled: true, DurationMS: 9000,
		CreatedAt: createdAt + 2, UpdatedAt: 1300,
	}
	for _, item := range []model.HomepageMedia{first, second, third} {
		if err := db.HomepageMedia.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	updatedTitle := "Second updated"
	patchedAt := int64(2000)
	patched, err := db.HomepageMedia.Patch(ctx, second.ID, homepage.Patch{
		Title:     &updatedTitle,
		UpdatedAt: patchedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedSecond := second
	expectedSecond.Title = updatedTitle
	expectedSecond.UpdatedAt = patchedAt
	if patched != expectedSecond {
		t.Fatalf("incremental patch changed unsubmitted fields: got=%#v want=%#v", patched, expectedSecond)
	}
	for _, want := range []model.HomepageMedia{first, third} {
		got, err := db.HomepageMedia.Get(ctx, want.ID)
		if err != nil || got != want {
			t.Fatalf("patch changed unrelated media %s: got=%#v err=%v want=%#v", want.ID, got, err, want)
		}
	}

	reorderedAt := int64(3000)
	if err := db.HomepageMedia.Reorder(ctx, []string{third.ID, second.ID, first.ID}, reorderedAt); err != nil {
		t.Fatal(err)
	}
	expectedThird := third
	expectedThird.SortOrder = 0
	expectedThird.UpdatedAt = reorderedAt
	expectedSecond.SortOrder = 1
	expectedSecond.UpdatedAt = reorderedAt
	expectedFirst := first
	expectedFirst.SortOrder = 2
	expectedFirst.UpdatedAt = reorderedAt

	items, err := db.HomepageMedia.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	expected := []model.HomepageMedia{expectedThird, expectedSecond, expectedFirst}
	if len(items) != len(expected) {
		t.Fatalf("reorder media count=%d want=%d: %#v", len(items), len(expected), items)
	}
	for index := range expected {
		if items[index] != expected[index] {
			t.Fatalf("reorder item[%d] changed non-order fields: got=%#v want=%#v", index, items[index], expected[index])
		}
	}
}

func testMedia(id string, order int, now int64) model.HomepageMedia {
	return model.HomepageMedia{
		ID: id, Type: "image", Title: id, StoragePath: id + ".png",
		OverlayOpacityLight: 0.45, OverlayOpacityDark: 0.45, YawSpeedDPS: 4,
		SortOrder: order, Enabled: true, DurationMS: 6000, CreatedAt: now, UpdatedAt: now,
	}
}

func homepagePatch(title string, enabled bool, duration int, updatedAt int64) homepage.Patch {
	return homepage.Patch{Title: &title, Enabled: &enabled, DurationMS: &duration, UpdatedAt: updatedAt}
}
