package probe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/service/probe"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

type probeRoute struct {
	Method  string
	Path    string
	Status  int
	Calls   int32
}

func TestProbeRunMarksUpFor200And404AndDownOtherwise(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	// endpoint 1: session 200, account 404, services 500
	ep1Routes := map[string]int{
		"/session/minecraft/profile/" + probe.TestUUID:        http.StatusOK,
		"/users/profiles/minecraft/" + probe.TestName:          http.StatusNotFound,
		"/minecraft/profile/lookup/name/" + probe.TestName:     http.StatusInternalServerError,
	}
	server1 := newCountingServer(t, ep1Routes)
	defer server1.Close()
	// endpoint 2: every API returns 200
	ep2Routes := map[string]int{
		"/session/minecraft/profile/" + probe.TestUUID:    http.StatusOK,
		"/users/profiles/minecraft/" + probe.TestName:     http.StatusOK,
		"/minecraft/profile/lookup/name/" + probe.TestName: http.StatusOK,
	}
	server2 := newCountingServer(t, ep2Routes)
	defer server2.Close()

	stg := settings.Settings{DB: db, Redis: redis}
	if err := stg.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{
			map[string]any{
				"priority":     1,
				"session_url":  server1.URL,
				"account_url":  server1.URL,
				"services_url": server1.URL,
				"note":         "first",
			},
			map[string]any{
				"priority":     2,
				"session_url":  server2.URL,
				"account_url":  server2.URL,
				"services_url": server2.URL,
				"note":         "second",
			},
		},
	}); err != nil {
		t.Fatalf("save fallback: %v", err)
	}

	svc := probe.New(db, redis)
	checkedAt := time.Now()
	svc.Now = func() time.Time { return checkedAt }
	if err := svc.Run(ctx); err != nil {
		t.Fatalf("run probe: %v", err)
	}

	samples, err := redis.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("expected one sample per endpoint, got %d: %#v", len(samples), samples)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].EndpointID < samples[j].EndpointID })

	first := samples[0]
	if first.Note != "first" || first.Session != probe.StatusUp || first.Account != probe.StatusUp || first.Services != probe.StatusDown {
		t.Fatalf("first endpoint should treat 200 and 404 as up but 500 as down: %#v", first)
	}
	second := samples[1]
	if second.Note != "second" || second.Session != probe.StatusUp || second.Account != probe.StatusUp || second.Services != probe.StatusUp {
		t.Fatalf("second endpoint should be all up: %#v", second)
	}
	expectedCheckedAt := checkedAt.UnixMilli()
	if first.CheckedAt != expectedCheckedAt || second.CheckedAt != expectedCheckedAt {
		t.Fatalf("checked_at should use injected clock: first=%d second=%d expected=%d", first.CheckedAt, second.CheckedAt, expectedCheckedAt)
	}

	// Each server should have been hit exactly three times (once per API).
	if got := atomic.LoadInt32(&server1.calls); got != 3 {
		t.Fatalf("first server should be probed once per API: got %d", got)
	}
	if got := atomic.LoadInt32(&server2.calls); got != 3 {
		t.Fatalf("second server should be probed once per API: got %d", got)
	}
}

func TestProbeRunMarksConnectionFailuresAsDown(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	stg := settings.Settings{DB: db, Redis: redis}
	if err := stg.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{
			map[string]any{
				"priority":     1,
				"session_url":  "http://127.0.0.1:1",
				"account_url":  "http://127.0.0.1:1",
				"services_url": "http://127.0.0.1:1",
				"note":         "unreachable",
			},
		},
	}); err != nil {
		t.Fatalf("save fallback: %v", err)
	}

	svc := probe.New(db, redis)
	svc.Client = &http.Client{Timeout: 200 * time.Millisecond}
	if err := svc.Run(ctx); err != nil {
		t.Fatalf("run probe: %v", err)
	}
	samples, err := redis.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected one sample, got %d", len(samples))
	}
	s := samples[0]
	if s.Session != probe.StatusDown || s.Account != probe.StatusDown || s.Services != probe.StatusDown {
		t.Fatalf("connection failures should record down: %#v", s)
	}
}

func TestProbeReadIntervalRespectsBoundsAndDefault(t *testing.T) {
	ctx := context.Background()

	if got := probe.ReadInterval(ctx, nil); got != 10*time.Minute {
		t.Fatalf("nil reader should return default 10m, got %s", got)
	}

	cases := []struct {
		raw      string
		expected time.Duration
	}{
		{"600", 10 * time.Minute},
		{"30", time.Minute},     // below the floor → clamp to minInterval
		{"-50", 10 * time.Minute}, // negative → fall back to default
		{"abc", 10 * time.Minute},
		{"3600", time.Hour},
	}
	for _, c := range cases {
		got := probe.ReadInterval(ctx, fakeIntervalReader{value: c.raw})
		if got != c.expected {
			t.Fatalf("ReadInterval(%q) = %s, want %s", c.raw, got, c.expected)
		}
	}
}

func TestProbeRunWithNoEndpointsLeavesHistoryEmpty(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	svc := probe.New(db, redis)
	if err := svc.Run(ctx); err != nil {
		t.Fatalf("run probe: %v", err)
	}
	samples, err := redis.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("expected empty history, got %d", len(samples))
	}
}

func TestProbeRunRetainsHistoryWindow(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	server := newCountingServer(t, map[string]int{
		"/session/minecraft/profile/" + probe.TestUUID:    http.StatusOK,
		"/users/profiles/minecraft/" + probe.TestName:     http.StatusOK,
		"/minecraft/profile/lookup/name/" + probe.TestName: http.StatusOK,
	})
	defer server.Close()

	stg := settings.Settings{DB: db, Redis: redis}
	if err := stg.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{
			map[string]any{
				"priority":     1,
				"session_url":  server.URL,
				"account_url":  server.URL,
				"services_url": server.URL,
				"note":         "first",
			},
		},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	svc := probe.New(db, redis)
	svc.Retention = time.Hour
	now := time.Now()
	clock := now.Add(-3 * time.Hour)
	svc.Now = func() time.Time { return clock }
	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	clock = now
	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	samples, err := redis.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("retention should drop the 3h-old sample, got %d: %#v", len(samples), samples)
	}
	if samples[0].CheckedAt != now.UnixMilli() {
		t.Fatalf("retained sample should be the recent one: %#v", samples[0])
	}
}

type fakeIntervalReader struct {
	value string
}

func (f fakeIntervalReader) Get(_ context.Context, _, _ string) (string, error) {
	return f.value, nil
}

type countingServer struct {
	*httptest.Server
	calls int32
}

func newCountingServer(t *testing.T, statuses map[string]int) *countingServer {
	t.Helper()
	s := &countingServer{}
	mux := http.NewServeMux()
	for path, code := range statuses {
		path := path
		code := code
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&s.calls, 1)
			w.WriteHeader(code)
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	})
	s.Server = httptest.NewServer(mux)
	if _, err := url.Parse(s.URL); err != nil {
		t.Fatalf("server URL parse: %v", err)
	}
	return s
}
