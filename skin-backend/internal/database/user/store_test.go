package user_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestStoreCreateUpdateDeleteAndInviteExhaustion(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	if err := db.Invites.Create(ctx, "domain_user_invite", 1, "domain user"); err != nil {
		t.Fatal(err)
	}
	hash, err := util.HashPassword("Password123")
	if err != nil {
		t.Fatal(err)
	}
	u := model.User{ID: "domain_user", Email: "domain-user@test.com", Password: hash, DisplayName: "DomainUser"}
	p := model.Profile{ID: "domain_user_profile", UserID: u.ID, Name: "DomainUserProfile", TextureModel: "default"}
	if err := store.CreateWithProfile(ctx, u, p, "domain_user_invite", u.Email); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWithProfile(ctx, model.User{ID: "domain_user_2", Email: "domain-user-2@test.com", Password: hash}, model.Profile{ID: "domain_user_profile_2", UserID: "domain_user_2", Name: "DomainUserProfile2", TextureModel: "default"}, "domain_user_invite", "second"); !errors.Is(err, invite.ErrExhausted) {
		t.Fatalf("expected exhausted invite, got %v", err)
	}
	if err := store.Update(ctx, u.ID, map[string]any{"email": "updated-domain-user@test.com", "display_name": "UpdatedDomainUser"}); err != nil {
		t.Fatal(err)
	}
	avatar := "domain-avatar-hash"
	if err := store.Update(ctx, u.ID, map[string]any{"preferred_language": "zh_CN", "avatar_hash": &avatar, "ignored_field": "ignored"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByEmail(ctx, "updated-domain-user@test.com")
	if err != nil || got == nil || got.ID != u.ID || got.DisplayName != "UpdatedDomainUser" || got.PreferredLanguage != "zh_CN" || got.AvatarHash == nil || *got.AvatarHash != avatar {
		t.Fatalf("updated user mismatch: user=%#v err=%v", got, err)
	}
	if count, err := store.Count(ctx); err != nil || count != 1 {
		t.Fatalf("count mismatch: count=%d err=%v", count, err)
	}
	if taken, err := store.IsDisplayNameTaken(ctx, "UpdatedDomainUser", ""); err != nil || !taken {
		t.Fatalf("display name should be taken: taken=%v err=%v", taken, err)
	}
	if taken, err := store.IsDisplayNameTaken(ctx, "UpdatedDomainUser", u.ID); err != nil || taken {
		t.Fatalf("display name exclude should ignore current user: taken=%v err=%v", taken, err)
	}
	directHash, err := util.HashPassword("DirectPassword123")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePassword(ctx, u.ID, directHash); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetByID(ctx, u.ID); err != nil || got == nil || !util.VerifyPassword("DirectPassword123", got.Password) {
		t.Fatalf("UpdatePassword should persist hash: user=%#v err=%v", got, err)
	}
	newHash, err := util.HashPassword("NewPassword123")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdatePasswordAndRevokeRefresh(ctx, u.ID, newHash)
	if err != nil || !updated {
		t.Fatalf("password update mismatch: updated=%v err=%v", updated, err)
	}
	if updated, err := store.UpdatePasswordAndRevokeRefresh(ctx, "missing-user", newHash); err != nil || updated {
		t.Fatalf("password update missing user should return false: updated=%v err=%v", updated, err)
	}
	if err := store.Update(ctx, "missing-user", map[string]any{"preferred_language": "en_US"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user update error = %v; want pgx.ErrNoRows", err)
	}
	if err := store.Update(ctx, "missing-user", map[string]any{"display_name": got.DisplayName}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user with taken display name error = %v; want pgx.ErrNoRows", err)
	}
	deleted, err := store.Delete(ctx, u.ID)
	if err != nil || !deleted {
		t.Fatalf("delete mismatch: deleted=%v err=%v", deleted, err)
	}
	if got, err := store.GetByID(ctx, u.ID); err != nil || got != nil {
		t.Fatalf("deleted user should be gone: user=%#v err=%v", got, err)
	}
	if deleted, err := store.Delete(ctx, u.ID); err != nil || deleted {
		t.Fatalf("delete missing user should return false: deleted=%v err=%v", deleted, err)
	}
}

func TestCreateWithProfileSerializesCompetingInitialSuperAdmins(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	store := user.Store{Pool: db.Pool}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id := "initial_super_" + strconv.Itoa(i)
			errs <- store.CreateWithProfile(context.Background(),
				model.User{
					ID:           id,
					Email:        id + "@test.com",
					Password:     "hash",
					DisplayName:  "InitialSuper" + strconv.Itoa(i),
					IsAdmin:      true,
					IsSuperAdmin: true,
				},
				model.Profile{
					ID:           id + "_profile",
					UserID:       id,
					Name:         "InitialSuperProfile" + strconv.Itoa(i),
					TextureModel: "default",
				},
				"",
				"",
			)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("both competing first registrations should succeed, got %v", err)
		}
	}
	var users, admins, superAdmins int
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE is_admin=TRUE),
		       COUNT(*) FILTER (WHERE is_super_admin=TRUE)
		FROM users
	`).Scan(&users, &admins, &superAdmins); err != nil {
		t.Fatal(err)
	}
	if users != 2 || admins != 1 || superAdmins != 1 {
		t.Fatalf("competing initial registrations must create two users and exactly one super admin: users=%d admins=%d super_admins=%d", users, admins, superAdmins)
	}
}

func TestPublicUserDoesNotExposePassword(t *testing.T) {
	u := model.User{
		ID:                "user-id",
		Email:             "user@test.com",
		Password:          "secret-hash",
		IsAdmin:           true,
		PreferredLanguage: "zh_CN",
		DisplayName:       "Public User",
	}

	body := user.PublicUser(u)
	if body["id"] != u.ID || body["email"] != u.Email || body["display_name"] != u.DisplayName || body["is_admin"] != true || body["preferred_language"] != "zh_CN" {
		t.Fatalf("public user body mismatch: %#v", body)
	}
	if _, ok := body["password"]; ok {
		t.Fatalf("public user body must not expose password: %#v", body)
	}
}

func TestUpdateRollsBackAllUserFieldsWhenOneFieldFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "user-update-atomic@test.com", "Password123", "UserUpdateAtomic", false)
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE users
		ADD CONSTRAINT preferred_language_zh_only CHECK (preferred_language = 'zh_CN')
	`); err != nil {
		t.Fatal(err)
	}

	err := store.Update(ctx, target.ID, map[string]any{
		"email":              "changed-update-atomic@test.com",
		"display_name":       "ChangedUpdateAtomic",
		"preferred_language": "en_US",
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("Update error = %#v; want PostgreSQL 23514", err)
	}
	got, err := store.GetByID(ctx, target.ID)
	if err != nil || got == nil ||
		got.Email != target.Email ||
		got.DisplayName != target.DisplayName ||
		got.PreferredLanguage != target.PreferredLanguage {
		t.Fatalf("failed update changed user: user=%#v err=%v want=%#v", got, err, target)
	}
}

func TestIsEmailConflictMatchesOnlyUsersEmailConstraint(t *testing.T) {
	emailConflict := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
	otherConflict := &pgconn.PgError{Code: "23505", ConstraintName: "profiles_name_key"}
	if !user.IsEmailConflict(emailConflict) {
		t.Fatal("users_email_key 23505 should be recognized as an email conflict")
	}
	if user.IsEmailConflict(otherConflict) || user.IsEmailConflict(errors.New("duplicate key")) || user.IsEmailConflict(nil) {
		t.Fatal("non-email errors must not be recognized as email conflicts")
	}
}

func TestDeleteRollsBackProfilesTokensAndTexturesWhenUserDeleteFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "domain-delete-rollback@test.com", "Password123", "DomainDeleteRollback", false)
	owner := testutil.CreateUser(t, db, "domain-delete-owner@test.com", "Password123", "DomainDeleteOwner", false)
	other := testutil.CreateUser(t, db, "domain-delete-other@test.com", "Password123", "DomainDeleteOther", false)
	profile := testutil.CreateProfile(t, db, target.ID, "domain_delete_rollback", "DomainDeleteRollbackProfile")
	if err := db.Tokens.AddRefresh(ctx, "delete_rollback_refresh", target.ID, 1000, 10); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_rollback_owned", "skin", "Owned", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, other.ID, "delete_rollback_owned", "skin"); err != nil || !added {
		t.Fatalf("add owned texture to other wardrobe: added=%v err=%v", added, err)
	}
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_rollback_shared", "skin", "Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, target.ID, "delete_rollback_shared", "skin"); err != nil || !added {
		t.Fatalf("add shared texture to target wardrobe: added=%v err=%v", added, err)
	}
	if _, err := db.Pool.Exec(ctx, `CREATE TABLE user_delete_guards (user_id TEXT PRIMARY KEY REFERENCES users(id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO user_delete_guards (user_id) VALUES ($1)`, target.ID); err != nil {
		t.Fatal(err)
	}

	deleted, err := store.Delete(ctx, target.ID)
	var pgErr *pgconn.PgError
	if deleted || !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("guarded delete = %v, %#v; want false and PostgreSQL 23503", deleted, err)
	}
	if got, err := store.GetByID(ctx, target.ID); err != nil || got == nil {
		t.Fatalf("failed delete must preserve user: user=%#v err=%v", got, err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got == nil || got.UserID != target.ID {
		t.Fatalf("failed delete must preserve profile: profile=%#v err=%v", got, err)
	}
	if got, err := db.Tokens.GetRefresh(ctx, "delete_rollback_refresh"); err != nil || got == nil || got["user_id"] != target.ID {
		t.Fatalf("failed delete must preserve refresh token: token=%#v err=%v", got, err)
	}
	for _, check := range []struct {
		userID string
		hash   string
	}{
		{target.ID, "delete_rollback_owned"},
		{other.ID, "delete_rollback_owned"},
		{target.ID, "delete_rollback_shared"},
	} {
		if got, err := db.Textures.GetInfo(ctx, check.userID, check.hash, "skin"); err != nil || got == nil {
			t.Fatalf("failed delete must preserve texture user=%q hash=%q: texture=%#v err=%v", check.userID, check.hash, got, err)
		}
	}
	for _, check := range []struct {
		hash  string
		usage int64
	}{
		{"delete_rollback_owned", 2},
		{"delete_rollback_shared", 2},
	} {
		var usage int64
		if err := db.Pool.QueryRow(ctx,
			`SELECT usage_count FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`,
			check.hash,
		).Scan(&usage); err != nil {
			t.Fatal(err)
		}
		if usage != check.usage {
			t.Fatalf("failed delete changed usage for %q: got=%d want=%d", check.hash, usage, check.usage)
		}
	}
}

func TestDeleteRemovesOwnedTexturesAndRecountsOnlySharedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "domain-delete-state@test.com", "Password123", "DomainDeleteState", false)
	owner := testutil.CreateUser(t, db, "domain-delete-state-owner@test.com", "Password123", "DomainDeleteStateOwner", false)
	collector := testutil.CreateUser(t, db, "domain-delete-state-collector@test.com", "Password123", "DomainDeleteStateCollector", false)

	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_state_owned", "skin", "Owned", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, collector.ID, "delete_state_owned", "skin"); err != nil || !added {
		t.Fatalf("collector add owned texture: added=%v err=%v", added, err)
	}
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_state_shared", "skin", "Shared", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, target.ID, "delete_state_shared", "skin"); err != nil || !added {
		t.Fatalf("target add shared texture: added=%v err=%v", added, err)
	}
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_state_unrelated", "cape", "Unrelated", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, collector.ID, "delete_state_unrelated", "cape"); err != nil || !added {
		t.Fatalf("collector add unrelated texture: added=%v err=%v", added, err)
	}

	deleted, err := store.Delete(ctx, target.ID)
	if err != nil || !deleted {
		t.Fatalf("Delete = %v, %v; want true, nil", deleted, err)
	}
	if exists, err := db.Textures.Exists(ctx, "delete_state_owned", "skin"); err != nil || exists {
		t.Fatalf("owned texture exists=%v err=%v; want false, nil", exists, err)
	}
	for _, userID := range []string{target.ID, collector.ID} {
		if info, err := db.Textures.GetInfo(ctx, userID, "delete_state_owned", "skin"); err != nil || info != nil {
			t.Fatalf("owned texture reference for %q=%#v err=%v; want nil, nil", userID, info, err)
		}
	}
	if info, err := db.Textures.GetInfo(ctx, target.ID, "delete_state_shared", "skin"); err != nil || info != nil {
		t.Fatalf("deleted user's shared texture reference=%#v err=%v; want nil, nil", info, err)
	}
	if info, err := db.Textures.GetInfo(ctx, owner.ID, "delete_state_shared", "skin"); err != nil || info == nil {
		t.Fatalf("shared texture owner reference=%#v err=%v; want existing row", info, err)
	}
	for _, check := range []struct {
		hash        string
		textureType string
		wantUsage   int64
	}{
		{"delete_state_shared", "skin", 1},
		{"delete_state_unrelated", "cape", 2},
	} {
		var usage int64
		if err := db.Pool.QueryRow(ctx, `
			SELECT usage_count
			FROM skin_library
			WHERE skin_hash=$1 AND texture_type=$2
		`, check.hash, check.textureType).Scan(&usage); err != nil {
			t.Fatal(err)
		}
		if usage != check.wantUsage {
			t.Fatalf("%s/%s usage_count=%d; want %d", check.hash, check.textureType, usage, check.wantUsage)
		}
	}
}

func TestDeleteMissingUserDoesNotRemoveOrphanedLibraryTexture(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	const missingUserID = "missing-texture-uploader"
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO skin_library (
			skin_hash, texture_type, is_public, uploader, model, name, created_at, usage_count
		) VALUES ('orphaned_hash', 'skin', 1, $1, 'default', 'Orphaned', 1234, 0)
	`, missingUserID); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.Delete(ctx, missingUserID)
	if err != nil || deleted {
		t.Fatalf("Delete missing user = %v, %v; want false, nil", deleted, err)
	}
	var uploader, name string
	var createdAt, usageCount int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT uploader, name, created_at, usage_count
		FROM skin_library
		WHERE skin_hash='orphaned_hash' AND texture_type='skin'
	`).Scan(&uploader, &name, &createdAt, &usageCount); err != nil {
		t.Fatalf("orphaned library row should remain: %v", err)
	}
	if uploader != missingUserID || name != "Orphaned" || createdAt != 1234 || usageCount != 0 {
		t.Fatalf("orphaned library row changed: uploader=%q name=%q created_at=%d usage_count=%d",
			uploader, name, createdAt, usageCount)
	}
}

func TestDeleteDoesNotDeadlockWithConcurrentTextureReupload(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "delete-reupload-race@test.com", "Password123", "DeleteReuploadRace", false)
	const hash = "delete_reupload_race"
	if err := db.Textures.AddToLibrary(ctx, target.ID, hash, "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM user_textures
		WHERE user_id=$1 AND hash=$2 AND texture_type='skin'
	`, target.ID, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		UPDATE skin_library
		SET usage_count=0
		WHERE skin_hash=$1 AND texture_type='skin'
	`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION pause_delete_reupload_insert() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(74628391);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER pause_delete_reupload_insert
		BEFORE INSERT ON user_textures
		FOR EACH ROW
		WHEN (NEW.hash = 'delete_reupload_race')
		EXECUTE FUNCTION pause_delete_reupload_insert();
	`); err != nil {
		t.Fatal(err)
	}

	blocker, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(74628391)`); err != nil {
		t.Fatal(err)
	}
	var blockerPID int
	if err := blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}

	reuploadResult := make(chan error, 1)
	go func() {
		reuploadResult <- db.Textures.AddToLibrary(
			context.Background(),
			target.ID,
			hash,
			"skin",
			"Reuploaded",
			false,
			"slim",
		)
	}()
	reuploadPID := waitForBlockedBackend(t, db.Pool, blockerPID, reuploadResult)

	type deleteResult struct {
		deleted bool
		err     error
	}
	deleteResults := make(chan deleteResult, 1)
	go func() {
		deleted, err := store.Delete(context.Background(), target.ID)
		deleteResults <- deleteResult{deleted: deleted, err: err}
	}()
	_ = waitForBlockedBackend(t, db.Pool, reuploadPID, deleteResults)

	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-reuploadResult; err != nil {
		t.Fatalf("concurrent reupload failed: %v", err)
	}
	deleted := <-deleteResults
	if deleted.err != nil || !deleted.deleted {
		t.Fatalf("concurrent delete=%v, %v; want true, nil", deleted.deleted, deleted.err)
	}
	if got, err := store.GetByID(ctx, target.ID); err != nil || got != nil {
		t.Fatalf("deleted user still exists: user=%#v err=%v", got, err)
	}
	if exists, err := db.Textures.Exists(ctx, hash, "skin"); err != nil || exists {
		t.Fatalf("deleted user's library texture exists=%v err=%v; want false, nil", exists, err)
	}
	if owned, err := db.Textures.VerifyOwnership(ctx, target.ID, hash, "skin"); err != nil || owned {
		t.Fatalf("deleted user's texture ownership exists=%v err=%v; want false, nil", owned, err)
	}
}

func waitForBlockedBackend[T any](
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	blockerPID int,
	result <-chan T,
) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case <-result:
			t.Fatal("database operation completed before the expected lock was released")
		default:
		}
		var blockedPID int
		err := db.QueryRow(t.Context(), `
			SELECT pid
			FROM pg_stat_activity
			WHERE $1 = ANY(pg_blocking_pids(pid))
			ORDER BY pid
			LIMIT 1
		`, blockerPID).Scan(&blockedPID)
		if err == nil {
			return blockedPID
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for a backend to block on pid %d", blockerPID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
