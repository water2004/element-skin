package main

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestParseConcurrency(t *testing.T) {
	got, err := parseConcurrency("1, 5,10")
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 5, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseConcurrency mismatch: got=%v want=%v", got, want)
	}
	if _, err := parseConcurrency("1, nope"); err == nil {
		t.Fatal("invalid concurrency should fail")
	}
	if _, err := parseConcurrency("0"); err == nil {
		t.Fatal("zero concurrency should fail")
	}
}

func TestBuildURL(t *testing.T) {
	got, err := buildURL("http://127.0.0.1:8000/api", "/public/settings")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8000/api/public/settings" {
		t.Fatalf("unexpected URL: %s", got)
	}
	got, err = buildURL("http://127.0.0.1:8000/api", "/admin/users?limit=20&q=Load")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8000/api/admin/users?limit=20&q=Load" {
		t.Fatalf("query string should stay as query, got: %s", got)
	}
	got, err = buildURL("http://ignored", "https://example.com/me")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/me" {
		t.Fatalf("absolute URL should pass through: %s", got)
	}
	if _, err := buildURL("127.0.0.1:8000", "/me"); err == nil {
		t.Fatal("target without scheme should fail")
	}
}

func TestBestCapacity(t *testing.T) {
	summaries := []stepSummary{
		{Concurrency: 10, Total: 100, FailurePct: 0.5, P95: 90 * time.Millisecond},
		{Concurrency: 25, Total: 100, FailurePct: 1.0, P95: 150 * time.Millisecond},
		{Concurrency: 50, Total: 100, FailurePct: 3.0, P95: 100 * time.Millisecond},
	}
	best, ok := bestCapacity(summaries, 1, 200*time.Millisecond)
	if !ok || best != 25 {
		t.Fatalf("best capacity mismatch: best=%d ok=%v", best, ok)
	}
	best, ok = bestCapacity(summaries, 1, 100*time.Millisecond)
	if !ok || best != 10 {
		t.Fatalf("p95 threshold should lower capacity: best=%d ok=%v", best, ok)
	}
	if _, ok = bestCapacity(summaries, 0.1, 50*time.Millisecond); ok {
		t.Fatal("no capacity should pass strict thresholds")
	}
}

func TestNewHTTPClientCapsConnectionsPerHost(t *testing.T) {
	client := newHTTPClient(37, time.Second, false)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type: %T", client.Transport)
	}
	if transport.MaxConnsPerHost != 37 {
		t.Fatalf("MaxConnsPerHost mismatch: %d", transport.MaxConnsPerHost)
	}
	if transport.TLSClientConfig != nil {
		t.Fatal("TLS config should only be set for insecure mode")
	}

	insecure := newHTTPClient(1, time.Second, true)
	insecureTransport := insecure.Transport.(*http.Transport)
	if insecureTransport.TLSClientConfig == nil || !insecureTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("insecure mode should skip TLS verification")
	}
}

func TestSummarize(t *testing.T) {
	summary := summarize(2, []requestResult{
		{status: 200, latency: 10 * time.Millisecond},
		{status: 204, latency: 20 * time.Millisecond},
		{status: 500, latency: 30 * time.Millisecond},
	}, time.Second)
	if summary.Total != 3 || summary.Success != 2 || summary.Failed != 1 {
		t.Fatalf("unexpected counts: %#v", summary)
	}
	if summary.RPS != 3 {
		t.Fatalf("unexpected rps: %f", summary.RPS)
	}
	if summary.SuccessRPS != 2 {
		t.Fatalf("unexpected success rps: %f", summary.SuccessRPS)
	}
	if summary.P50 != 20*time.Millisecond || summary.P95 != 30*time.Millisecond {
		t.Fatalf("unexpected percentiles: p50=%s p95=%s", summary.P50, summary.P95)
	}
}

func TestLoadTestConcurrency(t *testing.T) {
	t.Setenv("LOADTEST_CONCURRENCY", "")
	got, err := loadTestConcurrency()
	if err != nil {
		t.Fatal(err)
	}
	if got != 200 {
		t.Fatalf("default fixed concurrency mismatch: got=%d want=200", got)
	}
	t.Setenv("LOADTEST_CONCURRENCY", "250")
	got, err = loadTestConcurrency()
	if err != nil {
		t.Fatal(err)
	}
	if got != 250 {
		t.Fatalf("env fixed concurrency mismatch: got=%d want=250", got)
	}
	t.Setenv("LOADTEST_CONCURRENCY", "1,2")
	if _, err := loadTestConcurrency(); err == nil {
		t.Fatal("fixed concurrency should reject comma-separated levels")
	}
}

func TestDefaultLoadScenariosIncludeExactYggdrasilEndpoints(t *testing.T) {
	profileID := "load_profile_id"
	seed := loadSeed{
		User:           model.User{ID: "user-id", Email: "load-user@example.com"},
		Admin:          model.User{ID: "admin-id", Email: "load-admin@example.com"},
		YggUser:        model.User{ID: "ygg-user-id", Email: "load-ygg@example.com"},
		ProfileID:      profileID,
		ProfileName:    "LoadProfile",
		TextureHash:    "load_texture_hash",
		YggAccessToken: "access-token",
		YggClientToken: "client-token",
		YggServerID:    "server-id",
	}
	scenarios := defaultLoadScenarios(seed, "user_cookie=1", "admin_cookie=1", func(testing.TB) {})
	got := map[string]loadScenario{}
	for _, scenario := range scenarios {
		got[scenario.Name] = scenario
	}
	want := map[string]loadScenario{
		"ygg-metadata":     {Area: "Yggdrasil", Method: http.MethodGet, Path: "/"},
		"ygg-authenticate": {Area: "Yggdrasil", Method: http.MethodPost, Path: "/authserver/authenticate", Body: `{"username":"load-user@example.com","password":"Password123","requestUser":true}`},
		"ygg-validate":     {Area: "Yggdrasil", Method: http.MethodPost, Path: "/authserver/validate", Body: `{"accessToken":"access-token","clientToken":"client-token"}`},
		"ygg-profile":      {Area: "Yggdrasil", Method: http.MethodGet, Path: "/sessionserver/session/minecraft/profile/load_profile_id"},
		"ygg-lookup-name":  {Area: "Yggdrasil", Method: http.MethodGet, Path: "/api/users/profiles/minecraft/LoadProfile"},
		"ygg-has-joined":   {Area: "Yggdrasil", Method: http.MethodGet, Path: "/sessionserver/session/minecraft/hasJoined?username=LoadProfile&serverId=server-id"},
	}
	for name, expected := range want {
		actual, ok := got[name]
		if !ok {
			t.Fatalf("missing scenario %s in %#v", name, got)
		}
		if actual.Area != expected.Area || actual.Method != expected.Method || actual.Path != expected.Path || actual.Body != expected.Body {
			t.Fatalf("scenario %s mismatch:\n got: area=%q method=%q path=%q body=%q\nwant: area=%q method=%q path=%q body=%q",
				name, actual.Area, actual.Method, actual.Path, actual.Body, expected.Area, expected.Method, expected.Path, expected.Body)
		}
	}
	if got["me"].Cookie != "user_cookie=1" || got["admin-users"].Cookie != "admin_cookie=1" {
		t.Fatalf("authenticated scenario cookies mismatch: me=%q admin-users=%q", got["me"].Cookie, got["admin-users"].Cookie)
	}
	if got["ygg-has-joined"].Prepare == nil {
		t.Fatal("ygg-has-joined should refresh its pre-joined session before measurement")
	}
	if len(scenarios) != 21 {
		t.Fatalf("default scenario count mismatch: got=%d want=21", len(scenarios))
	}
}

func TestWriteLoadTestReportIncludesExactYggdrasilRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.md")
	results := []scenarioResult{
		{
			Scenario:    loadScenario{Area: "Yggdrasil", Name: "ygg-profile", Method: http.MethodGet, Path: "/sessionserver/session/minecraft/profile/load_profile_id"},
			Concurrency: 200,
			Summary: stepSummary{
				Concurrency: 200,
				Total:       300,
				Success:     300,
				Failed:      0,
				SuccessRPS:  299.5,
				RPS:         299.5,
				Avg:         2 * time.Millisecond,
				P50:         time.Millisecond,
				P95:         4 * time.Millisecond,
				P99:         5 * time.Millisecond,
				Statuses:    map[int]int{200: 300},
			},
		},
	}
	err := writeLoadTestReport(path, loadTestConfigValue{Duration: time.Second, MaxDBConns: 20}, 200, results)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report := string(data)
	for _, want := range []string{
		"- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites, 1 pre-joined Yggdrasil session",
		"- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup\n- Redis: real test Redis with isolated `elementskin:test:*` key prefix, cleaned by test cleanup",
		"| Yggdrasil | `ygg-profile` | `GET` | `/sessionserver/session/minecraft/profile/load_profile_id` |",
		"| Yggdrasil | `ygg-profile` | 200 | 300 | 300 | 0 | 0.00 | 299.5 | 299.5 | 2.0ms | 1.0ms | 4.0ms | 5.0ms | `200:300` | `` |",
		"- This report covers public, site, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("load-test report missing exact line:\n%s\n\nreport:\n%s", want, report)
		}
	}
}
