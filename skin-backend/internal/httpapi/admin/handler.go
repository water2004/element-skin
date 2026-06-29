package admin

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	adminsvc "element-skin/backend/internal/service/admin"
	noticesvc "element-skin/backend/internal/service/notice"
	settingssvc "element-skin/backend/internal/service/settings"
	sitepkg "element-skin/backend/internal/service/site"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	settings settingssvc.Settings
	site     sitepkg.Site
	notices  noticesvc.Service
	perms    adminsvc.PermissionService
	auth     shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewWithRedis(cfg, db, redis, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc) Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	return Handler{cfg: cfg, db: db, redis: redis, settings: settings, site: sitepkg.Site{DB: db, Cfg: cfg, Redis: redis, Settings: settings}, notices: noticesvc.Service{DB: db}, perms: adminsvc.PermissionService{DB: db, Redis: redis}, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
