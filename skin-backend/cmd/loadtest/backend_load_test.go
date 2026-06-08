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

type capacityResult struct {
	Scenario       loadScenario
	Best           stepSummary
	TestedMax      int
	HitTestCeiling bool
	Pass           bool
}

type capacityConfig struct {
	Levels        []int
	Duration      time.Duration
	FailThreshold float64
	MaxP95        time.Duration
	MaxDBConns    int
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
	cfg, err := loadTestConfig()
	if err != nil {
		t.Fatal(err)
	}

	db, handler := testutil.NewTestAppWithMaxConnectionsTB(t, int32(cfg.MaxDBConns))
	cfg.MaxDBConns = int(db.Pool.Stat().MaxConns())
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

	results := make([]scenarioResult, 0, len(scenarios)*len(cfg.Levels))
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			for i, concurrency := range cfg.Levels {
				client := newHTTPClient(concurrency, 5*time.Second, false)
				target, err := buildURL(server.URL, scenario.Path)
				if err != nil {
					t.Fatal(err)
				}
				opts := options{
					method:      scenario.Method,
					body:        scenario.Body,
					contentType: "application/json",
					duration:    cfg.Duration,
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
				if i == 0 && !summaryPasses(summary, cfg.FailThreshold, cfg.MaxP95) {
					t.Fatalf("baseline concurrency %d did not pass capacity thresholds: fail_pct=%.2f p95=%s first_error=%s",
						concurrency,
						summary.FailurePct,
						formatDuration(summary.P95),
						summary.FirstError,
					)
				}
				if !summaryPasses(summary, cfg.FailThreshold, cfg.MaxP95) {
					t.Logf("capacity boundary reached at concurrency=%d fail_pct=%.2f p95=%s", concurrency, summary.FailurePct, formatDuration(summary.P95))
					break
				}
			}
		})
	}
	if err := writeLoadTestReport(reportPath(), cfg, results); err != nil {
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

func loadTestConfig() (capacityConfig, error) {
	levels, err := loadTestConcurrencyLevels()
	if err != nil {
		return capacityConfig{}, err
	}
	return capacityConfig{
		Levels:        levels,
		Duration:      loadTestDuration(),
		FailThreshold: loadTestFailThreshold(),
		MaxP95:        loadTestMaxP95(),
		MaxDBConns:    loadTestMaxDBConns(),
	}, nil
}

func loadTestConcurrencyLevels() ([]int, error) {
	raw := os.Getenv("LOADTEST_CONCURRENCY")
	if raw == "" {
		raw = "1,10,50,100,200,400,800"
	}
	return parseConcurrency(raw)
}

func loadTestDuration() time.Duration {
	raw := os.Getenv("LOADTEST_DURATION")
	if raw == "" {
		return time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return time.Second
	}
	return d
}

func loadTestFailThreshold() float64 {
	raw := os.Getenv("LOADTEST_FAIL_THRESHOLD")
	if raw == "" {
		return 1
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n < 0 {
		return 1
	}
	return n
}

func loadTestMaxP95() time.Duration {
	raw := os.Getenv("LOADTEST_MAX_P95")
	if raw == "" {
		return time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return time.Second
	}
	return d
}

func loadTestMaxDBConns() int {
	raw := os.Getenv("LOADTEST_DB_MAX_CONNECTIONS")
	if raw == "" {
		return 20
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 20
	}
	return n
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

func writeLoadTestReport(path string, cfg capacityConfig, results []scenarioResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	now := time.Now().Format(time.RFC3339)
	fmt.Fprintf(&b, "# Backend Concurrency Load Test Report\n\n")
	fmt.Fprintf(&b, "- Generated at: `%s`\n", now)
	fmt.Fprintf(&b, "- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`\n")
	fmt.Fprintf(&b, "- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites\n")
	fmt.Fprintf(&b, "- Concurrency search levels: `%s`\n", joinInts(cfg.Levels))
	fmt.Fprintf(&b, "- Duration per level: `%s`\n", cfg.Duration)
	fmt.Fprintf(&b, "- Pass condition: failure rate <= `%.2f%%`", cfg.FailThreshold)
	if cfg.MaxP95 > 0 {
		fmt.Fprintf(&b, ", p95 <= `%s`", cfg.MaxP95)
	}
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "- Backend database pool used by harness: `%d` max connections\n", cfg.MaxDBConns)
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
	fmt.Fprintf(&b, "\n## Per-Endpoint One-Second Capacity\n\n")
	fmt.Fprintf(&b, "| Area | Scenario | Sustainable concurrency | Successful req/s at that point | Total req/s | P95 | P99 | Tested ceiling? |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, capacity := range summarizeCapacities(results, cfg) {
		ceiling := "no"
		if capacity.HitTestCeiling {
			ceiling = "yes; raise `LOADTEST_CONCURRENCY` to find the real ceiling"
		}
		if !capacity.Pass {
			ceiling = "no passing level"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %d | %.1f | %.1f | %s | %s | %s |\n",
			capacity.Scenario.Area,
			capacity.Scenario.Name,
			capacity.Best.Concurrency,
			capacity.Best.SuccessRPS,
			capacity.Best.RPS,
			formatDuration(capacity.Best.P95),
			formatDuration(capacity.Best.P99),
			ceiling,
		)
	}
	fmt.Fprintf(&b, "\n## Results\n\n")
	fmt.Fprintf(&b, "| Area | Scenario | Concurrency | Requests | OK | Fail | Fail %% | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Pass | Status | First Error |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | --- |\n")
	for _, result := range results {
		summary := result.Summary
		pass := "yes"
		if !summaryPasses(summary, cfg.FailThreshold, cfg.MaxP95) {
			pass = "no"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %d | %d | %d | %d | %.2f | %.1f | %.1f | %s | %s | %s | %s | %s | `%s` | `%s` |\n",
			result.Scenario.Area,
			result.Scenario.Name,
			result.Concurrency,
			summary.Total,
			summary.Success,
			summary.Failed,
			summary.FailurePct,
			summary.SuccessRPS,
			summary.RPS,
			formatDuration(summary.Avg),
			formatDuration(summary.P50),
			formatDuration(summary.P95),
			formatDuration(summary.P99),
			pass,
			formatStatuses(summary.Statuses),
			escapeTable(summary.FirstError),
		)
	}
	fmt.Fprintf(&b, "\n## Notes\n\n")
	fmt.Fprintf(&b, "- `Sustainable concurrency` means the highest tested concurrent worker count whose one-second run met the pass condition above.\n")
	fmt.Fprintf(&b, "- `Successful req/s` is the useful per-second throughput at that sustainable concurrency, not merely the number of workers.\n")
	fmt.Fprintf(&b, "- `Tested ceiling? = yes` means the endpoint still passed at the highest configured level; increase `LOADTEST_CONCURRENCY` if you need the actual breaking point.\n")
	fmt.Fprintf(&b, "- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.\n")
	fmt.Fprintf(&b, "- A failure is any request with a transport error or non-2xx/3xx response.\n")
	fmt.Fprintf(&b, "- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func summarizeCapacities(results []scenarioResult, cfg capacityConfig) []capacityResult {
	bestByScenario := map[string]capacityResult{}
	order := make([]string, 0)
	for _, result := range results {
		name := result.Scenario.Name
		if _, ok := bestByScenario[name]; !ok {
			order = append(order, name)
			bestByScenario[name] = capacityResult{Scenario: result.Scenario}
		}
		capacity := bestByScenario[name]
		if result.Concurrency > capacity.TestedMax {
			capacity.TestedMax = result.Concurrency
		}
		if summaryPasses(result.Summary, cfg.FailThreshold, cfg.MaxP95) && result.Concurrency >= capacity.Best.Concurrency {
			capacity.Best = result.Summary
			capacity.Pass = true
		}
		bestByScenario[name] = capacity
	}
	out := make([]capacityResult, 0, len(order))
	for _, name := range order {
		capacity := bestByScenario[name]
		capacity.HitTestCeiling = capacity.Pass && capacity.Best.Concurrency == maxInt(cfg.Levels)
		out = append(out, capacity)
	}
	return out
}

func summaryPasses(summary stepSummary, failThreshold float64, maxP95 time.Duration) bool {
	if summary.Total == 0 || summary.FailurePct > failThreshold {
		return false
	}
	return maxP95 <= 0 || summary.P95 <= maxP95
}

func maxInt(values []int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
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
