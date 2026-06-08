package yggdrasil_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestLookupRoutesNamesReturnExactLocalProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-lookup@test.com", "Password123", "YggLookup", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_lookup_profile", "YggLookupProfile")

	req := httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`["YggLookupProfile","MissingName"]`))
	rec := httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"YggLookupProfile"`) {
		t.Fatalf("lookup names response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
