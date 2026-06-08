package microsoft

import (
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/httpapi/shared"
	mssvc "element-skin/backend/internal/service/microsoft"
	"element-skin/backend/internal/util"
)

func (h Handler) AuthURL(w http.ResponseWriter, req *http.Request) {
	state, err := randomToken(64)
	if err != nil {
		util.Error(w, err)
		return
	}
	clientID, err := h.settings.Get(req.Context(), "microsoft_client_id", "")
	if err != nil {
		util.Error(w, err)
		return
	}
	redirectURI, err := h.settings.Get(req.Context(), "microsoft_redirect_uri", strings.TrimRight(h.cfg.APIURL, "/")+"/microsoft/callback")
	if err != nil {
		util.Error(w, err)
		return
	}
	h.states.Put(state, map[string]any{"user_id": shared.CurrentUserID(req), "kind": stateKindOAuth}, 10*time.Minute)
	util.JSON(w, 200, map[string]any{
		"auth_url": mssvc.MicrosoftAuthorizationURL(clientID, redirectURI, state),
		"state":    state,
	})
}

func (h Handler) Callback(w http.ResponseWriter, req *http.Request) {
	siteURL := strings.TrimRight(h.cfg.SiteURL, "/")
	if siteURL == "" {
		siteURL = "http://localhost:5173"
	}
	if errText := req.URL.Query().Get("error"); errText != "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Authorization failed: " + errText})
		return
	}
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	if code == "" || state == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Missing code or state parameter"})
		return
	}
	session, err := h.popState(state, stateKindOAuth, "Invalid or expired state parameter")
	if err != nil {
		util.Error(w, err)
		return
	}
	clientID, err := h.settings.Get(req.Context(), "microsoft_client_id", "")
	if err != nil {
		util.Error(w, err)
		return
	}
	clientSecret, err := h.settings.Get(req.Context(), "microsoft_client_secret", "")
	if err != nil {
		util.Error(w, err)
		return
	}
	redirectURI, err := h.settings.Get(req.Context(), "microsoft_redirect_uri", strings.TrimRight(h.cfg.APIURL, "/")+"/microsoft/callback")
	if err != nil {
		util.Error(w, err)
		return
	}
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		http.Redirect(w, req, siteURL+"/dashboard/roles?error=auth_failed", http.StatusFound)
		return
	}
	result, err := (mssvc.MicrosoftAuthFlow{Client: mssvc.MicrosoftHTTPClient{
		ClientID: clientID, ClientSecret: clientSecret, RedirectURI: redirectURI,
	}}).Complete(req.Context(), code)
	if err != nil || result["profile"] == nil {
		http.Redirect(w, req, siteURL+"/dashboard/roles?error=auth_failed", http.StatusFound)
		return
	}
	token, err := randomToken(64)
	if err != nil {
		util.Error(w, err)
		return
	}
	h.states.Put(token, map[string]any{"user_id": session["user_id"], "kind": stateKindProfile, "profile": result}, 5*time.Minute)
	http.Redirect(w, req, siteURL+"/dashboard/roles?ms_token="+token, http.StatusFound)
}
