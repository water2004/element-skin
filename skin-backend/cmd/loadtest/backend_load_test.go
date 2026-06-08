package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestRealBackendLoad(t *testing.T) {
	if os.Getenv("LOADTEST_ENABLE") != "1" {
		t.Skip("set LOADTEST_ENABLE=1 to run the real test-backend load test")
	}
	levels, err := loadTestConcurrencyLevels()
	if err != nil {
		t.Fatal(err)
	}
	duration := loadTestDuration()

	db, handler := testutil.NewTestAppTB(t)
	seedLoadTestData(t, db)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	loginClient := newHTTPClient(1, 5*time.Second, false)
	cookie, err := login(loginClient, server.URL, "/site-login", "load-user-000@example.com", "Password123")
	if err != nil {
		t.Fatalf("login seed user: %v", err)
	}
	loginClient.CloseIdleConnections()

	scenarios := []struct {
		name   string
		path   string
		cookie string
	}{
		{name: "public-settings", path: "/public/settings"},
		{name: "public-library", path: "/public/skin-library?limit=20&q=Load"},
		{name: "me", path: "/me", cookie: cookie},
		{name: "my-textures", path: "/me/textures?limit=20", cookie: cookie},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			for _, concurrency := range levels {
				client := newHTTPClient(concurrency, 5*time.Second, false)
				target, err := buildURL(server.URL, scenario.path)
				if err != nil {
					t.Fatal(err)
				}
				opts := options{method: http.MethodGet, duration: duration, timeout: 5 * time.Second}
				summary := runStep(client, target, opts, scenario.cookie, concurrency)
				client.CloseIdleConnections()
				t.Logf("concurrency=%d requests=%d ok=%d fail=%d fail_pct=%.2f rps=%.1f avg=%s p50=%s p95=%s p99=%s status=%s",
					summary.Concurrency,
					summary.Total,
					summary.Success,
					summary.Failed,
					summary.FailurePct,
					summary.RPS,
					formatDuration(summary.Avg),
					formatDuration(summary.P50),
					formatDuration(summary.P95),
					formatDuration(summary.P99),
					formatStatuses(summary.Statuses),
				)
				if summary.FailurePct > 1 {
					t.Fatalf("failure rate %.2f%% exceeded threshold at concurrency %d", summary.FailurePct, concurrency)
				}
			}
		})
	}
}

func loadTestConcurrencyLevels() ([]int, error) {
	raw := os.Getenv("LOADTEST_CONCURRENCY")
	if raw == "" {
		raw = "1,10,50,100"
	}
	return parseConcurrency(raw)
}

func loadTestDuration() time.Duration {
	raw := os.Getenv("LOADTEST_DURATION")
	if raw == "" {
		return 5 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 5 * time.Second
	}
	return d
}

func seedLoadTestData(tb testing.TB, db *database.DB) {
	tb.Helper()
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		email := fmt.Sprintf("load-user-%03d@example.com", i)
		user := testutil.CreateUser(tb, db, email, "Password123", fmt.Sprintf("LoadUser%03d", i), i == 0)
		for p := 0; p < 3; p++ {
			testutil.CreateProfile(tb, db, user.ID, "", fmt.Sprintf("LoadProfile%03d_%d", i, p))
		}
		for n := 0; n < 5; n++ {
			hash := fmt.Sprintf("load_texture_%03d_%03d", i, n)
			textureType := "skin"
			model := "default"
			if n%2 == 1 {
				textureType = "cape"
			}
			if n%3 == 0 {
				model = "slim"
			}
			note := "Load Texture " + strconv.Itoa(i) + "-" + strconv.Itoa(n)
			if err := db.Textures.AddToLibrary(ctx, user.ID, hash, textureType, note, n%4 != 0, model); err != nil {
				tb.Fatalf("seed texture: %v", err)
			}
		}
	}
}
