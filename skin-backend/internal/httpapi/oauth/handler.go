package oauth

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

type Handler struct {
	cfg   config.Config
	auth  shared.AuthFunc
	oauth oauthsvc.Service
}

func New(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc, signers ...*oauthsvc.OIDCSigner) Handler {
	var signer *oauthsvc.OIDCSigner
	if len(signers) > 0 {
		signer = signers[0]
	}
	return Handler{cfg: cfg, auth: auth, oauth: oauthsvc.Service{DB: db, Redis: redis, Config: cfg, OIDCSigner: signer}}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}

func (h Handler) AuthorizationServerMetadata(w http.ResponseWriter, req *http.Request) {
	base := h.baseURL()
	metadata := map[string]any{
		"issuer":                                     base,
		"authorization_endpoint":                     base + "/oauth/authorize",
		"device_authorization_endpoint":              base + "/oauth/device/code",
		"token_endpoint":                             base + "/oauth/token",
		"revocation_endpoint":                        base + "/oauth/revoke",
		"introspection_endpoint":                     base + "/oauth/introspect",
		"response_types_supported":                   []string{"code"},
		"grant_types_supported":                      []string{"authorization_code", "refresh_token", "client_credentials", "urn:ietf:params:oauth:grant-type:device_code"},
		"code_challenge_methods_supported":           []string{"S256"},
		"token_endpoint_auth_methods_supported":      []string{"client_secret_basic", "client_secret_post", "none"},
		"revocation_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
		"scopes_supported":                           h.authorizationScopeCodes(),
		"protected_resources":                        []string{base + "/v2"},
	}
	if docs := strings.TrimRight(h.cfg.SiteURL, "/"); docs != "" {
		metadata["service_documentation"] = docs
	}
	util.JSON(w, http.StatusOK, metadata)
}

func (h Handler) ProtectedResourceMetadata(w http.ResponseWriter, req *http.Request) {
	base := h.baseURL()
	util.JSON(w, http.StatusOK, map[string]any{
		"resource":                 base + "/v2",
		"authorization_servers":    []string{base},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         h.scopeCodes(),
	})
}

func (h Handler) baseURL() string {
	if strings.TrimSpace(h.cfg.APIURL) != "" {
		return strings.TrimRight(h.cfg.APIURL, "/")
	}
	return strings.TrimRight(h.cfg.SiteURL, "/")
}

func (h Handler) scopeCodes() []string {
	codes := make([]string, 0)
	for _, def := range permission.Definitions {
		if def.Scope.ID != permission.ScopeSystem {
			codes = append(codes, def.Code)
		}
	}
	return codes
}

func (h Handler) authorizationScopeCodes() []string {
	return append([]string{"openid", "profile", "email", "offline_access"}, h.scopeCodes()...)
}
