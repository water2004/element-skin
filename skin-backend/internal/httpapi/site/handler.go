package site

import (
	"net/http"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	authsvc "element-skin/backend/internal/service/auth"
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	identitysvc "element-skin/backend/internal/service/identity"
	mailsvc "element-skin/backend/internal/service/mail"
	profilesvc "element-skin/backend/internal/service/profile"
	publicsitesvc "element-skin/backend/internal/service/publicsite"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	verificationsvc "element-skin/backend/internal/service/verification"
)

type Handler struct {
	cfg      config.Config
	redis    redisstore.Store
	authSvc  authsvc.Service
	accounts accountsvc.AccountService
	profiles profilesvc.Service
	textures texturesvc.LibraryService
	public   publicsitesvc.Service
	uploads  texturesvc.UploadService
	settings settingssvc.Settings
	auth     shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewWithRedis(cfg, db, redis, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc, senders ...mailsvc.Sender) Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	var sender mailsvc.Sender = mailsvc.SMTP{Settings: settings}
	if len(senders) > 0 && senders[0] != nil {
		sender = senders[0]
	}
	verification := verificationsvc.Service{DB: db, Redis: redis, Settings: settings, Sender: sender}
	emailPolicy := emailpolicysvc.Service{DB: db, Redis: redis}
	verification.EmailPolicy = emailPolicy
	identityService := identitysvc.Service{DB: db, Config: cfg, Redis: redis}
	authService := authsvc.Service{DB: db, Cfg: cfg, Redis: redis, Settings: settings, Verification: verification, Identity: identityService, EmailPolicy: emailPolicy}
	accounts := accountsvc.AccountService{DB: db, Redis: redis, Verification: verification, EmailPolicy: emailPolicy}
	profiles := profilesvc.Service{DB: db, Settings: settings}
	textures := texturesvc.LibraryService{DB: db, Settings: settings}
	public := publicsitesvc.Service{
		DB:          db,
		Redis:       redis,
		Settings:    settings,
		EmailPolicy: emailPolicy,
		SiteURL:     cfg.SiteURL,
		APIURL:      cfg.APIURL,
		CacheTTL:    time.Duration(cfg.PublicCacheTTL) * time.Second,
	}
	uploads := texturesvc.UploadService{DB: db, TexturesDir: cfg.TexturesDir}
	return Handler{cfg: cfg, redis: redis, authSvc: authService, accounts: accounts, profiles: profiles, textures: textures, public: public, uploads: uploads, settings: settings, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
