package texture_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestTextureLibraryCursorsAndDisabledPublicLibraryExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	if _, err := svc.PublicLibrary(ctx, permission.Actor{}, "", 10, "skin", "", "latest"); !httpErrorIs(err, 403, "permission.check.denied") {
		t.Fatalf("PublicLibrary without public permission mismatch: %#v", err)
	}
	user := testutil.CreateUser(t, db, "site-profile-cursor@test.com", "Password123", "ProfileCursor", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "profile_cursor_skin", "skin", "Profile Cursor Skin", true, "default"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListMyTextures(ctx, textureUserActor(user.ID), "not-base64", 10, "skin"); !httpErrorIs(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("invalid texture cursor should reject exactly, got %#v", err)
	}
	if _, err := svc.PublicLibrary(ctx, texturePublicActor(), "not-base64", 10, "skin", "", "latest"); !httpErrorIs(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("invalid public library cursor should reject exactly, got %#v", err)
	}

	if err := db.Settings.Set(ctx, "enable_skin_library", "false"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicLibrary(ctx, texturePublicActor(), "", 10, "skin", "", "latest"); !httpErrorIs(err, 403, "texture_library.read.disabled") {
		t.Fatalf("disabled public library should reject exactly, got %#v", err)
	}
}

func TestPublicLibraryPropagatesSettingsCacheErrorExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("settings cache unavailable")
	svc := texturesvc.LibraryService{DB: db, Settings: settingssvc.Settings{DB: db, Redis: cache}}

	result, err := svc.PublicLibrary(ctx, texturePublicActor(), "", 10, "skin", "", "latest")
	if result != nil || !errors.Is(err, cache.Err) {
		t.Fatalf("PublicLibrary settings dependency error result=%#v err=%v want nil %v", result, err, cache.Err)
	}
}

func TestTextureListsAndPublicLibraryCursorsAdvanceExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-profile-cursor-owner@test.com", "Password123", "ProfileCursorOwner", false)
	other := testutil.CreateUser(t, db, "site-profile-cursor-other@test.com", "Password123", "ProfileCursorOther", false)
	for _, item := range []struct {
		hash string
		name string
	}{
		{"profile_cursor_old", "Profile Cursor Old"},
		{"profile_cursor_new", "Profile Cursor New"},
	} {
		if err := db.Textures.AddToLibrary(ctx, owner.ID, item.hash, "skin", item.name, true, "default"); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "profile_cursor_old", "skin"); err != nil {
		t.Fatal(err)
	}

	firstPage, err := svc.ListMyTextures(ctx, textureUserActor(owner.ID), "", 1, "skin")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]map[string]any)
	cursor, _ := firstPage["next_cursor"].(string)
	if len(firstItems) != 1 || cursor == "" {
		t.Fatalf("ListMyTextures first page should include one item and next cursor: %#v", firstPage)
	}
	secondPage, err := svc.ListMyTextures(ctx, textureUserActor(owner.ID), cursor, 10, "skin")
	if err != nil {
		t.Fatal(err)
	}
	if secondItems := secondPage["items"].([]map[string]any); len(secondItems) != 1 || secondItems[0]["hash"] == firstItems[0]["hash"] {
		t.Fatalf("ListMyTextures cursor should advance to next item: first=%#v second=%#v", firstPage, secondPage)
	}

	public, err := svc.PublicLibrary(ctx, texturePublicActor(), "", 1, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	publicItems := public["items"].([]map[string]any)
	publicCursor, _ := public["next_cursor"].(string)
	if len(publicItems) != 1 || publicCursor == "" || publicItems[0]["usage_count"] != int64(2) {
		t.Fatalf("most_used public library first page mismatch: %#v", public)
	}
	nextPublic, err := svc.PublicLibrary(ctx, texturePublicActor(), publicCursor, 10, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	nextItems := nextPublic["items"].([]map[string]any)
	if len(nextItems) != 1 || nextItems[0]["hash"] == publicItems[0]["hash"] {
		t.Fatalf("most_used public library cursor should advance exactly: first=%#v next=%#v", public, nextPublic)
	}
}

func TestPublicLibraryRejectsIncompleteAndCrossSortCursors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)

	for _, tc := range []struct {
		name   string
		cursor string
		sort   string
	}{
		{
			name: "latest cursor missing hash",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": int64(1234),
			}),
			sort: "latest",
		},
		{
			name: "latest cursor reused for most used",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": int64(1234),
				"last_skin_hash":  "cursor_hash",
			}),
			sort: "most_used",
		},
		{
			name: "fractional timestamp",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": 1.5,
				"last_skin_hash":  "cursor_hash",
			}),
			sort: "latest",
		},
		{
			name: "negative timestamp",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": -1,
				"last_skin_hash":  "cursor_hash",
			}),
			sort: "latest",
		},
		{
			name: "fractional usage",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at":  1,
				"last_skin_hash":   "cursor_hash",
				"last_usage_count": 2.5,
			}),
			sort: "most_used",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.PublicLibrary(ctx, texturePublicActor(), tc.cursor, 10, "skin", "", tc.sort)
			if result != nil || !httpErrorIs(err, 400, "pagination_cursor.decode.invalid") {
				t.Fatalf("PublicLibrary result=%#v err=%#v; want nil and exact invalid cursor", result, err)
			}
		})
	}
}

func TestPrivateTextureListsRejectIncompleteCursors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "private-cursor@test.com", "Password123", "PrivateCursor", false)

	textureResult, err := svc.ListMyTextures(ctx, textureUserActor(user.ID), util.EncodeCursor(map[string]any{
		"last_created_at": int64(1234),
	}), 10, "skin")
	if textureResult != nil || !httpErrorIs(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListMyTextures result=%#v err=%#v; want nil and exact invalid cursor", textureResult, err)
	}

	textureResult, err = svc.ListMyTextures(ctx, textureUserActor(user.ID), util.EncodeCursor(map[string]any{
		"last_created_at": 1.5,
		"last_hash":       "cursor_hash",
	}), 10, "skin")
	if textureResult != nil || !httpErrorIs(err, 400, "pagination_cursor.decode.invalid") {
		t.Fatalf("ListMyTextures fractional cursor result=%#v err=%#v; want nil and exact invalid cursor", textureResult, err)
	}
}
