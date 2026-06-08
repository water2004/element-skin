package site

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	sitepkg "element-skin/backend/internal/service/site"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	site     sitepkg.Site
	settings settingssvc.Settings
	auth     shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, svc sitepkg.Site, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	if svc.Redis == nil {
		svc.Redis = redis
	}
	return NewWithRedis(cfg, db, redis, svc, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, svc sitepkg.Site, auth shared.AuthFunc) Handler {
	if svc.Redis == nil {
		svc.Redis = redis
	}
	settings := settingssvc.Settings{DB: db, Redis: redis}
	if svc.Settings.DB == nil {
		svc.Settings = settings
	}
	return Handler{cfg: cfg, db: db, redis: redis, site: svc, settings: settings, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next, false)
}
