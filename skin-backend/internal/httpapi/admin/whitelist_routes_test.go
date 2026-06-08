package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestWhitelistRoutesAddOfficialWhitelistPersistsUser(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Fallbacks.SaveEndpoints(context.Background(), []fallback.Endpoint{{
		Priority:        1,
		SessionURL:      "https://session.example",
		AccountURL:      "https://account.example",
		ServicesURL:     "https://services.example",
		CacheTTL:        60,
		EnableProfile:   true,
		EnableHasJoined: true,
		EnableWhitelist: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/official-whitelist", strings.NewReader(`{"username":"Steve","endpoint_id":1}`))
	rec := httptest.NewRecorder()
	h.AddOfficialWhitelist(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("add whitelist response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	ok, err := db.Fallbacks.IsUserInWhitelist(req.Context(), "Steve", 1)
	if err != nil || !ok {
		t.Fatalf("whitelist row should exist exactly: ok=%v err=%v", ok, err)
	}
}
