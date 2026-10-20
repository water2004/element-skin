package fallback_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbfallback "element-skin/backend/internal/database/fallback"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/testutil"
)

func TestFallbackHasJoinedForwardsAndWhitelist(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	var seenPath, seenUsername string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenUsername = r.URL.Query().Get("username")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"mock-uuid","name":"MockPlayer"}`))
	}))
	defer server.Close()

	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: server.URL, AccountURL: "a", ServicesURL: "s", CacheTTL: 60,
		EnableProfile: true, EnableHasJoined: true, EnableWhitelist: true, Note: "WhitelistedNode",
	}}); err != nil {
		t.Fatal(err)
	}
	eps, _ := db.Fallbacks.ListEndpoints(ctx)
	endpointID := eps[0]["id"].(int)
	fb := newFallback(db, server.Client())

	resp, err := fb.HasJoined(ctx, "Stranger", "sid", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Fatal("non-whitelisted user should not be forwarded")
	}
	if err := db.Fallbacks.AddWhitelistUser(ctx, "Stranger", endpointID); err != nil {
		t.Fatal(err)
	}
	resp, err = fb.HasJoined(ctx, "Stranger", "sid", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || resp.Status != 200 || string(resp.Body) != `{"id":"mock-uuid","name":"MockPlayer"}` {
		t.Fatalf("unexpected fallback response: %#v err=%v", resp, err)
	}
	if seenPath != "/session/minecraft/hasJoined" || seenUsername != "Stranger" {
		t.Fatalf("unexpected forwarded request path=%q username=%q", seenPath, seenUsername)
	}
}

func TestFallbackSkipsEndpointRequestAlreadyInFlight(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	calls := 0
	var fb fallbacksvc.Fallback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp, err := fb.HasJoined(r.Context(), r.URL.Query().Get("username"), r.URL.Query().Get("serverId"), r.URL.Query().Get("ip"))
		if err != nil {
			t.Fatalf("recursive fallback returned error: %v", err)
		}
		if resp != nil {
			t.Fatalf("recursive fallback should be skipped as in-flight duplicate, got %#v", resp)
		}
		w.WriteHeader(204)
	}))
	defer server.Close()

	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: server.URL, AccountURL: server.URL, ServicesURL: server.URL, CacheTTL: 60,
		EnableProfile: true, EnableHasJoined: true,
	}}); err != nil {
		t.Fatal(err)
	}
	fb = newFallback(db, server.Client())

	resp, err := fb.HasJoined(ctx, "LoopPlayer", "loop-server", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Fatalf("outer fallback should miss after remote 204, got %#v", resp)
	}
	if calls != 1 {
		t.Fatalf("fallback loop guard should allow exactly one outbound request, got %d", calls)
	}
	resp, err = fb.HasJoined(ctx, "LoopPlayer", "loop-server", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil || calls != 2 {
		t.Fatalf("completed request mark should be released for later attempts: resp=%#v calls=%d", resp, calls)
	}
}

func TestFallbackConcurrentIdenticalRequestsMakeOneOutboundCall(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		_, _ = w.Write([]byte(`{"id":"single-call","name":"ConcurrentPlayer"}`))
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: server.URL, AccountURL: server.URL, ServicesURL: server.URL, CacheTTL: 60,
		EnableProfile: true,
	}}); err != nil {
		t.Fatal(err)
	}
	fb := newFallback(db, server.Client())

	type result struct {
		body string
		err  error
	}
	const attempts = 12
	begin := make(chan struct{})
	results := make(chan result, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-begin
			response, err := fb.GetProfile(context.Background(), "concurrent-profile", true)
			if response == nil {
				results <- result{err: err}
				return
			}
			results <- result{body: string(response.Body), err: err}
		}()
	}
	close(begin)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("fallback request did not reach the outbound server")
	}

	for range attempts - 1 {
		select {
		case result := <-results:
			if result.err != nil || result.body != "" {
				t.Fatalf("duplicate in-flight request result=%#v; want nil miss", result)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("duplicate fallback calls did not return while the single outbound request was blocked; calls=%d", calls.Load())
		}
	}
	close(release)
	wg.Wait()
	final := <-results
	if final.err != nil || final.body != `{"id":"single-call","name":"ConcurrentPlayer"}` {
		t.Fatalf("winning fallback response=%#v; want exact remote payload", final)
	}
	if calls.Load() != 1 {
		t.Fatalf("concurrent identical fallback requests made %d outbound calls; want exactly 1", calls.Load())
	}
}

func TestFallbackLookupRoutesForwardExactRequests(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		switch r.URL.EscapedPath() {
		case "/users/profiles/minecraft/Name%20With%20Space":
			_, _ = w.Write([]byte(`{"id":"account-id","name":"Name With Space"}`))
		case "/minecraft/profile/lookup/name/Name%20With%20Space":
			_, _ = w.Write([]byte(`{"id":"services-id","name":"Name With Space"}`))
		case "/profiles/minecraft":
			var names []string
			if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
				t.Fatalf("decode bulk body: %v", err)
			}
			if len(names) != 2 || names[0] != "Alex" || names[1] != "Steve" {
				t.Fatalf("unexpected bulk names: %#v", names)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("bulk lookup should send JSON content type, got %q", r.Header.Get("Content-Type"))
			}
			_, _ = w.Write([]byte(`[{"id":"alex-id","name":"Alex"},{"id":"steve-id","name":"Steve"}]`))
		default:
			t.Fatalf("unexpected fallback request: %s", r.URL.String())
		}
	}))
	defer server.Close()

	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: server.URL, AccountURL: server.URL, ServicesURL: server.URL, CacheTTL: 60,
		EnableProfile: true, EnableHasJoined: true,
	}}); err != nil {
		t.Fatal(err)
	}
	fb := newFallback(db, server.Client())

	account, err := fb.GetProfileByName(ctx, "Name With Space")
	if err != nil || account == nil || string(account.Body) != `{"id":"account-id","name":"Name With Space"}` {
		t.Fatalf("account lookup got resp=%#v err=%v", account, err)
	}
	services, err := fb.ServicesLookup(ctx, "Name With Space")
	if err != nil || services == nil || string(services.Body) != `{"id":"services-id","name":"Name With Space"}` {
		t.Fatalf("services lookup got resp=%#v err=%v", services, err)
	}
	bulk, err := fb.BulkLookup(ctx, []string{"Alex", "Steve"}, fallbacksvc.AccountAPI)
	if err != nil {
		t.Fatal(err)
	}
	if len(bulk) != 2 || bulk[0]["id"] != "alex-id" || bulk[1]["name"] != "Steve" {
		t.Fatalf("unexpected bulk response: %#v", bulk)
	}
	want := []string{
		"GET /users/profiles/minecraft/Name%20With%20Space",
		"GET /minecraft/profile/lookup/name/Name%20With%20Space",
		"POST /profiles/minecraft",
	}
	if len(seen) != len(want) {
		t.Fatalf("unexpected requests: %#v", seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("request %d got %q want %q; all=%#v", i, seen[i], want[i], seen)
		}
	}
}

func TestFallbackLookupNamesMergesLocalAndFallbackProfilesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "fallback-lookup-names@test.com", "Password123", "FallbackLookupNames", false)
	local := testutil.CreateProfile(t, db, user.ID, "fallback_lookup_local", "LocalLookup")
	fallbackCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		if r.Method != http.MethodPost || r.URL.Path != "/profiles/minecraft" {
			t.Fatalf("fallback lookup names request=%s %s; want POST /profiles/minecraft", r.Method, r.URL.Path)
		}
		var names []string
		if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
			t.Fatalf("decode fallback lookup names body: %v", err)
		}
		if len(names) != 2 || names[0] != "RemoteLookup" || names[1] != "remoteSecond" {
			t.Fatalf("fallback lookup names body=%#v; want only missing names in request order", names)
		}
		_, _ = w.Write([]byte(`[{"id":"remote-lookup","name":"RemoteLookup"},{"id":"remote-second","name":"remoteSecond"}]`))
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority:      1,
		SessionURL:    server.URL,
		AccountURL:    server.URL,
		ServicesURL:   server.URL,
		CacheTTL:      60,
		EnableProfile: true,
	}}); err != nil {
		t.Fatal(err)
	}
	fb := newFallback(db, server.Client())

	got, err := fb.LookupNames(ctx, []string{"RemoteLookup", "LocalLookup", "remoteSecond", "LOCALLOOKUP"}, fallbacksvc.AccountAPI)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"id": local.ID, "name": "LocalLookup"},
		{"id": "remote-lookup", "name": "RemoteLookup"},
		{"id": "remote-second", "name": "remoteSecond"},
	}
	if len(got) != len(want) || fallbackCalls != 1 {
		t.Fatalf("lookup names result count/calls mismatch: got=%#v calls=%d", got, fallbackCalls)
	}
	for i := range want {
		if got[i]["id"] != want[i]["id"] || got[i]["name"] != want[i]["name"] {
			t.Fatalf("lookup names item %d=%#v want %#v; all=%#v", i, got[i], want[i], got)
		}
	}

	got, err = fb.LookupNames(ctx, []string{"LocalLookup"}, fallbacksvc.AccountAPI)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != local.ID || got[0]["name"] != "LocalLookup" || fallbackCalls != 1 {
		t.Fatalf("local-only lookup should not call fallback: got=%#v calls=%d", got, fallbackCalls)
	}
}

func TestFallbackSerialFailoverSkipsEmptyURLsAndRejectsMalformedBulk(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	fb := newFallback(db, nil)

	if resp, err := fb.HasJoined(ctx, "Missing", "server", ""); err != nil || resp != nil {
		t.Fatalf("unconfigured hasJoined fallback=%#v err=%v; want nil, nil", resp, err)
	}
	if resp, err := fb.GetProfile(ctx, "missing", true); err != nil || resp != nil {
		t.Fatalf("unconfigured profile fallback=%#v err=%v; want nil, nil", resp, err)
	}
	if resp, err := fb.GetProfileByName(ctx, "Missing"); err != nil || resp != nil {
		t.Fatalf("unconfigured account fallback=%#v err=%v; want nil, nil", resp, err)
	}
	if resp, err := fb.ServicesLookup(ctx, "Missing"); err != nil || resp != nil {
		t.Fatalf("unconfigured services fallback=%#v err=%v; want nil, nil", resp, err)
	}
	if profiles, err := fb.BulkLookup(ctx, []string{"Missing"}, fallbacksvc.AccountAPI); err != nil || profiles != nil {
		t.Fatalf("unconfigured bulk fallback=%#v err=%v; want nil, nil", profiles, err)
	}

	var failedCalls, successCalls atomic.Int32
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		failedCalls.Add(1)
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	defer failed.Close()
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		successCalls.Add(1)
		switch req.URL.Path {
		case "/session/minecraft/profile/failover-profile":
			_, _ = w.Write([]byte(`{"id":"failover-profile","name":"Failover"}`))
		case "/users/profiles/minecraft/AccountName":
			_, _ = w.Write([]byte(`{"id":"account-id","name":"AccountName"}`))
		case "/minecraft/profile/lookup/name/ServicesName":
			_, _ = w.Write([]byte(`{"id":"services-id","name":"ServicesName"}`))
		case "/profiles/minecraft":
			_, _ = w.Write([]byte(`not-json`))
		default:
			t.Fatalf("unexpected success fallback path: %s", req.URL.Path)
		}
	}))
	defer success.Close()
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{
		{
			Priority: 1, SessionURL: failed.URL, AccountURL: "", ServicesURL: "",
			CacheTTL: 60, EnableProfile: true,
		},
		{
			Priority: 2, SessionURL: success.URL, AccountURL: success.URL, ServicesURL: success.URL,
			CacheTTL: 60, EnableProfile: true,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fb = newFallback(db, success.Client())

	profile, err := fb.GetProfile(ctx, "failover-profile", true)
	if err != nil || profile == nil || string(profile.Body) != `{"id":"failover-profile","name":"Failover"}` {
		t.Fatalf("serial profile failover=%#v err=%v; want exact second-endpoint response", profile, err)
	}
	account, err := fb.GetProfileByName(ctx, "AccountName")
	if err != nil || account == nil || string(account.Body) != `{"id":"account-id","name":"AccountName"}` {
		t.Fatalf("empty-account URL failover=%#v err=%v; want exact second-endpoint response", account, err)
	}
	services, err := fb.ServicesLookup(ctx, "ServicesName")
	if err != nil || services == nil || string(services.Body) != `{"id":"services-id","name":"ServicesName"}` {
		t.Fatalf("empty-services URL failover=%#v err=%v; want exact second-endpoint response", services, err)
	}
	profiles, err := fb.BulkLookup(ctx, []string{"Malformed"}, fallbacksvc.AccountAPI)
	var syntaxErr *json.SyntaxError
	if profiles != nil || !errors.As(err, &syntaxErr) || syntaxErr.Offset != 2 {
		t.Fatalf("malformed bulk response=%#v err=%#v; want nil and JSON syntax error at offset 2", profiles, err)
	}
	if failedCalls.Load() != 1 || successCalls.Load() != 4 {
		t.Fatalf("fallback calls: failed=%d success=%d; want 1 profile failure and 4 successful-endpoint calls",
			failedCalls.Load(), successCalls.Load())
	}
}
