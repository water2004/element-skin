package integration_test

import (
	"context"
	"sync"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestConcurrentRefreshSingleWinner(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "race@test.com", "Password123", "RaceUser", false)

	login := doJSON(t, h, "POST", "/v2/auth/login", map[string]any{"email": user.Email, "password": "Password123"})
	refresh := cookieNamed(login, "refresh_token")
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}

	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := doJSON(t, h, "POST", "/v2/auth/session/refresh", nil, refresh)
			codes <- rr.Code
		}()
	}
	wg.Wait()
	close(codes)

	seen := map[int]int{}
	for code := range codes {
		seen[code]++
	}
	if seen[200] != 1 || seen[401] != 1 {
		t.Fatalf("expected one 200 and one 401, got %#v", seen)
	}
	row, err := db.Tokens.GetRefresh(context.Background(), util.HashRefreshToken(refresh.Value))
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatal("old refresh token hash should be deleted")
	}
}

func TestDatabaseRefreshTokenConsumeIsAtomicAndOneShot(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "primitive@test.com", "Password123", "PrimitiveUser", false)
	now := database.NowMS()
	future := now + 7*24*3600*1000

	if err := db.Tokens.AddRefresh(ctx, "hash_consume", user.ID, future, now); err != nil {
		t.Fatal(err)
	}
	row, err := db.Tokens.ConsumeRefresh(ctx, "hash_consume")
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["user_id"] != user.ID || row["expires_at"] != future {
		t.Fatalf("unexpected consumed refresh row: %#v", row)
	}
	row, err = db.Tokens.ConsumeRefresh(ctx, "hash_consume")
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatalf("refresh token should be one-shot, got %#v", row)
	}

	if err := db.Tokens.AddRefresh(ctx, "hash_race", user.ID, future, now); err != nil {
		t.Fatal(err)
	}
	results := make(chan map[string]any, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := db.Tokens.ConsumeRefresh(context.Background(), "hash_race")
			if err != nil {
				t.Errorf("consume refresh: %v", err)
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for got := range results {
		if got != nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("expected one refresh consume winner, got %d", winners)
	}
}
