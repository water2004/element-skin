package admin_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestEmailSuffixPolicyRoutesReplaceAndReturnNormalizedWholeLists(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(testutil.TestConfig(), db, cache, nil)
	if err := cache.SetPublicSettings(t.Context(), map[string]any{"site_name": "stale"}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/v2/admin/settings/email-suffix-policy", strings.NewReader(`{"mode":"allowlist","allowlist":["QQ.COM","@Example.com"],"denylist":["Blocked.Test"]}`))
	req = withAdminActor(req, "admin-test-user")
	rec := httptest.NewRecorder()
	h.PutEmailSuffixPolicy(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("put policy response: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := cache.GetPublicSettings(t.Context()); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("put policy should invalidate public settings cache, got %v", err)
	}

	req = withAdminActor(httptest.NewRequest(http.MethodGet, "/v2/admin/settings/email-suffix-policy", nil), "admin-test-user")
	rec = httptest.NewRecorder()
	h.GetEmailSuffixPolicy(rec, req)
	want := "{\"mode\":\"allowlist\",\"allowlist\":[\"@example.com\",\"@qq.com\"],\"denylist\":[\"@blocked.test\"]}\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("get policy response: status=%d body=%q want=%q", rec.Code, rec.Body.String(), want)
	}
}

func TestEmailSuffixPolicyRoutesRejectBadInputAndMissingPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	for _, tc := range []struct {
		name   string
		body   string
		actor  bool
		status int
		want   string
	}{
		{"malformed", `{`, true, http.StatusBadRequest, "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n"},
		{"empty allowlist", `{"mode":"allowlist","allowlist":[],"denylist":[]}`, true, http.StatusBadRequest, "{\"error\":{\"object\":\"email_allowlist\",\"operation\":\"configure\",\"reason\":\"required\"}}\n"},
		{"missing update permission", `{"mode":"disabled","allowlist":[],"denylist":[]}`, false, http.StatusForbidden, "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/v2/admin/settings/email-suffix-policy", strings.NewReader(tc.body))
			if tc.actor {
				req = withAdminActor(req, "admin-test-user")
			}
			rec := httptest.NewRecorder()
			h.PutEmailSuffixPolicy(rec, req)
			if rec.Code != tc.status || rec.Body.String() != tc.want {
				t.Fatalf("response status=%d body=%q want status=%d body=%q", rec.Code, rec.Body.String(), tc.status, tc.want)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/v2/admin/settings/email-suffix-policy", nil)
	rec := httptest.NewRecorder()
	h.GetEmailSuffixPolicy(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n" {
		t.Fatalf("get without permission: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
