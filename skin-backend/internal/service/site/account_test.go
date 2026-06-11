package site_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestAccountMeReturnsCountsAndUpdateMePersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-account-service@test.com", "Password123", "SiteAccountService", false)

	if err := svc.UpdateMe(ctx, user.ID, map[string]any{"email": "updated-account@test.com", "display_name": "UpdatedAccount", "preferred_language": "en_US", "avatar_hash": "avatar_hash"}); err != nil {
		t.Fatal(err)
	}
	me, err := svc.Me(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if me["email"] != "updated-account@test.com" || me["display_name"] != "UpdatedAccount" || me["lang"] != "en_US" ||
		me["profile_count"] != 0 || me["texture_count"] != 0 {
		t.Fatalf("Me response mismatch: %#v", me)
	}
}

func TestAccountMeReturnsDatabaseErrorsInsteadOfZeroCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table string
	}{
		{name: "profile count", table: "profiles"},
		{name: "texture count", table: "user_textures"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestApp(t)
			ctx := context.Background()
			svc := newSiteService(db, testutil.TestConfig())
			user := testutil.CreateUser(t, db, tc.name+"@test.com", "Password123", "AccountMeFailure", false)
			if _, err := db.Pool.Exec(ctx, `ALTER TABLE `+tc.table+` RENAME TO unavailable_`+tc.table); err != nil {
				t.Fatal(err)
			}
			result, err := svc.Me(ctx, user.ID)
			var pgErr *pgconn.PgError
			if result != nil || !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
				t.Fatalf("Me result=%#v err=%#v; want nil and PostgreSQL 42P01", result, err)
			}
		})
	}
}

func TestAccountRejectsInvalidUpdatesAndWrongPasswordExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-account-invalid@test.com", "Password123", "SiteAccountInvalid", false)
	other := testutil.CreateUser(t, db, "site-account-invalid-other@test.com", "Password123", "SiteAccountInvalidOther", false)

	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"invalid email", map[string]any{"email": "not-an-email"}, "Invalid email format"},
		{"duplicate display name", map[string]any{"display_name": other.DisplayName}, "Username already exists"},
		{"blank display name", map[string]any{"display_name": "   "}, "Username cannot be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateMe(ctx, user.ID, tc.body)
			var httpErr util.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status != 400 || httpErr.Detail != tc.want {
				t.Fatalf("UpdateMe should reject %s exactly, got %#v", tc.name, err)
			}
		})
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email || unchanged.DisplayName != user.DisplayName {
		t.Fatalf("invalid updates should not mutate account: user=%#v err=%v", unchanged, err)
	}

	err = svc.ChangePassword(ctx, user.ID, "WrongPassword", "NewPassword123")
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 403 || httpErr.Detail != "旧密码错误" {
		t.Fatalf("wrong old password should reject exactly, got %#v", err)
	}
	afterWrongPassword, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || afterWrongPassword == nil || !util.VerifyPassword("Password123", afterWrongPassword.Password) {
		t.Fatalf("wrong old password should not change hash: user=%#v err=%v", afterWrongPassword, err)
	}

	err = svc.ChangePassword(ctx, "missing-user", "Password123", "NewPassword123")
	if !errors.As(err, &httpErr) || httpErr.Status != 404 || httpErr.Detail != "用户不存在" {
		t.Fatalf("missing user password change should reject exactly, got %#v", err)
	}
}

func TestChangePasswordPreservesPasswordAndRefreshWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "change-password-ygg-fail@test.com", "Password123", "ChangePasswordYggFail", false)
	const refreshHash = "change_password_ygg_fail_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	cache := &deleteYggFailStore{Store: testutil.NewMemoryRedis()}
	svc := site.Site{
		DB:       db,
		Cfg:      testutil.TestConfig(),
		Redis:    cache,
		Settings: settingssvc.Settings{DB: db, Redis: cache},
	}

	err := svc.ChangePassword(ctx, user.ID, "Password123", "NewPassword123")
	if err == nil || err.Error() != "ygg token revocation failed" {
		t.Fatalf("ygg revocation failure should be returned exactly, got %v", err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) || util.VerifyPassword("NewPassword123", unchanged.Password) {
		t.Fatalf("failed password change must preserve old hash: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh == nil || refresh["user_id"] != user.ID {
		t.Fatalf("failed password change must preserve refresh token: refresh=%#v err=%v", refresh, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("password change should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}

type deleteYggFailStore struct {
	redisstore.Store
	deleteCalls int
}

func (s *deleteYggFailStore) DeleteYggTokensByUser(context.Context, string) error {
	s.deleteCalls++
	return errors.New("ygg token revocation failed")
}
