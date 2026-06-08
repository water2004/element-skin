package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

type loadScenario struct {
	Area   string
	Name   string
	Method string
	Path   string
	Body   string
	Cookie string
}

type scenarioResult struct {
	Scenario    loadScenario
	Concurrency int
	Summary     stepSummary
}

type loadSeed struct {
	User        model.User
	Admin       model.User
	ProfileID   string
	TextureHash string
}

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
	seed := seedLoadTestData(t, db)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	loginClient := newHTTPClient(1, 5*time.Second, false)
	userCookie, err := login(loginClient, server.URL, "/site-login", seed.User.Email, "Password123")
	if err != nil {
		t.Fatalf("login seed user: %v", err)
	}
	adminCookie, err := login(loginClient, server.URL, "/site-login", seed.Admin.Email, "Password123")
	if err != nil {
		t.Fatalf("login seed admin: %v", err)
	}
	loginClient.CloseIdleConnections()

	scenarios := []loadScenario{
		{Area: "Public home", Name: "public-settings", Method: http.MethodGet, Path: "/public/settings"},
		{Area: "Public home", Name: "public-carousel", Method: http.MethodGet, Path: "/public/carousel"},
		{Area: "Public library", Name: "public-library-search", Method: http.MethodGet, Path: "/public/skin-library?limit=20&q=Load"},
		{Area: "Authentication", Name: "site-login", Method: http.MethodPost, Path: "/site-login", Body: fmt.Sprintf(`{"email":%q,"password":"Password123"}`, seed.User.Email)},
		{Area: "User center", Name: "me", Method: http.MethodGet, Path: "/me", Cookie: userCookie},
		{Area: "User center", Name: "my-profiles", Method: http.MethodGet, Path: "/me/profiles?limit=20", Cookie: userCookie},
		{Area: "User center", Name: "my-textures", Method: http.MethodGet, Path: "/me/textures?limit=20", Cookie: userCookie},
		{Area: "User center", Name: "texture-detail", Method: http.MethodGet, Path: "/me/textures/" + seed.TextureHash + "/skin", Cookie: userCookie},
		{Area: "Admin console", Name: "admin-users", Method: http.MethodGet, Path: "/admin/users?limit=20&q=Load", Cookie: adminCookie},
		{Area: "Admin console", Name: "admin-user-detail", Method: http.MethodGet, Path: "/admin/users/" + seed.User.ID, Cookie: adminCookie},
		{Area: "Admin console", Name: "admin-user-profiles", Method: http.MethodGet, Path: "/admin/users/" + seed.User.ID + "/profiles?limit=20", Cookie: adminCookie},
		{Area: "Admin console", Name: "admin-profiles", Method: http.MethodGet, Path: "/admin/profiles?limit=20", Cookie: adminCookie},
		{Area: "Admin console", Name: "admin-textures", Method: http.MethodGet, Path: "/admin/textures?limit=20", Cookie: adminCookie},
		{Area: "Admin console", Name: "admin-invites", Method: http.MethodGet, Path: "/admin/invites?limit=20", Cookie: adminCookie},
		{Area: "Admin console", Name: "admin-settings-site", Method: http.MethodGet, Path: "/admin/settings/site", Cookie: adminCookie},
	}
	scenarios = filterScenarios(scenarios, os.Getenv("LOADTEST_SCENARIOS"))

	results := make([]scenarioResult, 0, len(scenarios)*len(levels))
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			for _, concurrency := range levels {
				client := newHTTPClient(concurrency, 5*time.Second, false)
				target, err := buildURL(server.URL, scenario.Path)
				if err != nil {
					t.Fatal(err)
				}
				opts := options{
					method:      scenario.Method,
					body:        scenario.Body,
					contentType: "application/json",
					duration:    duration,
					timeout:     5 * time.Second,
				}
				summary := runStep(client, target, opts, scenario.Cookie, concurrency)
				client.CloseIdleConnections()
				results = append(results, scenarioResult{Scenario: scenario, Concurrency: concurrency, Summary: summary})
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
				if summary.FirstError != "" {
					t.Logf("first_error=%s", summary.FirstError)
				}
				if summary.FailurePct > 1 {
					t.Fatalf("failure rate %.2f%% exceeded threshold at concurrency %d", summary.FailurePct, concurrency)
				}
			}
		})
	}
	if err := writeLoadTestReport(reportPath(), levels, duration, results); err != nil {
		t.Fatalf("write load test report: %v", err)
	}
}

func filterScenarios(scenarios []loadScenario, raw string) []loadScenario {
	if strings.TrimSpace(raw) == "" {
		return scenarios
	}
	allowed := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			allowed[part] = true
		}
	}
	filtered := make([]loadScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if allowed[scenario.Name] {
			filtered = append(filtered, scenario)
		}
	}
	return filtered
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

func seedLoadTestData(tb testing.TB, db *database.DB) loadSeed {
	tb.Helper()
	ctx := context.Background()
	var seed loadSeed
	for i := 0; i < 100; i++ {
		email := fmt.Sprintf("load-user-%03d@example.com", i)
		user := testutil.CreateUser(tb, db, email, "Password123", fmt.Sprintf("LoadUser%03d", i), i == 0)
		if i == 0 {
			seed.User = user
			seed.Admin = user
		}
		if i == 1 {
			seed.User = user
		}
		for p := 0; p < 3; p++ {
			profile := testutil.CreateProfile(tb, db, user.ID, "", fmt.Sprintf("LoadProfile%03d_%d", i, p))
			if i == 1 && p == 0 {
				seed.ProfileID = profile.ID
			}
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
			if i == 1 && n == 0 {
				seed.TextureHash = hash
			}
		}
	}
	for i := 0; i < 50; i++ {
		if err := db.Invites.Create(ctx, fmt.Sprintf("LOAD_INVITE_%03d", i), 10, "Load invite"); err != nil {
			tb.Fatalf("seed invite: %v", err)
		}
	}
	return seed
}

func reportPath() string {
	if path := os.Getenv("LOADTEST_REPORT"); path != "" {
		return path
	}
	return filepath.Clean(filepath.Join("..", "..", "reports", "concurrency-load-test.md"))
}

func writeLoadTestReport(path string, levels []int, duration time.Duration, results []scenarioResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	now := time.Now().Format(time.RFC3339)
	fmt.Fprintf(&b, "# Backend Concurrency Load Test Report\n\n")
	fmt.Fprintf(&b, "- Generated at: `%s`\n", now)
	fmt.Fprintf(&b, "- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`\n")
	fmt.Fprintf(&b, "- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites\n")
	fmt.Fprintf(&b, "- Concurrency levels: `%s`\n", joinInts(levels))
	fmt.Fprintf(&b, "- Duration per level: `%s`\n", duration)
	fmt.Fprintf(&b, "- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup\n\n")
	fmt.Fprintf(&b, "## Scenario Coverage\n\n")
	fmt.Fprintf(&b, "| Area | Scenario | Method | Path |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- |\n")
	seen := map[string]bool{}
	for _, result := range results {
		key := result.Scenario.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | `%s` |\n", result.Scenario.Area, result.Scenario.Name, result.Scenario.Method, result.Scenario.Path)
	}
	fmt.Fprintf(&b, "\n## Capacity Summary\n\n")
	fmt.Fprintf(&b, "| Area | Scenario | Highest tested concurrency with 0%% failures | Peak RPS at that level | P95 at that level |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: |\n")
	for _, capacity := range summarizeCapacities(results) {
		fmt.Fprintf(&b, "| %s | `%s` | %d | %.1f | %s |\n",
			capacity.Scenario.Area,
			capacity.Scenario.Name,
			capacity.Summary.Concurrency,
			capacity.Summary.RPS,
			formatDuration(capacity.Summary.P95),
		)
	}
	fmt.Fprintf(&b, "\n## Results\n\n")
	fmt.Fprintf(&b, "| Area | Scenario | Concurrency | Requests | OK | Fail | Fail %% | RPS | Avg | P50 | P95 | P99 | Status | First Error |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |\n")
	for _, result := range results {
		summary := result.Summary
		fmt.Fprintf(&b, "| %s | `%s` | %d | %d | %d | %d | %.2f | %.1f | %s | %s | %s | %s | `%s` | `%s` |\n",
			result.Scenario.Area,
			result.Scenario.Name,
			result.Concurrency,
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
			escapeTable(summary.FirstError),
		)
	}
	fmt.Fprintf(&b, "\n## Notes\n\n")
	fmt.Fprintf(&b, "- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.\n")
	fmt.Fprintf(&b, "- A failure is any request with a transport error or non-2xx/3xx response.\n")
	fmt.Fprintf(&b, "- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func summarizeCapacities(results []scenarioResult) []scenarioResult {
	bestByScenario := map[string]scenarioResult{}
	order := make([]string, 0)
	for _, result := range results {
		if result.Summary.FailurePct != 0 {
			continue
		}
		name := result.Scenario.Name
		if _, ok := bestByScenario[name]; !ok {
			order = append(order, name)
			bestByScenario[name] = result
			continue
		}
		if result.Concurrency > bestByScenario[name].Concurrency {
			bestByScenario[name] = result
		}
	}
	out := make([]scenarioResult, 0, len(order))
	for _, name := range order {
		out = append(out, bestByScenario[name])
	}
	return out
}

func escapeTable(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}
