package auth_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	authsvc "element-skin/backend/internal/service/auth"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestAuthRegisterPropagatesDependencyErrorsWithoutCreatingUser(t *testing.T) {
	t.Run("settings cache error", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		cache := redisstore.NewMemoryStore()
		cache.Err = errors.New("settings cache unavailable")
		svc := authsvc.Service{DB: db, Cfg: testutil.TestConfig(), Redis: cache, Settings: settingssvc.Settings{DB: db, Redis: cache}}

		id, err := svc.Register(ctx, "register-settings-error@test.com", "Password123", "RegisterSettingsError", "", "")
		if id != "" || !errors.Is(err, cache.Err) {
			t.Fatalf("settings dependency Register id=%q err=%v want empty exact cache error", id, err)
		}
		if count, countErr := db.Users.Count(ctx); countErr != nil || count != 0 {
			t.Fatalf("settings dependency failure left users=%d err=%v", count, countErr)
		}
	})

	t.Run("verification code read error", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
			t.Fatal(err)
		}
		cache := &verificationCodeFailStore{Store: testutil.NewMemoryRedis(), failGet: true}
		svc := authsvc.Service{DB: db, Cfg: testutil.TestConfig(), Redis: cache, Settings: settingssvc.Settings{DB: db, Redis: cache}}

		id, err := svc.Register(ctx, "register-verify-error@test.com", "Password123", "RegisterVerifyError", "", "VERIFY12")
		if id != "" || err == nil || err.Error() != "get verification failed" {
			t.Fatalf("verification dependency Register id=%q err=%v want empty exact get failure", id, err)
		}
		if user, userErr := db.Users.GetByEmail(ctx, "register-verify-error@test.com"); userErr != nil || user != nil {
			t.Fatalf("verification dependency failure created user=%#v err=%v", user, userErr)
		}
	})

	t.Run("invite read error", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		svc := newAuthService(db, testutil.TestConfig())
		if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
			t.Fatal(err)
		}
		if err := svc.Settings.InvalidateCache(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `DROP TABLE invites CASCADE`); err != nil {
			t.Fatal(err)
		}

		id, err := svc.Register(ctx, "register-invite-error@test.com", "Password123", "RegisterInviteError", "BROKEN", "")
		if id != "" {
			t.Fatalf("invite dependency Register id=%q want empty", id)
		}
		assertPgCode(t, err, "42P01")
		if user, userErr := db.Users.GetByEmail(ctx, "register-invite-error@test.com"); userErr != nil || user != nil {
			t.Fatalf("invite dependency failure created user=%#v err=%v", user, userErr)
		}
	})

	t.Run("profile lookup error", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		svc := newAuthService(db, testutil.TestConfig())
		if _, err := db.Pool.Exec(ctx, `DROP TABLE profiles CASCADE`); err != nil {
			t.Fatal(err)
		}

		id, err := svc.Register(ctx, "register-profile-error@test.com", "Password123", "RegisterProfileError", "", "")
		if id != "" {
			t.Fatalf("profile dependency Register id=%q want empty", id)
		}
		assertPgCode(t, err, "42P01")
		if user, userErr := db.Users.GetByEmail(ctx, "register-profile-error@test.com"); userErr != nil || user != nil {
			t.Fatalf("profile dependency failure created user=%#v err=%v", user, userErr)
		}
	})
}

func TestAuthServiceClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	db.Close()

	if result, err := svc.Login(ctx, "closed-auth@test.com", "Password123"); result != nil || !closedPoolError(err) {
		t.Fatalf("Login closed database = result=%#v err=%v; want nil and closed pool", result, err)
	}
	if result, err := svc.IssueSessionForUser(ctx, "closed-user"); result != nil || !closedPoolError(err) {
		t.Fatalf("IssueSessionForUser closed database = result=%#v err=%v; want nil and closed pool", result, err)
	}
	if id, err := svc.Register(ctx, "closed-register-dependency@test.com", "Password123", "ClosedRegisterDependency", "", ""); id != "" || !closedPoolError(err) {
		t.Fatalf("Register closed database = id=%q err=%v; want empty id and closed pool", id, err)
	}
}

func TestAuthRegisterPropagatesEachSettingsStageFailureWithoutCreatingUser(t *testing.T) {
	for _, key := range []string{
		"allow_register",
		"email_verify_enabled",
		"require_invite",
		"profile_uuid_mode",
	} {
		t.Run(key, func(t *testing.T) {
			db, _ := testutil.NewTestApp(t)
			ctx := context.Background()
			wantErr := errors.New(key + " unavailable")
			cache := &settingKeyFailStore{
				Store:   testutil.NewMemoryRedis(),
				failKey: key,
				err:     wantErr,
			}
			svc := authsvc.Service{
				DB:       db,
				Cfg:      testutil.TestConfig(),
				Redis:    cache,
				Settings: settingssvc.Settings{DB: db, Redis: cache},
			}

			id, err := svc.Register(ctx, key+"@test.com", "Password123", "SettingsStage", "", "")
			if id != "" || !errors.Is(err, wantErr) {
				t.Fatalf("Register at %s = id=%q err=%v; want empty id and exact dependency error", key, id, err)
			}
			if count, countErr := db.Users.Count(ctx); countErr != nil || count != 0 {
				t.Fatalf("Register at %s left users=%d err=%v", key, count, countErr)
			}
		})
	}
}

type settingKeyFailStore struct {
	redisstore.Store
	failKey string
	err     error
}

func (s *settingKeyFailStore) GetSetting(ctx context.Context, key string) (string, error) {
	if key == s.failKey {
		return "", s.err
	}
	return s.Store.GetSetting(ctx, key)
}
