package site_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestProfilesCreateListAndClearTextureExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profiles-service@test.com", "Password123", "SiteProfilesService", false)
	created, err := svc.CreateProfile(ctx, testUserActor(user.ID), "ProfileSvc", "slim")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "ProfileSvc" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	list, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "ProfileSvc" || list["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", list)
	}
}

func TestProfilesRejectInvalidProfileAndLibraryInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profile-invalid@test.com", "Password123", "ProfileInvalid", false)
	other := testutil.CreateUser(t, db, "site-profile-invalid-other@test.com", "Password123", "ProfileInvalidOther", false)
	existing := testutil.CreateProfile(t, db, user.ID, "site_profile_invalid_existing", "ExistingSvc")
	target := testutil.CreateProfile(t, db, user.ID, "site_profile_invalid_target", "TargetSvc")
	foreign := testutil.CreateProfile(t, db, other.ID, "site_profile_invalid_foreign", "ForeignSvc")

	for _, tc := range []struct {
		name string
		call func() error
		code int
		want string
	}{
		{"empty create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), "", "default")
			return err
		}, 400, "name required"},
		{"invalid create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), "bad-name!", "default")
			return err
		}, 400, "角色名只能包含字母、数字、下划线，长度1-16字符"},
		{"duplicate create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), existing.Name, "default")
			return err
		}, 400, "角色名已被占用，请换一个名称"},
		{"empty update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, "")
		}, 400, "name required"},
		{"invalid update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, "bad-name!")
		}, 400, "角色名只能包含字母、数字、下划线，长度1-16字符"},
		{"duplicate update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, existing.Name)
		}, 400, "角色名已被占用"},
		{"foreign update", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), foreign.ID, "StolenProfileSvc")
		}, 403, "not allowed"},
		{"missing update", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), "missing-profile", "MissingProfileSvc")
		}, 404, "profile not found"},
		{"foreign delete", func() error {
			return svc.DeleteProfile(ctx, testUserActor(user.ID), foreign.ID)
		}, 403, "not allowed"},
		{"missing delete", func() error {
			return svc.DeleteProfile(ctx, testUserActor(user.ID), "missing-profile")
		}, 404, "profile not found"},
		{"invalid clear texture type", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), target.ID, "elytra")
		}, 400, "Invalid texture_type"},
		{"foreign clear texture", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), foreign.ID, "skin")
		}, 403, "not allowed"},
		{"missing clear texture", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), "missing-profile", "skin")
		}, 404, "profile not found"},
		{"missing wardrobe add", func() error {
			return svc.AddTextureToWardrobe(ctx, testUserActor(user.ID), "missing_texture_hash", "skin")
		}, 404, "Texture not found in library"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !httpError(err, tc.code, tc.want) {
				t.Fatalf("%s should reject exactly, got %#v", tc.name, err)
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(ctx, target.ID)
	if err != nil || unchanged == nil || unchanged.Name != target.Name {
		t.Fatalf("invalid profile mutations should not change target: profile=%#v err=%v", unchanged, err)
	}
	foreignAfter, err := db.Profiles.GetByID(ctx, foreign.ID)
	if err != nil || foreignAfter == nil || foreignAfter.Name != foreign.Name {
		t.Fatalf("foreign profile should remain unchanged: profile=%#v err=%v", foreignAfter, err)
	}
}

func TestConcurrentProfileNameWritesReturnExactBusinessConflict(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "profile-name-race@test.com", "Password123", "ProfileNameRace", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_profile_name_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_profile_name_insert
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_profile_name_write();
	`); err != nil {
		t.Fatal(err)
	}

	createErrors := runConcurrentProfileWrites(2, func() error {
		_, err := svc.CreateProfile(context.Background(), testUserActor(user.ID), "ConcurrentCreate", "default")
		return err
	})
	assertOneProfileWriteConflict(t, createErrors, "角色名已被占用，请换一个名称")
	var createCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE name='ConcurrentCreate'`).Scan(&createCount); err != nil {
		t.Fatal(err)
	}
	if createCount != 1 {
		t.Fatalf("concurrent create stored %d target names; want exactly 1", createCount)
	}

	if _, err := db.Pool.Exec(ctx, `
		DROP TRIGGER delay_profile_name_insert ON profiles;
		CREATE TRIGGER delay_profile_name_update
		BEFORE UPDATE OF name ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_profile_name_write();
	`); err != nil {
		t.Fatal(err)
	}
	first := testutil.CreateProfile(t, db, user.ID, "profile_name_race_first", "RaceFirst")
	second := testutil.CreateProfile(t, db, user.ID, "profile_name_race_second", "RaceSecond")
	profileIDs := []string{first.ID, second.ID}
	var index int
	var mu sync.Mutex
	renameErrors := runConcurrentProfileWrites(2, func() error {
		mu.Lock()
		profileID := profileIDs[index]
		index++
		mu.Unlock()
		return svc.UpdateProfile(context.Background(), testUserActor(user.ID), profileID, "ConcurrentRename")
	})
	assertOneProfileWriteConflict(t, renameErrors, "角色名已被占用")
	var renamedCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE name='ConcurrentRename'),
			COUNT(*) FILTER (WHERE name IN ('RaceFirst','RaceSecond'))
		FROM profiles
		WHERE id = ANY($1)
	`, profileIDs).Scan(&renamedCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if renamedCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent rename state: renamed=%d original=%d; want 1 and 1", renamedCount, originalCount)
	}
}

func TestCreateProfileMapsDatabaseIDConflictExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "profile-id-conflict@test.com", "Password123", "ProfileIDConflict", false)
	existing := testutil.CreateProfile(t, db, user.ID, "forced_profile_id_conflict", "ExistingID")
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION force_profile_id_conflict() RETURNS trigger AS $$
		BEGIN
			IF NEW.name = 'ForcedUUID' THEN
				NEW.id := 'forced_profile_id_conflict';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER force_profile_id_conflict
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION force_profile_id_conflict();
	`); err != nil {
		t.Fatal(err)
	}

	result, err := svc.CreateProfile(ctx, testUserActor(user.ID), "ForcedUUID", "slim")
	if result != nil || !httpError(err, 400, "角色 UUID 冲突，无法新建角色") {
		t.Fatalf("forced profile ID conflict result=%#v err=%#v; want nil and exact 400", result, err)
	}
	if stored, err := db.Profiles.GetByName(ctx, "ForcedUUID"); err != nil || stored != nil {
		t.Fatalf("forced UUID conflict persisted target name: profile=%#v err=%v", stored, err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, existing.ID)
	if err != nil || unchanged == nil || unchanged.Name != "ExistingID" || unchanged.TextureModel != "default" {
		t.Fatalf("forced UUID conflict changed existing profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestUpdateProfileReturnsNotFoundWhenProfileIsDeletedAfterRead(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "profile-update-delete-race@test.com", "Password123", "ProfileUpdateDeleteRace", false)
	target := testutil.CreateProfile(t, db, user.ID, "profile_update_delete_race", "DeleteRace")

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `SELECT 1, pg_backend_pid() FROM profiles WHERE id=$1 FOR UPDATE`, target.ID).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- svc.UpdateProfile(context.Background(), testUserActor(user.ID), target.ID, target.Name)
	}()
	waitForBlockedDatabaseOperation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !httpError(err, 404, "profile not found") {
		t.Fatalf("profile deleted after read should return exact not found error, got %#v", err)
	}
}

func TestClearProfileTextureReturnsNotFoundWhenProfileIsDeletedAfterRead(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "profile-clear-delete-race@test.com", "Password123", "ProfileClearRace", false)
	target := testutil.CreateProfile(t, db, user.ID, "profile_clear_delete_race", "ClearRace")
	skin := "clear_race_skin"
	if err := db.Profiles.UpdateSkin(ctx, target.ID, &skin); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `SELECT 1, pg_backend_pid() FROM profiles WHERE id=$1 FOR UPDATE`, target.ID).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- svc.ClearProfileTexture(context.Background(), testUserActor(user.ID), target.ID, "skin")
	}()
	waitForBlockedDatabaseOperation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !httpError(err, 404, "profile not found") {
		t.Fatalf("profile deleted before texture update should return exact not found error, got %#v", err)
	}
}

func waitForBlockedDatabaseOperation(t *testing.T, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, lockHolderPID int, result <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-result:
			t.Fatalf("database operation completed before row-lock release: %#v", err)
		default:
		}
		var waiting bool
		if err := db.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("database operation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func runConcurrentProfileWrites(count int, write func() error) []error {
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- write()
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	out := make([]error, 0, count)
	for err := range results {
		out = append(out, err)
	}
	return out
}

func assertOneProfileWriteConflict(t *testing.T, results []error, detail string) {
	t.Helper()
	successes := 0
	conflicts := 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case httpError(err, 400, detail):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent profile result: %#v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent profile writes: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
}

func TestProfilesCursorsDisabledLibraryAndAdminDeleteByID(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profile-cursor@test.com", "Password123", "ProfileCursor", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_profile_admin_delete", "AdminDeleteProfileSvc")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "profile_cursor_skin", "skin", "Profile Cursor Skin", true, "default"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), "not-base64", 10); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid profile cursor should reject exactly, got %#v", err)
	}
	if _, err := svc.ListMyTextures(ctx, testUserActor(user.ID), "not-base64", 10, "skin"); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid texture cursor should reject exactly, got %#v", err)
	}
	if _, err := svc.PublicLibrary(ctx, "not-base64", 10, "skin", "", "latest"); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid public library cursor should reject exactly, got %#v", err)
	}

	if err := db.Settings.Set(ctx, "enable_skin_library", "false"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicLibrary(ctx, "", 10, "skin", "", "latest"); !httpError(err, 403, "Skin library is disabled by administrator") {
		t.Fatalf("disabled public library should reject exactly, got %#v", err)
	}

	if err := svc.DeleteProfileByID(ctx, profile.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got != nil {
		t.Fatalf("DeleteProfileByID should remove profile regardless of owner: profile=%#v err=%v", got, err)
	}
	if err := svc.DeleteProfileByID(ctx, profile.ID); !httpError(err, 404, "profile not found") {
		t.Fatalf("DeleteProfileByID missing profile should reject exactly, got %#v", err)
	}
}

func TestProfilesListTexturesParsesCursorAndPublicLibraryMostUsedCursor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
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
	if err := svc.AddTextureToWardrobe(ctx, testUserActor(other.ID), "profile_cursor_old", "skin"); err != nil {
		t.Fatal(err)
	}

	firstPage, err := svc.ListMyTextures(ctx, testUserActor(owner.ID), "", 1, "skin")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]map[string]any)
	cursor, _ := firstPage["next_cursor"].(string)
	if len(firstItems) != 1 || cursor == "" {
		t.Fatalf("ListMyTextures first page should include one item and next cursor: %#v", firstPage)
	}
	secondPage, err := svc.ListMyTextures(ctx, testUserActor(owner.ID), cursor, 10, "skin")
	if err != nil {
		t.Fatal(err)
	}
	if secondItems := secondPage["items"].([]map[string]any); len(secondItems) != 1 || secondItems[0]["hash"] == firstItems[0]["hash"] {
		t.Fatalf("ListMyTextures cursor should advance to next item: first=%#v second=%#v", firstPage, secondPage)
	}

	public, err := svc.PublicLibrary(ctx, "", 1, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	publicItems := public["items"].([]map[string]any)
	publicCursor, _ := public["next_cursor"].(string)
	if len(publicItems) != 1 || publicCursor == "" || publicItems[0]["usage_count"] != int64(2) {
		t.Fatalf("most_used public library first page mismatch: %#v", public)
	}
	nextPublic, err := svc.PublicLibrary(ctx, publicCursor, 10, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	nextItems := nextPublic["items"].([]map[string]any)
	if len(nextItems) != 1 || nextItems[0]["hash"] == publicItems[0]["hash"] {
		t.Fatalf("most_used public library cursor should advance exactly: first=%#v next=%#v", public, nextPublic)
	}
}

func TestDeleteUserRecountsSharedLibraryButDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-profile-delete-owner@test.com", "Password123", "ProfileDeleteOwner", false)
	target := testutil.CreateUser(t, db, "site-profile-delete-target@test.com", "Password123", "ProfileDeleteTarget", false)
	other := testutil.CreateUser(t, db, "site-profile-delete-other@test.com", "Password123", "ProfileDeleteOther", false)

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, testUserActor(target.ID), "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, testUserActor(other.ID), "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, testUserActor(other.ID), "delete_user_uploaded_skin", "skin"); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.DeleteUser(ctx, testActorWithCodes("admin-delete-user", "account.delete.any"), target.ID)
	if err != nil || !ok {
		t.Fatalf("DeleteUser returned ok=%v err=%v", ok, err)
	}
	assertServicePublicUsage(t, svc, "delete_user_shared_skin", int64(2))
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}

func TestPublicLibraryRejectsIncompleteAndCrossSortCursors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())

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
			result, err := svc.PublicLibrary(ctx, tc.cursor, 10, "skin", "", tc.sort)
			if result != nil || !httpError(err, 400, "Invalid cursor") {
				t.Fatalf("PublicLibrary result=%#v err=%#v; want nil and exact invalid cursor", result, err)
			}
		})
	}
}

func TestPrivateListsRejectIncompleteCursors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "private-cursor@test.com", "Password123", "PrivateCursor", false)

	profileResult, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), util.EncodeCursor(map[string]any{
		"unexpected": "value",
	}), 10)
	if profileResult != nil || !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("ListMyProfiles result=%#v err=%#v; want nil and exact invalid cursor", profileResult, err)
	}

	textureResult, err := svc.ListMyTextures(ctx, testUserActor(user.ID), util.EncodeCursor(map[string]any{
		"last_created_at": int64(1234),
	}), 10, "skin")
	if textureResult != nil || !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("ListMyTextures result=%#v err=%#v; want nil and exact invalid cursor", textureResult, err)
	}

	textureResult, err = svc.ListMyTextures(ctx, testUserActor(user.ID), util.EncodeCursor(map[string]any{
		"last_created_at": 1.5,
		"last_hash":       "cursor_hash",
	}), 10, "skin")
	if textureResult != nil || !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("ListMyTextures fractional cursor result=%#v err=%#v; want nil and exact invalid cursor", textureResult, err)
	}
}

func TestSetProfileTextureSkipsExactNoOpWrites(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "profile-noop@test.com", "Password123", "ProfileNoop", false)
	profile := testutil.CreateProfile(t, db, user.ID, "profile_noop_values", "ProfileNoopValues")
	empty := testutil.CreateProfile(t, db, user.ID, "profile_noop_empty", "ProfileNoopEmpty")
	adminActor := testActorWithCodes("profile-texture-admin", "profile.update.any")
	skin := "same_skin_hash"
	cape := "same_cape_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatalf("seed skin: %v", err)
	}
	if err := db.Profiles.UpdateCape(ctx, profile.ID, &cape); err != nil {
		t.Fatalf("seed cape: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_profile_noop_updates() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'profile update should not run' USING ERRCODE='23514';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_profile_noop_updates
		BEFORE UPDATE ON profiles
		FOR EACH ROW
		WHEN (OLD.id IN ('profile_noop_values', 'profile_noop_empty'))
		EXECUTE FUNCTION reject_profile_noop_updates();
	`); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		profileID   string
		textureType string
		hash        *string
	}{
		{name: "same skin", profileID: profile.ID, textureType: "skin", hash: &skin},
		{name: "same cape", profileID: profile.ID, textureType: "cape", hash: &cape},
		{name: "already clear skin", profileID: empty.ID, textureType: "skin", hash: nil},
		{name: "already clear cape", profileID: empty.ID, textureType: "cape", hash: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.SetProfileTexture(ctx, adminActor, tc.profileID, tc.textureType, tc.hash); err != nil {
				t.Fatalf("exact no-op should skip database update: %v", err)
			}
		})
	}

	different := "different_skin_hash"
	err := svc.SetProfileTexture(ctx, adminActor, profile.ID, "skin", &different)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.Message != "profile update should not run" {
		t.Fatalf("different skin error=%#v; want exact trigger failure", err)
	}
	got, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || got == nil || got.SkinHash == nil || *got.SkinHash != skin ||
		got.CapeHash == nil || *got.CapeHash != cape {
		t.Fatalf("failed non-noop update changed profile: profile=%#v err=%v", got, err)
	}
}

func assertServicePublicUsage(t *testing.T, svc anyPublicLibrary, hash string, want int64) {
	t.Helper()
	page, err := svc.PublicLibrary(context.Background(), "", 10, "skin", "", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page["items"].([]map[string]any) {
		if item["hash"] == hash {
			if item["usage_count"] != want {
				t.Fatalf("usage_count mismatch for %s want=%d got=%#v", hash, want, item)
			}
			return
		}
	}
	t.Fatalf("missing public library item %s in %#v", hash, page)
}

type anyPublicLibrary interface {
	PublicLibrary(context.Context, string, int, string, string, string) (map[string]any, error)
}
