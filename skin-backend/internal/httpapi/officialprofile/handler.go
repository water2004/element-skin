package officialprofile

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	identitysvc "element-skin/backend/internal/service/identity"
	officialsvc "element-skin/backend/internal/service/officialprofile"
)

type Handler struct {
	service officialsvc.Service
	auth    shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc) Handler {
	identities := identitysvc.Service{DB: db, Config: cfg, Redis: redis}
	return NewWithService(
		officialsvc.Service{
			DB: db, Identities: identities, TexturesDir: cfg.TexturesDir,
		},
		auth,
	)
}

func NewWithService(service officialsvc.Service, auth shared.AuthFunc) Handler {
	return Handler{service: service, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
