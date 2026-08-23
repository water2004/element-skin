package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	mailsvc "element-skin/backend/internal/service/mail"
	oauthsvc "element-skin/backend/internal/service/oauth"
	settingssvc "element-skin/backend/internal/service/settings"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Router struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	settings settingssvc.Settings
	ygg      yggpkg.Yggdrasil
	mux      *http.ServeMux
	mail     mailsvc.Sender
	oidc     *oauthsvc.OIDCSigner
}

func NewRouter(cfg config.Config, db *database.DB, ygg yggpkg.Yggdrasil) http.Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewRouterWithRedis(cfg, db, redis, ygg)
}

func NewRouterWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, ygg yggpkg.Yggdrasil, senders ...mailsvc.Sender) http.Handler {
	return newRouter(cfg, db, redis, ygg, nil, senders...)
}

func NewRouterWithOIDCSigner(cfg config.Config, db *database.DB, redis redisstore.Store, ygg yggpkg.Yggdrasil, signer *oauthsvc.OIDCSigner, senders ...mailsvc.Sender) http.Handler {
	return newRouter(cfg, db, redis, ygg, signer, senders...)
}

func newRouter(cfg config.Config, db *database.DB, redis redisstore.Store, ygg yggpkg.Yggdrasil, signer *oauthsvc.OIDCSigner, senders ...mailsvc.Sender) http.Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	var sender mailsvc.Sender = mailsvc.SMTP{Settings: settings}
	if len(senders) > 0 && senders[0] != nil {
		sender = senders[0]
	}
	ygg.Redis = redis
	ygg.Settings = settings
	r := &Router{cfg: cfg, db: db, redis: redis, settings: settings, ygg: ygg, mux: http.NewServeMux(), mail: sender, oidc: signer}
	r.routes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.cfg.APIURL != "" {
		w.Header().Set("X-Authlib-Injector-API-Location", r.cfg.APIURL)
	}
	if r.applyCORS(w, req) {
		return
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) handle(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	})
}

func (r *Router) applyCORS(w http.ResponseWriter, req *http.Request) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return false
	}
	allowed, wildcard := corsOriginAllowed(origin, r.cfg.CORSOrigins)
	if wildcard && r.cfg.CORSCredentials {
		allowed = false
	}
	if !allowed {
		if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return true
		}
		return false
	}
	w.Header().Add("Vary", "Origin")
	if wildcard {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	if r.cfg.CORSCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if requested := req.Header.Get("Access-Control-Request-Headers"); requested != "" {
			w.Header().Set("Access-Control-Allow-Headers", requested)
			w.Header().Add("Vary", "Access-Control-Request-Headers")
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func corsOriginAllowed(origin string, allowedOrigins []string) (bool, bool) {
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			return true, true
		}
		if allowed == origin {
			return true, false
		}
	}
	return false, false
}
