package identity

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	authsvc "element-skin/backend/internal/service/auth"
	identitysvc "element-skin/backend/internal/service/identity"
	settingssvc "element-skin/backend/internal/service/settings"
)

type Handler struct {
	cfg     config.Config
	service identitysvc.Service
	authSvc authsvc.Service
	auth    shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc) Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	return Handler{
		cfg:     cfg,
		service: identitysvc.Service{DB: db, Config: cfg, Redis: redis},
		authSvc: authsvc.Service{DB: db, Cfg: cfg, Redis: redis, Settings: settings},
		auth:    auth,
	}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
