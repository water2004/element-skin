package site_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestSessionRotateRefreshIsSingleUse(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	testutil.CreateUser(t, db, "site-session-service@test.com", "Password123", "SiteSessionService", false)
	login, err := svc.Login(ctx, "site-session-service@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := svc.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if rotated["refresh_token"] == "" || rotated["refresh_token"] == login["refresh_token"] {
		t.Fatalf("rotated refresh should be new and non-empty: %#v", rotated)
	}
	if _, err := svc.RotateRefresh(ctx, login["refresh_token"].(string)); err == nil {
		t.Fatal("old refresh token should be consumed")
	}
}

func TestSessionRotateRefreshRejectsExpiredTokenAndConsumesIt(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-expired@test.com", "Password123", "SiteSessionExpired", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, database.NowMS()-1, database.NowMS()-2); err != nil {
		t.Fatal(err)
	}

	rotated, err := svc.RotateRefresh(ctx, raw)
	if !httpError(err, 401, "refresh token expired") || rotated != nil {
		t.Fatalf("expired refresh should be rejected exactly: rotated=%#v err=%v", rotated, err)
	}
	if row, err := db.Tokens.GetRefresh(ctx, hash); err != nil || row != nil {
		t.Fatalf("expired refresh token should still be consumed on failed rotation: row=%#v err=%v", row, err)
	}
}

func TestSessionRotateRefreshRejectsTokenAfterUserDeletion(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-deleted-user@test.com", "Password123", "SiteSessionDeletedUser", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, database.NowMS()+60_000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Users.Delete(ctx, user.ID); err != nil || !ok {
		t.Fatalf("delete user mismatch: ok=%v err=%v", ok, err)
	}

	rotated, err := svc.RotateRefresh(ctx, raw)
	if !httpError(err, 401, "invalid refresh token") || rotated != nil {
		t.Fatalf("refresh for deleted user should be rejected exactly: rotated=%#v err=%v", rotated, err)
	}
	if row, err := db.Tokens.GetRefresh(ctx, hash); err != nil || row != nil {
		t.Fatalf("deleting user should remove refresh tokens: row=%#v err=%v", row, err)
	}
}

func TestSessionIssueAndRotateUseConfiguredRefreshLifetime(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	testutil.CreateUser(t, db, "site-session-lifetime@test.com", "Password123", "SiteSessionLifetime", true)
	if err := db.Settings.Set(ctx, "jwt_expire_days", "3"); err != nil {
		t.Fatal(err)
	}

	login, err := svc.Login(ctx, "site-session-lifetime@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	if login["refresh_max_age_seconds"] != 3*24*3600 || login["is_admin"] != true || login["is_super_admin"] != false {
		t.Fatalf("login should use configured refresh lifetime and roles: %#v", login)
	}
	rotated, err := svc.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if rotated["refresh_max_age_seconds"] != 3*24*3600 || rotated["is_admin"] != true || rotated["is_super_admin"] != false {
		t.Fatalf("rotated session should preserve configured refresh lifetime and roles: %#v", rotated)
	}
}

func TestSessionRotatePreservesOldTokenWhenPreparingNewSessionFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "site-session-prepare-fail@test.com", "Password123", "SiteSessionPrepareFail", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := database.NowMS() + 60_000
	createdAt := database.NowMS()
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, expiresAt, createdAt); err != nil {
		t.Fatal(err)
	}
	cache := &getSettingFailStore{Store: testutil.NewMemoryRedis()}
	svc := site.Site{
		DB:       db,
		Cfg:      testutil.TestConfig(),
		Redis:    cache,
		Settings: settingssvc.Settings{DB: db, Redis: cache},
	}

	rotated, err := svc.RotateRefresh(ctx, raw)
	if err == nil || err.Error() != "settings cache unavailable" || rotated != nil {
		t.Fatalf("failed session preparation = %#v, %v; want nil and exact dependency error", rotated, err)
	}
	old, err := db.Tokens.GetRefresh(ctx, hash)
	if err != nil || old == nil ||
		old["user_id"] != user.ID ||
		old["expires_at"] != expiresAt ||
		old["created_at"] != createdAt {
		t.Fatalf("failed rotation must preserve exact old token: token=%#v err=%v", old, err)
	}
}

func TestSessionConcurrentRotationAllowsExactlyOneSuccess(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-concurrent@test.com", "Password123", "SiteSessionConcurrent", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, database.NowMS()+60_000, database.NowMS()); err != nil {
		t.Fatal(err)
	}

	const attempts = 12
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.RotateRefresh(context.Background(), raw)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	rejected := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case httpError(err, 401, "invalid refresh token"):
			rejected++
		default:
			t.Fatalf("concurrent rotation returned unexpected error: %#v", err)
		}
	}
	if successes != 1 || rejected != attempts-1 {
		t.Fatalf("concurrent rotation successes=%d rejected=%d, want 1 and %d", successes, rejected, attempts-1)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM site_refresh_tokens WHERE user_id=$1`, user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent rotation left %d refresh tokens, want exactly 1", count)
	}
	if old, err := db.Tokens.GetRefresh(ctx, hash); err != nil || old != nil {
		t.Fatalf("concurrent rotation must consume old token: token=%#v err=%v", old, err)
	}
}

type getSettingFailStore struct {
	redisstore.Store
}

func (s *getSettingFailStore) GetSetting(context.Context, string) (string, error) {
	return "", errors.New("settings cache unavailable")
}
