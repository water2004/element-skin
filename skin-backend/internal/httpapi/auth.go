package httpapi

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (r *Router) auth(next http.HandlerFunc, requireAdmin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("access_token")
		if err != nil || cookie.Value == "" {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		claims, ok := util.DecodeAccessToken(r.cfg.JWTSecret, cookie.Value)
		if !ok {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		userID, _ := claims["sub"].(string)
		user, err := r.db.GetUserByID(req.Context(), userID)
		if err != nil {
			util.Error(w, err)
			return
		}
		if user == nil {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		if requireAdmin && !user.IsAdmin {
			util.Error(w, util.HTTPError{Status: 403, Detail: "admin required"})
			return
		}
		next(w, req.WithContext(shared.WithUser(req.Context(), user.ID, user.IsAdmin)))
	}
}
