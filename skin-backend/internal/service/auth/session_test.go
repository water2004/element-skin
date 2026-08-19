package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	authsvc "element-skin/backend/internal/service/auth"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestSessionRotateRefreshIsSingleUse(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newAuthService(db, cfg)
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

func TestIssueSessionForUserSupportsOIDCLoginAndRejectsMissingUserExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newAuthService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-oidc@test.com", "Password123", "SiteSessionOIDC", false)

	session, err := svc.IssueSessionForUser(ctx, "  "+user.ID+"  ")
	if err != nil || session["user_id"] != user.ID || session["access_token"] == "" || session["refresh_token"] == "" ||
		session["refresh_max_age_seconds"] != cfg.JWTExpireDays*24*3600 {
		t.Fatalf("OIDC user session=%#v err=%v", session, err)
	}
	permissions, ok := session["permissions"].([]string)
	if !ok || len(permissions) == 0 || !containsString(permissions, "account.read.self") {
		t.Fatalf("OIDC user session permissions=%#v", session["permissions"])
	}
	missing, err := svc.IssueSessionForUser(ctx, "missing-user")
	if missing != nil || !httpError(err, 404, "user.resolve.not_found") {
		t.Fatalf("missing OIDC user session=%#v err=%v", missing, err)
	}
}

func TestSessionRotateRefreshRejectsExpiredTokenAndConsumesIt(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newAuthService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-expired@test.com", "Password123", "SiteSessionExpired", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, database.NowMS()-1, database.NowMS()-2); err != nil {
		t.Fatal(err)
	}

	rotated, err := svc.RotateRefresh(ctx, raw)
	if !httpError(err, 401, "refresh_token.verify.expired") || rotated != nil {
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
	svc := newAuthService(db, cfg)
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
	if !httpError(err, 401, "refresh_token.verify.invalid") || rotated != nil {
		t.Fatalf("refresh for deleted user should be rejected exactly: rotated=%#v err=%v", rotated, err)
	}
	if row, err := db.Tokens.GetRefresh(ctx, hash); err != nil || row != nil {
		t.Fatalf("deleting user should remove refresh tokens: row=%#v err=%v", row, err)
	}
}

func TestSessionRevokeRefreshDeletesExactTokenOnly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	userA := testutil.CreateUser(t, db, "site-session-revoke-a@test.com", "Password123", "SiteSessionRevokeA", false)
	userB := testutil.CreateUser(t, db, "site-session-revoke-b@test.com", "Password123", "SiteSessionRevokeB", false)
	rawA, hashA, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	_, hashB, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	now := database.NowMS()
	if err := db.Tokens.AddRefresh(ctx, hashA, userA.ID, now+60_000, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hashB, userB.ID, now+120_000, now+1); err != nil {
		t.Fatal(err)
	}

	if err := svc.RevokeRefresh(ctx, rawA); err != nil {
		t.Fatal(err)
	}
	if revoked, err := db.Tokens.GetRefresh(ctx, hashA); err != nil || revoked != nil {
		t.Fatalf("revoked token should be absent: token=%#v err=%v", revoked, err)
	}
	remaining, err := db.Tokens.GetRefresh(ctx, hashB)
	if err != nil || remaining == nil ||
		remaining["user_id"] != userB.ID ||
		remaining["expires_at"] != now+120_000 ||
		remaining["created_at"] != now+1 {
		t.Fatalf("other refresh token should remain exact: token=%#v err=%v", remaining, err)
	}
}

func TestSessionIssueAndRotateUseConfiguredRefreshLifetime(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newAuthService(db, cfg)
	testutil.CreateUser(t, db, "site-session-lifetime@test.com", "Password123", "SiteSessionLifetime", true)
	if err := db.Settings.Set(ctx, "jwt_expire_days", "3"); err != nil {
		t.Fatal(err)
	}

	login, err := svc.Login(ctx, "site-session-lifetime@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	loginPermissions := login["permissions"].([]string)
	if login["refresh_max_age_seconds"] != 3*24*3600 || !containsString(loginPermissions, "user.read.any") || containsString(loginPermissions, "permission_protected.manage.any") {
		t.Fatalf("login should use configured refresh lifetime and admin permissions: %#v", login)
	}
	rotated, err := svc.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	rotatedPermissions := rotated["permissions"].([]string)
	if rotated["refresh_max_age_seconds"] != 3*24*3600 || !containsString(rotatedPermissions, "user.read.any") || containsString(rotatedPermissions, "permission_protected.manage.any") {
		t.Fatalf("rotated session should preserve configured refresh lifetime and permissions: %#v", rotated)
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
	svc := authsvc.Service{
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
	svc := newAuthService(db, cfg)
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
		case httpError(err, 401, "refresh_token.verify.invalid"):
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
