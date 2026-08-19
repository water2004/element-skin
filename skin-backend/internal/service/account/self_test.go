package account_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestAccountServiceMeReturnsCountsAndUpdateSelfPersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	user := testutil.CreateUser(t, db, "account-self@test.com", "Password123", "AccountSelf", false)
	actor := accountActor(t, db, user.ID)
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: user.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, user.ID, "avatar_hash", "skin", "avatar skin", false, "default"); err != nil {
		t.Fatal(err)
	}

	if err := svc.UpdateSelf(ctx, actor, map[string]any{
		"display_name":       "UpdatedAccount",
		"preferred_language": "en_US",
		"avatar_hash":        "avatar_hash",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetAuthUser(ctx, user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("UpdateSelf should invalidate auth cache exactly, got %v", err)
	}
	me, err := svc.Me(ctx, accountActor(t, db, user.ID))
	if err != nil {
		t.Fatal(err)
	}
	permissions := me["permissions"].([]string)
	if me["id"] != user.ID ||
		me["email"] != user.Email ||
		me["display_name"] != "UpdatedAccount" ||
		me["lang"] != "en_US" ||
		me["avatar_hash"].(*string) == nil ||
		*me["avatar_hash"].(*string) != "avatar_hash" ||
		!containsString(permissions, "account.read.self") ||
		me["protected"] != false ||
		me["profile_count"] != 0 ||
		me["texture_count"] != 1 {
		t.Fatalf("Me response mismatch: %#v", me)
	}
}

func TestAccountServiceIncrementalSelfUpdatesPreserveEveryUnsubmittedField(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	user := testutil.CreateUser(t, db, "account-self-incremental@test.com", "Password123", "AccountIncremental", false)
	actor := accountActor(t, db, user.ID)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "incremental_avatar", "skin", "avatar", false, "default"); err != nil {
		t.Fatal(err)
	}
	baseline, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || baseline == nil {
		t.Fatalf("load baseline user: user=%#v err=%v", baseline, err)
	}

	displayName := "Incremental Display"
	if err := svc.UpdateSelf(ctx, actor, map[string]any{"display_name": displayName}); err != nil {
		t.Fatal(err)
	}
	expected := *baseline
	expected.DisplayName = displayName
	assertExactUser(t, db, user.ID, expected)

	language := "en_US"
	if err := svc.UpdateSelf(ctx, actor, map[string]any{"preferred_language": language}); err != nil {
		t.Fatal(err)
	}
	expected.PreferredLanguage = language
	assertExactUser(t, db, user.ID, expected)

	if err := svc.UpdateSelf(ctx, actor, map[string]any{"avatar_hash": "incremental_avatar"}); err != nil {
		t.Fatal(err)
	}
	expected.AvatarHash = accountStringPtr("incremental_avatar")
	assertExactUser(t, db, user.ID, expected)

	if err := svc.UpdateSelf(ctx, actor, map[string]any{"avatar_hash": nil}); err != nil {
		t.Fatal(err)
	}
	expected.AvatarHash = nil
	assertExactUser(t, db, user.ID, expected)
}

func assertExactUser(t *testing.T, db *database.DB, userID string, expected model.User) {
	t.Helper()
	actual, err := db.Users.GetByID(context.Background(), userID)
	if err != nil || actual == nil || !reflect.DeepEqual(*actual, expected) {
		t.Fatalf("incremental account update mismatch: user=%#v err=%v want=%#v", actual, err, expected)
	}
}

func accountStringPtr(value string) *string {
	return &value
}

func TestAccountServiceMeReturnsDatabaseErrorsInsteadOfZeroCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table string
	}{
		{name: "profile count", table: "profiles"},
		{name: "texture count", table: "user_textures"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestAppTB(t)
			ctx := context.Background()
			svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
			user := testutil.CreateUser(t, db, tc.name+"@test.com", "Password123", "AccountMeFailure", false)
			if _, err := db.Pool.Exec(ctx, `ALTER TABLE `+tc.table+` RENAME TO unavailable_`+tc.table); err != nil {
				t.Fatal(err)
			}
			result, err := svc.Me(ctx, accountActor(t, db, user.ID))
			var pgErr *pgconn.PgError
			if result != nil || !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
				t.Fatalf("Me result=%#v err=%#v; want nil and PostgreSQL 42P01", result, err)
			}
		})
	}
}

func TestAccountServiceRejectsInvalidSelfUpdatesAndWrongPasswordExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	user := testutil.CreateUser(t, db, "account-self-invalid@test.com", "Password123", "AccountSelfInvalid", false)
	other := testutil.CreateUser(t, db, "account-self-invalid-other@test.com", "Password123", "AccountSelfInvalidOther", false)
	actor := accountActor(t, db, user.ID)

	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"direct email change", map[string]any{"email": "new-account@test.com"}, "email.update.denied"},
		{"duplicate display name", map[string]any{"display_name": other.DisplayName}, "username.reserve.conflict"},
		{"blank display name", map[string]any{"display_name": "   "}, "username.validate.required"},
		{"missing avatar", map[string]any{"avatar_hash": "missing_avatar_hash"}, "avatar.resolve.not_found"},
		{"invalid avatar type", map[string]any{"avatar_hash": 123}, "avatar.configure.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateSelf(ctx, actor, tc.body)
			if !httpErrorIs(err, http.StatusBadRequest, tc.want) {
				t.Fatalf("UpdateSelf should reject %s exactly, got %#v", tc.name, err)
			}
		})
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email || unchanged.DisplayName != user.DisplayName {
		t.Fatalf("invalid updates should not mutate account: user=%#v err=%v", unchanged, err)
	}

	if err := svc.ChangePasswordSelf(ctx, actor, "WrongPassword", "NewPassword123"); !httpErrorIs(err, http.StatusForbidden, "password.verify.invalid") {
		t.Fatalf("wrong old password should reject exactly, got %#v", err)
	}
	afterWrongPassword, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || afterWrongPassword == nil || !util.VerifyPassword("Password123", afterWrongPassword.Password) {
		t.Fatalf("wrong old password should not change hash: user=%#v err=%v", afterWrongPassword, err)
	}

	missingPasswordActor := actorWithPermissions("missing-user", "account_password.update.self")
	if err := svc.ChangePasswordSelf(ctx, missingPasswordActor, "Password123", "NewPassword123"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("missing user password change should reject exactly, got %#v", err)
	}
	missingUpdateActor := actorWithPermissions("missing-user", "account.update.self")
	if err := svc.UpdateSelf(ctx, missingUpdateActor, map[string]any{"preferred_language": "en_US"}); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("missing user account update should reject exactly, got %#v", err)
	}
}

func TestAccountServiceSelfPermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	user := testutil.CreateUser(t, db, "account-self-perm@test.com", "Password123", "AccountSelfPerm", false)

	for _, tc := range []struct {
		name      string
		actorCode string
		call      func(permission.Actor) error
	}{
		{
			name:      "Me without account.read.self",
			actorCode: "account.update.self",
			call: func(a permission.Actor) error {
				result, err := svc.Me(ctx, a)
				if result != nil {
					t.Fatalf("Me denied call returned result=%#v", result)
				}
				return err
			},
		},
		{
			name:      "UpdateSelf without account.update.self",
			actorCode: "account.read.self",
			call: func(a permission.Actor) error {
				return svc.UpdateSelf(ctx, a, map[string]any{"preferred_language": "en_US"})
			},
		},
		{
			name:      "ChangePasswordSelf without account_password.update.self",
			actorCode: "account.update.self",
			call: func(a permission.Actor) error {
				return svc.ChangePasswordSelf(ctx, a, "Password123", "NewPassword123")
			},
		},
		{
			name:      "DeleteSelf without account.delete.self",
			actorCode: "account.read.self",
			call: func(a permission.Actor) error {
				return svc.DeleteSelf(ctx, a)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := actorWithPermissions(user.ID, tc.actorCode)
			if err := tc.call(actor); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
				t.Fatalf("expected 403 permission denied, got %#v", err)
			}
		})
	}
}
