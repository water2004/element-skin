package httpapi

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	authsvc "element-skin/backend/internal/service/auth"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (r *Router) auth(next http.HandlerFunc, required ...permission.Definition) http.HandlerFunc {
	return r.withActor(next, false, required...)
}

func (r *Router) publicAuth(next http.HandlerFunc, required ...permission.Definition) http.HandlerFunc {
	return r.withActor(next, true, required...)
}

func (r *Router) withActor(next http.HandlerFunc, allowGuest bool, required ...permission.Definition) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		actor, authenticated, err := r.requestActor(req)
		if err != nil {
			util.Error(w, err)
			return
		}
		if !authenticated {
			if !allowGuest {
				util.Error(w, util.HTTPError{Status: 401, Object: "authentication", Operation: "verify", Reason: "required"})
				return
			}
			actor = permission.GuestActor()
		}
		for _, def := range required {
			if err := actor.Require(def); err != nil {
				util.Error(w, util.HTTPError{Status: 403, Object: "permission", Operation: "check", Reason: "denied"})
				return
			}
		}
		next(w, req.WithContext(shared.WithActor(req.Context(), actor)))
	}
}

func (r *Router) requestActor(req *http.Request) (permission.Actor, bool, error) {
	if bearer, ok := shared.BearerToken(req); ok {
		actor, authenticated, err := (oauthsvc.Service{DB: r.db, Redis: r.redis}).ActorForBearer(req.Context(), bearer)
		if err != nil {
			return permission.Actor{}, false, err
		}
		if !authenticated {
			return permission.Actor{}, false, util.HTTPError{Status: 401, Object: "authentication", Operation: "verify", Reason: "required"}
		}
		return actor, true, nil
	}
	cookie, err := req.Cookie("access_token")
	if err != nil || cookie.Value == "" {
		return permission.Actor{}, false, nil
	}
	actor, authenticated, err := (authsvc.Service{DB: r.db, Cfg: r.cfg, Redis: r.redis}).ActorForWebAccessToken(req.Context(), cookie.Value)
	if err != nil {
		return permission.Actor{}, false, err
	}
	if !authenticated {
		return permission.Actor{}, false, util.HTTPError{Status: 401, Object: "authentication", Operation: "verify", Reason: "required"}
	}
	return actor, true, nil
}
