package yggdrasil_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/testutil"
)

func TestAuthRoutesValidateMissingTokenAndMetadataExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := yggdrasil.New(cfg, db, service.Yggdrasil{DB: db, Cfg: cfg})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Metadata(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"implementationName"`) || !strings.Contains(rec.Body.String(), `"skinDomains"`) {
		t.Fatalf("metadata response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/authserver/validate", strings.NewReader(`{"accessToken":"missing"}`))
	rec = httptest.NewRecorder()
	h.Validate(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "Invalid token") {
		t.Fatalf("validate missing token mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
