package yggdrasil_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

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
		case result.response == nil && yggError(result.err, 403, "ForbiddenOperationException", "Invalid token."):
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

type replaceFailStore struct {
	redisstore.Store
	err          error
	replaceCalls int
	oldAccess    string
	nextToken    model.Token
}

func (s *replaceFailStore) ReplaceYggToken(_ context.Context, oldAccess string, token model.Token, _ time.Duration) (bool, error) {
	s.replaceCalls++
	s.oldAccess = oldAccess
	s.nextToken = token
	return false, s.err
}
