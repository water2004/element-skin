package site

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	sitepkg "element-skin/backend/internal/service/site"
)

type Handler struct {
	cfg  config.Config
	db   *database.DB
	site sitepkg.Site
	auth shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, site sitepkg.Site, auth shared.AuthFunc) Handler {
	return Handler{cfg: cfg, db: db, site: site, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next, false)
}
