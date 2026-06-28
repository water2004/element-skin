package yggdrasil_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestYggdrasilAuthRefreshAndValidate(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-auth@test.com", "Password123", "YggAuth", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_auth_profile", "YggRole")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	auth, err := ygg.Authenticate(ctx, "ygg-auth@test.com", "Password123", "client_token", true)
	if err != nil {
		t.Fatal(err)
	}
	if auth["clientToken"] != "client_token" || auth["accessToken"] == "" {
		t.Fatalf("auth token response mismatch: %#v", auth)
	}
	selected := auth["selectedProfile"].(map[string]any)
	if selected["id"] != profile.ID || selected["name"] != profile.Name {
		t.Fatalf("selected profile mismatch: %#v", selected)
	}
	available := auth["availableProfiles"].([]map[string]any)
	if len(available) != 1 || available[0]["id"] != profile.ID || available[0]["name"] != profile.Name {
		t.Fatalf("available profiles mismatch: %#v", available)
	}
	userPayload := auth["user"].(map[string]any)
	props := userPayload["properties"].([]map[string]any)
	if userPayload["id"] != user.ID || len(props) != 1 || props[0]["name"] != "preferredLanguage" || props[0]["value"] != "zh_CN" {
		t.Fatalf("requestUser payload mismatch: %#v", userPayload)
	}
	access := auth["accessToken"].(string)
	if token, err := redis.GetYggToken(ctx, access); err != nil || token.UserID != user.ID {
		t.Fatalf("authenticate should store ygg token in redis: %#v err=%v", token, err)
	}
	if err := ygg.Validate(ctx, access, "client_token"); err != nil {
		t.Fatalf("fresh token should validate: %v", err)
	}

	refreshed, err := ygg.Refresh(ctx, access, "client_token", "", true)
	if err != nil {
		t.Fatal(err)
	}
	newAccess := refreshed["accessToken"].(string)
	if newAccess == "" || newAccess == access || refreshed["clientToken"] != "client_token" {
		t.Fatalf("refresh response mismatch: %#v", refreshed)
	}
	if err := ygg.Validate(ctx, access, "client_token"); err == nil {
		t.Fatal("old access token should be invalid after refresh")
	}
	if _, err := redis.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old access token should be removed from redis after refresh, got %v", err)
	}
	if err := ygg.Validate(ctx, newAccess, "client_token"); err != nil {
		t.Fatalf("new access token should validate: %v", err)
	}
	if err := redis.DeleteYggToken(ctx, newAccess); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "unbound_access", ClientToken: "client_unbound", UserID: user.ID, ProfileID: nil, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	bound, err := ygg.Refresh(ctx, "unbound_access", "client_unbound", profile.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	boundSelected := bound["selectedProfile"].(map[string]any)
	if boundSelected["id"] != profile.ID || boundSelected["name"] != profile.Name {
		t.Fatalf("refresh selectedID should bind profile: %#v", bound)
	}

	if _, err := ygg.Authenticate(ctx, profile.Name, "wrong-password", "", false); err == nil || !strings.Contains(err.Error(), "Invalid credentials") {
		t.Fatalf("bad credentials should return ygg error, got %v", err)
	}
}

func TestYggdrasilSessionPermissionsRejectExactOperations(t *testing.T) {
	for _, tc := range []struct {
		name       string
		permission string
		call       func(context.Context, yggdrasil.Yggdrasil, *testing.T, string, string) error
	}{
		{
			name:       "authenticate",
			permission: "yggdrasil_session.create.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				_, err := ygg.Authenticate(ctx, email, password, "client", false)
				return err
			},
		},
		{
			name:       "refresh",
			permission: "yggdrasil_session.refresh.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				auth := mustYggAuth(t, ctx, ygg, email, password)
				_, err := ygg.Refresh(ctx, auth["accessToken"].(string), auth["clientToken"].(string), "", false)
				return err
			},
		},
		{
			name:       "validate",
			permission: "yggdrasil_session.validate.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				auth := mustYggAuth(t, ctx, ygg, email, password)
				return ygg.Validate(ctx, auth["accessToken"].(string), auth["clientToken"].(string))
			},
		},
		{
			name:       "invalidate",
			permission: "yggdrasil_session.invalidate.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				auth := mustYggAuth(t, ctx, ygg, email, password)
				access := auth["accessToken"].(string)
				err := ygg.Invalidate(ctx, access)
				if _, tokenErr := ygg.Redis.GetYggToken(ctx, access); tokenErr != nil {
					t.Fatalf("denied invalidate must keep the token: %v", tokenErr)
				}
				return err
			},
		},
		{
			name:       "signout",
			permission: "yggdrasil_session.signout.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				mustYggAuth(t, ctx, ygg, email, password)
				return ygg.Signout(ctx, email, password)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestApp(t)
			ctx := context.Background()
			user := testutil.CreateUser(t, db, "ygg-"+tc.name+"-deny@test.com", "Password123", "YggPermissionDeny", false)
			testutil.CreateProfile(t, db, user.ID, "ygg_"+tc.name+"_deny_profile", "YggDeny"+tc.name)
			redis := testutil.NewMemoryRedis()
			ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

			def := permission.MustDefinitionByCode(tc.permission)
			if tc.name != "authenticate" {
				auth := mustYggAuth(t, ctx, ygg, user.Email, "Password123")
				if err := redis.DeleteYggToken(ctx, auth["accessToken"].(string)); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "deny", ""); err != nil {
				t.Fatal(err)
			}
			err := tc.call(ctx, ygg, t, user.Email, "Password123")
			if err == nil || !strings.Contains(err.Error(), "Permission denied.") {
				t.Fatalf("%s without %s should be denied with exact ygg permission error, got %v", tc.name, tc.permission, err)
			}
		})
	}
}

func mustYggAuth(t *testing.T, ctx context.Context, ygg yggdrasil.Yggdrasil, email, password string) map[string]any {
	t.Helper()
	auth, err := ygg.Authenticate(ctx, email, password, "client", false)
	if err != nil {
		t.Fatalf("authenticate fixture failed: %v", err)
	}
	return auth
}

func TestYggdrasilAuthenticateProfileNameBindsSelectedProfileAndMultiProfileEmailDoesNot(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-profile-login@test.com", "Password123", "YggProfileLogin", false)
	first := testutil.CreateProfile(t, db, user.ID, "ygg_profile_login_first", "YggLoginFirst")
	second := testutil.CreateProfile(t, db, user.ID, "ygg_profile_login_second", "YggLoginSecond")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	byProfileName, err := ygg.Authenticate(ctx, second.Name, "Password123", "client-profile-name", false)
	if err != nil {
		t.Fatal(err)
	}
	selected := byProfileName["selectedProfile"].(map[string]any)
	if selected["id"] != second.ID || selected["name"] != second.Name {
		t.Fatalf("profile-name login should bind exactly the named profile: %#v", byProfileName)
	}
	if profiles := byProfileName["availableProfiles"].([]map[string]any); len(profiles) != 2 {
		t.Fatalf("profile-name login should still expose both available profiles: %#v", profiles)
	}
	nameAccess := byProfileName["accessToken"].(string)
	nameToken, err := redis.GetYggToken(ctx, nameAccess)
	if err != nil || nameToken.ProfileID == nil || *nameToken.ProfileID != second.ID {
		t.Fatalf("profile-name login token should be bound to selected profile: token=%#v err=%v", nameToken, err)
	}

	byEmail, err := ygg.Authenticate(ctx, user.Email, "Password123", "client-email", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := byEmail["selectedProfile"]; ok {
		t.Fatalf("email login with multiple profiles must not auto-select a profile: %#v", byEmail)
	}
	emailAccess := byEmail["accessToken"].(string)
	emailToken, err := redis.GetYggToken(ctx, emailAccess)
	if err != nil || emailToken.ProfileID != nil {
		t.Fatalf("multi-profile email login token should remain unbound: token=%#v err=%v", emailToken, err)
	}
	profiles := byEmail["availableProfiles"].([]map[string]any)
	if len(profiles) != 2 || profiles[0]["id"] != first.ID || profiles[1]["id"] != second.ID {
		t.Fatalf("email login should return both available profiles in store order: %#v", profiles)
	}
}

func TestYggdrasilSignoutInvalidateAndTokenLimitUseRedisOnly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-signout@test.com", "Password123", "YggSignout", false)
	testutil.CreateProfile(t, db, user.ID, "ygg_signout_profile", "YggSignoutProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	var accesses []string
	for i := 0; i < 6; i++ {
		auth, err := ygg.Authenticate(ctx, user.Email, "Password123", "client", false)
		if err != nil {
			t.Fatal(err)
		}
		accesses = append(accesses, auth["accessToken"].(string))
	}
	if _, err := redis.GetYggToken(ctx, accesses[0]); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("oldest token should be trimmed from redis, got %v", err)
	}
	for _, access := range accesses[1:] {
		if _, err := redis.GetYggToken(ctx, access); err != nil {
			t.Fatalf("newer token %q should remain in redis: %v", access, err)
		}
	}

	if err := ygg.Invalidate(ctx, accesses[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := redis.GetYggToken(ctx, accesses[1]); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidate should delete one redis token, got %v", err)
	}
	if err := ygg.Signout(ctx, user.Email, "Password123"); err != nil {
		t.Fatal(err)
	}
	for _, access := range accesses[2:] {
		if _, err := redis.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("signout should delete all remaining redis tokens, %q got %v", access, err)
		}
	}
}

func TestYggdrasilTokenReadsRedisOnly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-token@test.com", "Password123", "YggToken", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_token_profile", "YggTokenProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	token := model.Token{AccessToken: "redis_access", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: database.NowMS()}
	if err := redis.SetYggToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := ygg.Token(ctx, token.AccessToken)
	if err != nil || got.AccessToken != token.AccessToken || got.UserID != user.ID || got.ProfileID == nil || *got.ProfileID != profile.ID {
		t.Fatalf("Token should read redis token: %#v err=%v", got, err)
	}
	if _, err := ygg.Token(ctx, "missing_access"); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("missing redis token should be unauthorized ygg error, got %v", err)
	}
}

func TestYggdrasilValidateRefreshSignoutAndInvalidateEdgeCases(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-edge@test.com", "Password123", "YggEdge", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_edge_profile", "YggEdgeProfile")
	otherUser := testutil.CreateUser(t, db, "ygg-edge-other@test.com", "Password123", "YggEdgeOther", false)
	otherProfile := testutil.CreateProfile(t, db, otherUser.ID, "ygg_edge_other_profile", "YggEdgeOtherProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}
	profileID := profile.ID

	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "bound_edge_access", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ygg.Invalidate(ctx, ""); err != nil {
		t.Fatalf("empty invalidate should be a no-op: %v", err)
	}
	if token, err := redis.GetYggToken(ctx, "bound_edge_access"); err != nil || token.AccessToken != "bound_edge_access" {
		t.Fatalf("empty invalidate must not delete existing tokens: token=%#v err=%v", token, err)
	}
	if err := ygg.Validate(ctx, "bound_edge_access", "wrong-client"); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("validate should reject wrong client token, got %v", err)
	}
	if _, err := ygg.Refresh(ctx, "bound_edge_access", "client", profile.ID, false); err == nil || !strings.Contains(err.Error(), "Access token already has a profile assigned") {
		t.Fatalf("refresh should reject selecting a profile on already-bound token, got %v", err)
	}

	oldProfileID := profile.ID
	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "expired_edge_access", ClientToken: "client", UserID: user.ID, ProfileID: &oldProfileID, CreatedAt: database.NowMS() - int64(16*24*time.Hour/time.Millisecond)}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ygg.Validate(ctx, "expired_edge_access", "client"); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("validate should reject expired token by created_at even if redis key exists, got %v", err)
	}

	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "unbound_edge_access", ClientToken: "client", UserID: user.ID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := ygg.Refresh(ctx, "unbound_edge_access", "wrong-client", "", false); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("refresh should reject wrong client token, got %v", err)
	}
	if _, err := ygg.Refresh(ctx, "unbound_edge_access", "client", otherProfile.ID, false); err == nil || !strings.Contains(err.Error(), "Invalid profile") {
		t.Fatalf("refresh should reject selecting a foreign profile, got %v", err)
	}

	if err := ygg.Signout(ctx, user.Email, "wrong-password"); err == nil || !strings.Contains(err.Error(), "Invalid credentials") {
		t.Fatalf("signout should reject bad credentials, got %v", err)
	}
}

func TestYggdrasilRejectsBoundTokenAfterProfileIDIsReassigned(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	originalOwner := testutil.CreateUser(t, db, "ygg-stale-owner@test.com", "Password123", "YggStaleOwner", false)
	newOwner := testutil.CreateUser(t, db, "ygg-stale-new-owner@test.com", "Password123", "YggStaleNewOwner", false)
	profile := testutil.CreateProfile(t, db, originalOwner.ID, "ygg_reassigned_profile", "YggOriginalRole")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}
	token := model.Token{
		AccessToken: "stale_reassigned_access",
		ClientToken: "stale_reassigned_client",
		UserID:      originalOwner.ID,
		ProfileID:   &profile.ID,
		CreatedAt:   database.NowMS(),
	}
	if err := redis.SetYggToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Profiles.DeleteCascade(ctx, profile.ID); err != nil || !ok {
		t.Fatalf("delete original profile: ok=%v err=%v", ok, err)
	}
	if err := db.Profiles.Create(ctx, model.Profile{
		ID:           profile.ID,
		UserID:       newOwner.ID,
		Name:         "YggReassignedRole",
		TextureModel: "default",
	}); err != nil {
		t.Fatal(err)
	}

	if err := ygg.Validate(ctx, token.AccessToken, token.ClientToken); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("validate must reject a token whose profile ID now belongs to another user, got %v", err)
	}
	if _, err := ygg.Token(ctx, token.AccessToken); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("token lookup must reject reassigned profile ownership, got %v", err)
	}
	if _, err := ygg.Refresh(ctx, token.AccessToken, token.ClientToken, "", false); err == nil || !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("refresh must reject reassigned profile ownership, got %v", err)
	}
}

func TestYggdrasilAuthenticateRevokesNewTokenWhenLimitTrimFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-trim-fail@test.com", "Password123", "YggTrimFail", false)
	testutil.CreateProfile(t, db, user.ID, "ygg_trim_fail_profile", "YggTrimFailProfile")
	cache := &trimFailStore{Store: testutil.NewMemoryRedis()}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}

	response, err := ygg.Authenticate(ctx, user.Email, "Password123", "trim-fail-client", false)
	if response != nil || err == nil || err.Error() != "token limit trim failed" {
		t.Fatalf("trim failure response=%#v err=%v, want nil and exact dependency error", response, err)
	}
	if cache.setCalls != 1 || cache.trimCalls != 1 || cache.lastToken.AccessToken == "" || cache.lastToken.UserID != user.ID {
		t.Fatalf("token operations mismatch: set=%d trim=%d token=%#v", cache.setCalls, cache.trimCalls, cache.lastToken)
	}
	if _, err := cache.Store.GetYggToken(ctx, cache.lastToken.AccessToken); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("trim failure must revoke newly-created token, got %v", err)
	}
}

func TestYggdrasilRefreshPreservesOldTokenWhenResponseUserLookupFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-refresh-user-fail@test.com", "Password123", "YggRefreshUserFail", false)
	cache := testutil.NewMemoryRedis()
	old := model.Token{
		AccessToken: "refresh_user_lookup_old_access",
		ClientToken: "refresh_user_lookup_client",
		UserID:      user.ID,
		CreatedAt:   database.NowMS(),
	}
	if err := cache.SetYggToken(ctx, old, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE users RENAME TO users_unavailable`); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}
	response, err := ygg.Refresh(ctx, old.AccessToken, old.ClientToken, "", true)
	var pgErr *pgconn.PgError
	if response != nil || !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
		t.Fatalf("Refresh response=%#v err=%#v; want nil and PostgreSQL 42P01", response, err)
	}
	got, err := cache.GetYggToken(ctx, old.AccessToken)
	if err != nil ||
		got.AccessToken != old.AccessToken ||
		got.ClientToken != old.ClientToken ||
		got.UserID != old.UserID ||
		got.ProfileID != nil ||
		got.CreatedAt != old.CreatedAt {
		t.Fatalf("failed refresh changed old token: token=%#v err=%v want=%#v", got, err, old)
	}
}

func TestConcurrentYggdrasilRefreshConsumesAccessTokenExactlyOnce(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-concurrent-refresh@test.com", "Password123", "YggConcurrentRefresh", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_concurrent_refresh_profile", "YggConcurrent")
	cache := testutil.NewMemoryRedis()
	old := model.Token{
		AccessToken: "concurrent_refresh_old_access",
		ClientToken: "concurrent_refresh_client",
		UserID:      user.ID,
		ProfileID:   &profile.ID,
		CreatedAt:   database.NowMS(),
	}
	if err := cache.SetYggToken(ctx, old, time.Minute); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}

	type result struct {
		response map[string]any
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			response, err := ygg.Refresh(
				context.Background(),
				old.AccessToken,
				old.ClientToken,
				"",
				false,
			)
			results <- result{response: response, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	rejected := 0
	newAccess := ""
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			if result.response["clientToken"] != old.ClientToken {
				t.Fatalf("successful refresh response=%#v; want original client token", result.response)
			}
			selected := result.response["selectedProfile"].(map[string]any)
			if selected["id"] != profile.ID || selected["name"] != profile.Name {
				t.Fatalf("successful refresh selected profile=%#v; want exact profile", selected)
			}
			newAccess = result.response["accessToken"].(string)
		case result.response == nil && result.err == (util.HTTPError{
			Status:   403,
			Detail:   "Invalid token.",
			YggError: "ForbiddenOperationException",
		}):
			rejected++
		default:
			t.Fatalf("unexpected concurrent refresh result: response=%#v err=%#v", result.response, result.err)
		}
	}
	if successes != 1 || rejected != 1 || newAccess == "" || newAccess == old.AccessToken {
		t.Fatalf("concurrent refresh: successes=%d rejected=%d new_access=%q; want 1, 1, and a new token", successes, rejected, newAccess)
	}
	if _, err := cache.GetYggToken(ctx, old.AccessToken); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old access token must be consumed, got %v", err)
	}
	got, err := cache.GetYggToken(ctx, newAccess)
	if err != nil ||
		got.AccessToken != newAccess ||
		got.ClientToken != old.ClientToken ||
		got.UserID != old.UserID ||
		got.ProfileID == nil ||
		*got.ProfileID != profile.ID {
		t.Fatalf("winning replacement token=%#v err=%v; want exact user, client, and profile", got, err)
	}
}

type trimFailStore struct {
	redisstore.Store
	setCalls  int
	trimCalls int
	lastToken model.Token
}

func (s *trimFailStore) SetYggToken(ctx context.Context, token model.Token, ttl time.Duration) error {
	s.setCalls++
	s.lastToken = token
	return s.Store.SetYggToken(ctx, token, ttl)
}

func (s *trimFailStore) TrimYggTokensByUser(context.Context, string, int) error {
	s.trimCalls++
	return errors.New("token limit trim failed")
}
