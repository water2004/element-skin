package remote

import (
	"net/http"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
)

type Handler struct {
	db   *database.DB
	auth shared.AuthFunc
}

func New(db *database.DB, auth shared.AuthFunc) Handler {
	return Handler{db: db, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next, false)
}
