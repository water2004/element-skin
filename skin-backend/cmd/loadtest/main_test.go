package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
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
	if _, err := parseConcurrency(" , "); err == nil || err.Error() != "at least one concurrency level is required" {
		t.Fatalf("empty concurrency error=%v; want exact required-level error", err)
	}
}

func TestParseFlagsAppliesEveryCommandLineOverrideExactly(t *testing.T) {
	oldCommandLine := flag.CommandLine
	oldArgs := os.Args
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
		os.Args = oldArgs
	})
	flag.CommandLine = flag.NewFlagSet("loadtest-test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{
		"loadtest",
		"-target=https://backend.example/base",
		"-path=/v1/admin/users?limit=7",
		"-method=patch",
		`-body={"name":"Updated"}`,
		"-content-type=application/v1/users/merge-patch+json",
		"-concurrency=2,4,8",
		"-duration=3s",
		"-timeout=750ms",
		"-fail-threshold=2.5",
		"-max-p95=125ms",
		"-login-email=load@example.com",
		"-login-password=Secret123",
		"-login-path=/custom-login",
		"-cookie=access=abc; refresh=def",
		"-insecure=true",
	}

	got := parseFlags()
	want := options{
		target:          "https://backend.example/base",
		path:            "/v1/admin/users?limit=7",
		method:          "patch",
		body:            `{"name":"Updated"}`,
		contentType:     "application/v1/users/merge-patch+json",
		concurrencyList: "2,4,8",
		duration:        3 * time.Second,
		timeout:         750 * time.Millisecond,
		failThreshold:   2.5,
		maxP95:          125 * time.Millisecond,
		loginEmail:      "load@example.com",
		loginPassword:   "Secret123",
		loginPath:       "/custom-login",
		cookieHeader:    "access=abc; refresh=def",
		insecureTLS:     true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed options mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestRunReturnsExactConfigurationAndLoginErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opts       options
		wantCode   int
		wantStderr string
	}{
		{
			name:       "invalid concurrency",
			opts:       options{concurrencyList: "1,bad"},
			wantCode:   2,
			wantStderr: "invalid concurrency level \"bad\"\n",
		},
		{
			name:       "invalid target",
			opts:       options{concurrencyList: "1", target: "missing-scheme", path: "/probe"},
			wantCode:   2,
			wantStderr: "target must include scheme and host\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.opts, &stdout, &stderr)
			if code != tc.wantCode || stdout.Len() != 0 || stderr.String() != tc.wantStderr {
				t.Fatalf("run result: code=%d stdout=%q stderr=%q; want code=%d empty stdout stderr=%q",
					code, stdout.String(), stderr.String(), tc.wantCode, tc.wantStderr)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := run(options{
		target:          server.URL,
		path:            "/probe",
		method:          http.MethodGet,
		concurrencyList: "1",
		duration:        time.Millisecond,
		timeout:         time.Second,
		loginEmail:      "user@example.com",
		loginPassword:   "wrong",
		loginPath:       "/login",
	}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || stderr.String() != "login: unexpected status 401\n" {
		t.Fatalf("login failure: code=%d stdout=%q stderr=%q; want exact login exit", code, stdout.String(), stderr.String())
	}
}

func TestRunReportsSuccessfulAndFailedCapacityExactly(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		maxP95         time.Duration
		wantSuggestion string
		wantStatus     string
	}{
		{
			name:           "successful capacity",
			status:         http.StatusNoContent,
			maxP95:         time.Hour,
			wantSuggestion: "Suggested capacity: 1 concurrent requests under the configured threshold.",
			wantStatus:     "204:",
		},
		{
			name:           "no acceptable capacity",
			status:         http.StatusInternalServerError,
			wantSuggestion: "Suggested capacity: none of the tested levels met the configured threshold.",
			wantStatus:     "500:",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPost || req.URL.Path != "/api/probe" ||
					req.Header.Get("Content-Type") != "application/json" ||
					req.Header.Get("Cookie") != "session=load" {
					t.Fatalf("run request mismatch: method=%s path=%s content-type=%q cookie=%q",
						req.Method, req.URL.Path, req.Header.Get("Content-Type"), req.Header.Get("Cookie"))
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatal(err)
				}
				if string(body) != `{"probe":true}` {
					t.Fatalf("run request body=%q; want exact probe JSON", body)
				}
				calls.Add(1)
				w.WriteHeader(tc.status)
				if tc.status >= 400 {
					_, _ = w.Write([]byte("probe failed"))
				}
			}))
			defer server.Close()

			var stdout, stderr bytes.Buffer
			code := run(options{
				target:          server.URL + "/api",
				path:            "/probe",
				method:          http.MethodPost,
				body:            `{"probe":true}`,
				contentType:     "application/json",
				concurrencyList: "1",
				duration:        20 * time.Millisecond,
				timeout:         time.Second,
				failThreshold:   0,
				maxP95:          tc.maxP95,
				cookieHeader:    "session=load",
			}, &stdout, &stderr)
			output := stdout.String()
			if code != 0 || stderr.Len() != 0 || calls.Load() <= 0 ||
				!strings.Contains(output, "Target: POST "+server.URL+"/api/probe\n") ||
				!strings.Contains(output, "failure threshold: 0.00%") ||
				!strings.Contains(output, tc.wantStatus) ||
				!strings.Contains(output, tc.wantSuggestion) {
				t.Fatalf("run output mismatch: code=%d calls=%d stderr=%q stdout=%q",
					code, calls.Load(), stderr.String(), output)
			}
			if tc.maxP95 > 0 && !strings.Contains(output, "p95 threshold: 1h0m0s") {
				t.Fatalf("run output missing exact p95 threshold: %q", output)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	got, err := buildURL("http://127.0.0.1:8000/api", "/v1/public/settings")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8000/api/v1/public/settings" {
		t.Fatalf("unexpected URL: %s", got)
	}
	got, err = buildURL("http://127.0.0.1:8000/api", "/v1/admin/users?limit=20&q=Load")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8000/api/v1/admin/users?limit=20&q=Load" {
		t.Fatalf("query string should stay as query, got: %s", got)
	}
	got, err = buildURL("http://ignored", "https://example.com/v1/users/me")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/v1/users/me" {
		t.Fatalf("absolute URL should pass through: %s", got)
	}
	if _, err := buildURL("127.0.0.1:8000", "/v1/users/me"); err == nil {
		t.Fatal("target without scheme should fail")
	}
	if _, err := buildURL("http://127.0.0.1:8000", "://bad path"); err == nil {
		t.Fatal("invalid request path should fail")
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
	if summary.FirstError != "status 500" || summary.Statuses[200] != 1 || summary.Statuses[204] != 1 || summary.Statuses[500] != 1 {
		t.Fatalf("unexpected failure/status summary: %#v", summary)
	}
}

func TestLoginSendsExactCredentialsAndReturnsCookieHeader(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if req.Method != http.MethodPost || req.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("login request=%s %s; want POST /api/v1/auth/login", req.Method, req.URL.Path)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("login content type=%q; want application/json", req.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"email":"load@example.com","password":"Secret123"}` {
			t.Fatalf("login body=%q; want exact credentials JSON", body)
		}
		http.SetCookie(w, &http.Cookie{Name: "access", Value: "access-value"})
		http.SetCookie(w, &http.Cookie{Name: "refresh", Value: "refresh-value"})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cookies, err := login(server.Client(), server.URL+"/api", "/v1/auth/login", "load@example.com", "Secret123")
	if err != nil || cookies != "access=access-value; refresh=refresh-value" {
		t.Fatalf("login cookies=%q err=%v; want exact joined cookie header", cookies, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("login calls=%d; want exactly 1", calls.Load())
	}
}

func TestLoginRejectsStatusAndMissingCookiesExactly(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		wantDetail string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, wantDetail: "unexpected status 401"},
		{name: "success without cookies", status: http.StatusOK, wantDetail: "login succeeded without cookies"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			cookies, err := login(server.Client(), server.URL, "/login", "user@example.com", "Password123")
			if cookies != "" || err == nil || err.Error() != tc.wantDetail {
				t.Fatalf("login result cookies=%q err=%v; want empty cookies and %q", cookies, err, tc.wantDetail)
			}
		})
	}
}

func TestDoRequestForwardsExactRequestAndClassifiesResponse(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		call := calls.Add(1)
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != http.MethodPatch ||
			req.Header.Get("Content-Type") != "application/v1/users/merge-patch+json" ||
			req.Header.Get("Cookie") != "access=abc; refresh=def" ||
			string(body) != `{"name":"Updated"}` {
			t.Fatalf("request mismatch: method=%s content-type=%q cookie=%q body=%q",
				req.Method, req.Header.Get("Content-Type"), req.Header.Get("Cookie"), body)
		}
		if call == 1 {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("ignored success body"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 600)))
	}))
	defer server.Close()
	opts := options{
		method:      http.MethodPatch,
		body:        `{"name":"Updated"}`,
		contentType: "application/v1/users/merge-patch+json",
	}

	success := doRequest(server.Client(), server.URL, opts, "access=abc; refresh=def")
	if success.err != nil || success.status != http.StatusAccepted || success.detail != "" {
		t.Fatalf("successful request result=%#v; want exact 202 success without detail", success)
	}
	failed := doRequest(server.Client(), server.URL, opts, "access=abc; refresh=def")
	if failed.err != nil || failed.status != http.StatusInternalServerError || len(failed.detail) != 512 ||
		failed.detail != strings.Repeat("x", 512) {
		t.Fatalf("failed request result=%#v; want exact 500 and first 512 response bytes", failed)
	}
}

func TestDoRequestReturnsConstructionAndTransportErrorsExactly(t *testing.T) {
	malformed := doRequest(http.DefaultClient, "://bad-url", options{method: http.MethodGet}, "")
	if malformed.err == nil || malformed.status != 0 || malformed.detail != "" {
		t.Fatalf("malformed request result=%#v; want construction error only", malformed)
	}

	wantErr := errors.New("transport unavailable")
	client := &http.Client{Transport: loadTestRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	})}
	failed := doRequest(client, "https://example.test/load", options{method: http.MethodGet}, "")
	const wantDetail = `Get "https://example.test/load": transport unavailable`
	if !errors.Is(failed.err, wantErr) || failed.status != 0 || failed.detail != wantDetail {
		t.Fatalf("transport error result=%#v; want exact transport failure", failed)
	}
}

func TestRunStepProducesExactSuccessfulSummaryInvariants(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.Header.Get("Cookie") != "session=load" {
			t.Fatalf("load request mismatch: method=%s cookie=%q", req.Method, req.Header.Get("Cookie"))
		}
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	summary := runStep(
		server.Client(),
		server.URL,
		options{method: http.MethodGet, duration: 25 * time.Millisecond},
		"session=load",
		3,
	)
	if summary.Concurrency != 3 || summary.Total <= 0 || summary.Total != int(calls.Load()) ||
		summary.Success != summary.Total || summary.Failed != 0 || summary.FailurePct != 0 ||
		summary.Statuses[http.StatusNoContent] != summary.Total || summary.FirstError != "" ||
		summary.Wall < 25*time.Millisecond || summary.RPS <= 0 || summary.SuccessRPS != summary.RPS {
		t.Fatalf("successful step summary violates exact invariants: %#v calls=%d", summary, calls.Load())
	}
}

func TestSummarizeFailurePriorityEmptyInputAndFormatting(t *testing.T) {
	summary := summarize(4, []requestResult{
		{status: http.StatusBadGateway, latency: 3 * time.Millisecond, detail: "upstream unavailable"},
		{latency: 5 * time.Millisecond, err: errors.New("dial failed"), detail: "dial failed"},
	}, 2*time.Second)
	if summary.Total != 2 || summary.Success != 0 || summary.Failed != 2 || summary.FailurePct != 100 ||
		summary.RPS != 1 || summary.SuccessRPS != 0 || summary.Avg != 4*time.Millisecond ||
		summary.P50 != 3*time.Millisecond || summary.P95 != 5*time.Millisecond ||
		summary.FirstError != "upstream unavailable" || !reflect.DeepEqual(summary.Statuses, map[int]int{502: 1}) {
		t.Fatalf("failure summary mismatch: %#v", summary)
	}
	empty := summarize(1, nil, 0)
	if empty.Total != 0 || empty.RPS != 0 || empty.Avg != 0 || empty.P50 != 0 ||
		empty.FirstError != "" || len(empty.Statuses) != 0 {
		t.Fatalf("empty summary mismatch: %#v", empty)
	}
	if got := percentile([]time.Duration{time.Millisecond, 2 * time.Millisecond}, -10); got != time.Millisecond {
		t.Fatalf("negative percentile=%s; want first sample", got)
	}
	if got := percentile([]time.Duration{time.Millisecond, 2 * time.Millisecond}, 200); got != 2*time.Millisecond {
		t.Fatalf("oversized percentile=%s; want last sample", got)
	}
	if formatDuration(0) != "-" || formatDuration(1500*time.Microsecond) != "1.5ms" ||
		formatDuration(1500*time.Millisecond) != "1.50s" {
		t.Fatalf("duration formatting mismatch: zero=%q ms=%q sec=%q",
			formatDuration(0), formatDuration(1500*time.Microsecond), formatDuration(1500*time.Millisecond))
	}
	if formatStatuses(nil) != "errors-only" ||
		formatStatuses(map[int]int{500: 2, 200: 3}) != "200:3,500:2" {
		t.Fatalf("status formatting mismatch: empty=%q values=%q",
			formatStatuses(nil), formatStatuses(map[int]int{500: 2, 200: 3}))
	}
}

type loadTestRoundTripFunc func(*http.Request) (*http.Response, error)

func (f loadTestRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
