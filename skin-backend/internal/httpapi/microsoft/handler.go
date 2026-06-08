package microsoft

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

type Handler struct {
	cfg    config.Config
	db     *database.DB
	auth   shared.AuthFunc
	states *util.InMemoryStateStore
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc, states *util.InMemoryStateStore) Handler {
	return Handler{cfg: cfg, db: db, auth: auth, states: states}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next, false)
}
