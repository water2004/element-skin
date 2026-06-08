package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestAdminSiteRoutesInviteWhitelistAndSettingsExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.SaveFallbackEndpoints(context.Background(), []database.FallbackEndpoint{{
		Priority: 1, SessionURL: "https://session.example", AccountURL: "https://account.example", ServicesURL: "https://services.example",
		CacheTTL: 60, EnableProfile: true, EnableHasJoined: true, EnableWhitelist: true,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"route-invite","total_uses":2,"note":"Route Invite"}`))
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"code\":\"route-invite\",\"note\":\"Route Invite\",\"total_uses\":2}\n" {
		t.Fatalf("create invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	invite, err := db.GetInvite(req.Context(), "route-invite")
	if err != nil || invite == nil || invite.Code != "route-invite" || invite.Note != "Route Invite" || invite.TotalUses == nil || *invite.TotalUses != 2 {
		t.Fatalf("created invite state mismatch: invite=%#v err=%v", invite, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/official-whitelist", strings.NewReader(`{"username":"Steve","endpoint_id":1}`))
	rec = httptest.NewRecorder()
	h.AddOfficialWhitelist(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("add whitelist response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	ok, err := db.IsUserInWhitelist(req.Context(), "Steve", 1)
	if err != nil || !ok {
		t.Fatalf("whitelist row should exist exactly: ok=%v err=%v", ok, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/settings/site", strings.NewReader(`{"site_name":"Route Site"}`))
	rec = httptest.NewRecorder()
	h.SaveSiteSettings(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("save settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	got, err := db.GetSetting(req.Context(), "site_name", "")
	if err != nil || got != "Route Site" {
		t.Fatalf("site setting should persist exactly: got=%q err=%v", got, err)
	}
}
