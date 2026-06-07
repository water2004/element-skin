package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service"
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

	if err := db.SaveFallbackEndpoints(ctx, []database.FallbackEndpoint{{
		Priority: 1, SessionURL: server.URL, AccountURL: "a", ServicesURL: "s", CacheTTL: 60,
		EnableProfile: true, EnableHasJoined: true, EnableWhitelist: true, Note: "WhitelistedNode",
	}}); err != nil {
		t.Fatal(err)
	}
	eps, _ := db.ListFallbackEndpoints(ctx)
	endpointID := eps[0]["id"].(int)
	fb := service.Fallback{DB: db, Client: server.Client()}

	resp, err := fb.HasJoined(ctx, "Stranger", "sid", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Fatal("non-whitelisted user should not be forwarded")
	}
	if err := db.AddWhitelistUser(ctx, "Stranger", endpointID); err != nil {
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

func TestFallbackParallelReturnsFastSuccess(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(404)
	}))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"fast":true}`))
	}))
	defer fast.Close()
	if err := db.SetSetting(ctx, "fallback_strategy", "parallel"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFallbackEndpoints(ctx, []database.FallbackEndpoint{
		{Priority: 1, SessionURL: slow.URL, AccountURL: "a", ServicesURL: "s", CacheTTL: 60, EnableProfile: true, EnableHasJoined: true},
		{Priority: 2, SessionURL: fast.URL, AccountURL: "a", ServicesURL: "s", CacheTTL: 60, EnableProfile: true, EnableHasJoined: true},
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := (service.Fallback{DB: db, Client: fast.Client()}).GetProfile(ctx, "some-uuid", true)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || string(resp.Body) != `{"fast":true}` {
		t.Fatalf("unexpected fallback profile response: %#v", resp)
	}
}
