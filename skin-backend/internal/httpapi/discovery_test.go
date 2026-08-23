package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/permission"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestDiscoveryCapabilitiesAndPermissionCatalogExactPayloads(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root/"
	cfg.APIURL = ""
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v2/capabilities", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities status mismatch: got=%d body=%q", rec.Code, rec.Body.String())
	}
	var capabilities struct {
		APIVersion   string          `json:"api_version"`
		SiteName     string          `json:"site_name"`
		SiteURL      string          `json:"site_url"`
		APIURL       string          `json:"api_url"`
		Features     map[string]bool `json:"features"`
		TextureTypes []string        `json:"texture_types"`
		SkinModels   []string        `json:"skin_models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &capabilities); err != nil {
		t.Fatalf("decode capabilities %q: %v", rec.Body.String(), err)
	}
	if capabilities.APIVersion != "v2" ||
		capabilities.SiteName != "Element Skin" ||
		capabilities.SiteURL != "https://skin.example/root" ||
		capabilities.APIURL != "https://skin.example/root" ||
		len(capabilities.Features) != 9 ||
		!capabilities.Features["skin_library"] ||
		!capabilities.Features["oauth"] ||
		!capabilities.Features["device_code"] ||
		!capabilities.Features["minecraft_api"] ||
		!capabilities.Features["external_identities"] ||
		!capabilities.Features["oidc_client"] ||
		!capabilities.Features["oidc_server"] ||
		!capabilities.Features["official_profiles"] ||
		!capabilities.Features["remote_ygg_import"] ||
		len(capabilities.TextureTypes) != 2 ||
		capabilities.TextureTypes[0] != "skin" ||
		capabilities.TextureTypes[1] != "cape" ||
		len(capabilities.SkinModels) != 2 ||
		capabilities.SkinModels[0] != "default" ||
		capabilities.SkinModels[1] != "slim" {
		t.Fatalf("capabilities payload mismatch: %#v", capabilities)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v2/permissions/catalog", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("permission catalog status mismatch: got=%d body=%q", rec.Code, rec.Body.String())
	}
	var catalog struct {
		Permissions []struct {
			ID                  uint64 `json:"id"`
			Code                string `json:"code"`
			Description         string `json:"description"`
			Resource            string `json:"resource"`
			ResourceDescription string `json:"resource_description"`
			Action              string `json:"action"`
			ActionDescription   string `json:"action_description"`
			Scope               string `json:"scope"`
			ScopeDescription    string `json:"scope_description"`
			ResolverKey         string `json:"resolver_key"`
			Delegable           bool   `json:"delegable"`
			AdminDelegable      bool   `json:"admin_delegable"`
			Protected           bool   `json:"protected"`
		} `json:"permissions"`
		Roles []struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			SystemRole  bool     `json:"system_role"`
			Protected   bool     `json:"protected"`
			Permissions []string `json:"permissions"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode catalog %q: %v", rec.Body.String(), err)
	}
	if len(catalog.Permissions) != len(permission.Definitions) ||
		catalog.Permissions[0].ID != uint64(permission.Definitions[0].ID) ||
		catalog.Permissions[0].Code != "account.read.self" ||
		catalog.Permissions[0].Resource != "account" ||
		catalog.Permissions[0].Action != "read" ||
		catalog.Permissions[0].Scope != "self" ||
		!catalog.Permissions[0].Delegable ||
		catalog.Permissions[0].AdminDelegable ||
		catalog.Permissions[0].Protected {
		t.Fatalf("permission catalog first item mismatch: %#v", catalog.Permissions[0])
	}
	last := catalog.Permissions[len(catalog.Permissions)-1]
	if last.Code == "" || last.Description == "" || last.Resource == "" || last.Action == "" || last.Scope == "" {
		t.Fatalf("permission catalog last item should be fully described: %#v", last)
	}
	if len(catalog.Roles) != len(permission.Roles) ||
		catalog.Roles[0].ID != permission.RoleUser ||
		catalog.Roles[0].Name != "用户" ||
		catalog.Roles[0].SystemRole != true ||
		catalog.Roles[0].Protected != false ||
		len(catalog.Roles[0].Permissions) == 0 ||
		catalog.Roles[0].Permissions[0] != "account.read.self" {
		t.Fatalf("role catalog first item mismatch: %#v", catalog.Roles[0])
	}
}
