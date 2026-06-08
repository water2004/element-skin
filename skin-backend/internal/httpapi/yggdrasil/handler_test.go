package yggdrasil_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ygghttp "element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestNewStoresConfigAndServicesByValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := ygghttp.New(cfg, db, settings.Settings{DB: db}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	rec := httptest.NewRecorder()
	h.Metadata(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "element-skin") {
		t.Fatalf("metadata via constructed handler mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
