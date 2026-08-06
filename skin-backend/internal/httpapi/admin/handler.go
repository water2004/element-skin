package admin

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	homepagesvc "element-skin/backend/internal/service/homepage"
	invitesvc "element-skin/backend/internal/service/invite"
	noticesvc "element-skin/backend/internal/service/notice"
	permissionssvc "element-skin/backend/internal/service/permissions"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
)

type Handler struct {
	cfg         config.Config
	settings    settingssvc.Settings
	emailPolicy emailpolicysvc.Service
	profiles    profilesvc.Service
	notices     noticesvc.Service
	perms       permissionssvc.PermissionService
	accounts    accountsvc.AccountService
	invites     invitesvc.Service
	textures    texturesvc.LibraryService
	fallback    fallbacksvc.Fallback
	homepage    homepagesvc.Service
	auth        shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewWithRedis(cfg, db, redis, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc) Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	profiles := profilesvc.Service{DB: db, Settings: settings}
	return Handler{
		cfg:         cfg,
		settings:    settings,
		emailPolicy: emailpolicysvc.Service{DB: db, Redis: redis},
		profiles:    profiles,
		notices:     noticesvc.Service{DB: db},
		perms:       permissionssvc.PermissionService{DB: db, Redis: redis},
		accounts:    accountsvc.AccountService{DB: db, Redis: redis},
		invites:     invitesvc.Service{DB: db},
		textures:    texturesvc.LibraryService{DB: db, Settings: settings},
		fallback:    fallbacksvc.Fallback{DB: db, Redis: redis, Settings: settings},
		homepage:    homepagesvc.Service{DB: db, Redis: redis, CarouselDir: cfg.CarouselDir},
		auth:        auth,
	}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
