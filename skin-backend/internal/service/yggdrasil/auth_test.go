package yggdrasil_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
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
	if token, err := db.Tokens.Get(ctx, access); err != nil || token != nil {
		t.Fatalf("authenticate must not persist ygg token in database: %#v err=%v", token, err)
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
	if token, err := db.Tokens.Get(ctx, newAccess); err != nil || token != nil {
		t.Fatalf("refresh must not persist new ygg token in database: %#v err=%v", token, err)
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
		if token, err := db.Tokens.Get(ctx, access); err != nil || token != nil {
			t.Fatalf("ygg token %q must not be stored in database: %#v err=%v", access, token, err)
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
