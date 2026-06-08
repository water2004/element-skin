package httpapi

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	sitepkg "element-skin/backend/internal/service/site"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"
)

type Router struct {
	cfg  config.Config
	db   *database.DB
	site sitepkg.Site
	ygg  yggpkg.Yggdrasil
	mux  *http.ServeMux
}

var MicrosoftImportStates = util.NewInMemoryStateStore()

func NewRouter(cfg config.Config, db *database.DB, site sitepkg.Site, ygg yggpkg.Yggdrasil) http.Handler {
	r := &Router{cfg: cfg, db: db, site: site, ygg: ygg, mux: http.NewServeMux()}
	r.routes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.cfg.APIURL != "" {
		w.Header().Set("X-Authlib-Injector-API-Location", r.cfg.APIURL)
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) handle(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	})
}
