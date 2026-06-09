package fallback_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	bulk, err := fb.BulkLookup(ctx, []string{"Alex", "Steve"})
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
