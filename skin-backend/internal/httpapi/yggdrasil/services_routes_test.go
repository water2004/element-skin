package yggdrasil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

var servicesBulkPaths = []string{
	"/minecraftservices/minecraft/profile/lookup/bulk/byname",
	"/api/minecraft/profile/lookup/bulk/byname",
	"/minecraft/profile/lookup/bulk/byname",
}

func TestServicesAndProxyLookupRoutesPreserveDisplayNames(t *testing.T) {
	db, router := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "services@test.com", "Password123", "Services", false)
	p := testutil.CreateProfile(t, db, user.ID, "123456781234123412341234567890ab", "CasePlayer")
	want := `{"id":"` + p.ID + `","name":"CasePlayer"}`
	for _, prefix := range []string{
		"/minecraftservices/minecraft/profile/lookup/name/",
		"/api/minecraft/profile/lookup/name/",
		"/minecraft/profile/lookup/name/",
		"/api/users/profiles/minecraft/",
		"/users/profiles/minecraft/",
		"/api/profiles/minecraft/",
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, prefix+"caseplayer", nil))
		if rec.Code != 200 || rec.Body.String() != want+"\n" {
			t.Fatalf("%s status=%d body=%q", prefix, rec.Code, rec.Body.String())
		}
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, prefix+"Missing", nil))
		wantCode, wantBody := 204, ""
		if strings.Contains(prefix, "/lookup/") {
			wantCode, wantBody = 404, "{\"error\":\"NOT_FOUND\",\"errorMessage\":\"Profile not found.\"}\n"
		}
		if rec.Code != wantCode || rec.Body.String() != wantBody {
			t.Fatalf("%s miss status=%d body=%q", prefix, rec.Code, rec.Body.String())
		}
	}
	// authlib-injector can rewrite the services URL or convert it to the account bulk URL.
	for _, path := range append(append([]string{}, servicesBulkPaths...), "/api/profiles/minecraft") {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`["caseplayer","CasePlayer","CASEPLAYER","Missing"]`)))
		if rec.Code != 200 || rec.Body.String() != "["+want+"]\n" {
			t.Fatalf("%s status=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}
	legacy := httptest.NewRecorder()
	router.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/publickeys", nil))
	for _, path := range []string{"/minecraftservices/publickeys", "/minecraftservices/publickeys/"} {
		services := httptest.NewRecorder()
		router.ServeHTTP(services, httptest.NewRequest(http.MethodGet, path, nil))
		if services.Code != 200 || legacy.Code != 200 || services.Body.String() != legacy.Body.String() ||
			!reflect.DeepEqual(services.Header(), legacy.Header()) {
			t.Fatalf("public keys %s differ: services=%d %v %s legacy=%d %v %s", path, services.Code, services.Header(), services.Body, legacy.Code, legacy.Header(), legacy.Body)
		}
	}
}

func TestServicesBulkValidationReturnsExactProtocolErrors(t *testing.T) {
	_, router := testutil.NewTestApp(t)
	for _, tc := range []struct {
		body, message string
	}{
		{`{}`, "Expected an array of profile names."},
		{`[`, "Expected an array of profile names."},
		{`["A"] []`, "Expected an array of profile names."},
		{`null`, "Invalid number of profile names."},
		{`[]`, "Invalid number of profile names."},
		{`[""]`, "Profile names must not be empty."},
		{`[null]`, "Profile names must not be empty."},
		{`["A","A","A","A","A","A","A","A","A","A","A"]`, "Invalid number of profile names."},
	} {
		for _, path := range servicesBulkPaths {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(tc.body)))
			want := `{"error":"IllegalArgumentException","errorMessage":"` + tc.message + "\"}\n"
			if rec.Code != 400 || rec.Body.String() != want {
				t.Fatalf("%s body=%s status=%d response=%q", path, tc.body, rec.Code, rec.Body.String())
			}
		}
	}
	for _, body := range []string{`["A"]`, `["A","B","C","D","E","F","G","H","I","J"]`} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, servicesBulkPaths[0], strings.NewReader(body)))
		if rec.Code != 200 || rec.Body.String() != "[]\n" {
			t.Fatalf("boundary body=%s status=%d response=%q", body, rec.Code, rec.Body.String())
		}
	}
}

func TestServicesLookupUsesServicesUpstreamAndDeduplicatesMissingNames(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	user := testutil.CreateUser(t, db, "services-fallback@test.com", "Password123", "ServicesFallback", false)
	local := testutil.CreateProfile(t, db, user.ID, "local-uuid", "LocalPlayer")
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.Method {
		case http.MethodPost:
			var names []string
			if err := json.NewDecoder(r.Body).Decode(&names); err != nil || !reflect.DeepEqual(names, []string{"RemotePlayer"}) {
				t.Errorf("upstream names=%v err=%v", names, err)
			}
			_, _ = w.Write([]byte(`[{"id":"remote-uuid","name":"RemotePlayer"}]`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"remote-uuid","name":"RemotePlayer"}`))
		}
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(t.Context(), []dbfallback.Endpoint{{
		Priority: 1, SessionURL: server.URL + "/session", AccountURL: server.URL + "/account",
		ServicesURL: server.URL + "/services", CacheTTL: 60, EnableProfile: true,
	}}); err != nil {
		t.Fatal(err)
	}
	h := yggdrasil.NewWithHTTPClient(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg}, server.Client())
	for _, path := range append(append([]string{}, servicesBulkPaths...), "/api/profiles/minecraft") {
		rec := httptest.NewRecorder()
		h.LookupNames(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`["localplayer","RemotePlayer","remoteplayer"]`)))
		want := `[{"id":"` + local.ID + `","name":"LocalPlayer"},{"id":"remote-uuid","name":"RemotePlayer"}]` + "\n"
		if rec.Code != 200 || rec.Body.String() != want {
			t.Fatalf("bulk %s status=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/minecraftservices/minecraft/profile/lookup/name/remoteplayer", nil)
	req.SetPathValue("playerName", "remoteplayer")
	rec := httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != 200 || rec.Body.String() != `{"id":"remote-uuid","name":"RemotePlayer"}` {
		t.Fatalf("single status=%d body=%q", rec.Code, rec.Body.String())
	}
	wantCalls := []string{
		"POST /services/minecraft/profile/lookup/bulk/byname",
		"POST /services/minecraft/profile/lookup/bulk/byname",
		"POST /services/minecraft/profile/lookup/bulk/byname",
		"POST /account/profiles/minecraft",
		"GET /services/minecraft/profile/lookup/name/remoteplayer",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls=%v want=%v", calls, wantCalls)
	}
}
