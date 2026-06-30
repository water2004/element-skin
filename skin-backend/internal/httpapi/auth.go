package httpapi

import (
	"errors"
	"net/http"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (r *Router) auth(next http.HandlerFunc, required ...permission.Definition) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if bearer, ok := shared.BearerToken(req); ok {
			actor, authenticated, err := (oauthsvc.Service{DB: r.db, Redis: r.redis}).ActorForBearer(req.Context(), bearer)
			if err != nil {
				util.Error(w, err)
				return
			}
			if !authenticated {
				util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
				return
			}
			for _, def := range required {
				if err := actor.Require(def); err != nil {
					util.Error(w, util.HTTPError{Status: 403, Detail: "permission denied"})
					return
				}
			}
			next(w, req.WithContext(shared.WithActor(req.Context(), actor)))
			return
		}
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
		authUser, err := r.redis.GetAuthUser(req.Context(), userID)
		if errors.Is(err, redisstore.ErrCacheMiss) {
			user, dbErr := r.db.Users.GetByID(req.Context(), userID)
			if dbErr != nil {
				util.Error(w, dbErr)
				return
			}
			if user == nil {
				util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
				return
			}
			authUser = redisstore.AuthUserFromModel(*user)
			if setErr := r.redis.SetAuthUser(req.Context(), authUser, time.Duration(r.cfg.AuthCacheTTL)*time.Second); setErr != nil {
				util.Error(w, setErr)
				return
			}
		} else if err != nil {
			util.Error(w, err)
			return
		}
		actor, err := r.db.Permissions.ActorForUser(req.Context(), authUser.ID, permissiondb.EffectiveOptions{
			SessionKind: permission.SessionKindWeb,
			Entrypoint:  permission.EntrypointDashboard,
		})
		if err != nil {
			util.Error(w, err)
			return
		}
		for _, def := range required {
			if err := actor.Require(def); err != nil {
				util.Error(w, util.HTTPError{Status: 403, Detail: "permission denied"})
				return
			}
		}
		next(w, req.WithContext(shared.WithActor(req.Context(), actor)))
	}
}
