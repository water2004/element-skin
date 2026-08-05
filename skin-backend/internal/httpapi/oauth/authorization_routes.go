package oauth

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (h Handler) AuthorizeInfo(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.AuthorizationDetails(req.Context(), shared.CurrentActor(req), authorizationRequest(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) ApproveAuthorization(w http.ResponseWriter, req *http.Request) {
	var body authorizeBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.oauth.ApproveAuthorization(req.Context(), shared.CurrentActor(req), body.request())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

type authorizeBody struct {
	ResponseType        string `json:"response_type"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Nonce               string `json:"nonce"`
}

func (b authorizeBody) request() oauthsvc.AuthorizationRequest {
	return oauthsvc.AuthorizationRequest{
		ResponseType:        b.ResponseType,
		ClientID:            b.ClientID,
		RedirectURI:         b.RedirectURI,
		Scope:               b.Scope,
		State:               b.State,
		CodeChallenge:       b.CodeChallenge,
		CodeChallengeMethod: b.CodeChallengeMethod,
		Nonce:               b.Nonce,
	}
}

func authorizationRequest(req *http.Request) oauthsvc.AuthorizationRequest {
	q := req.URL.Query()
	return oauthsvc.AuthorizationRequest{
		ResponseType:        q.Get("response_type"),
		ClientID:            q.Get("client_id"),
		RedirectURI:         q.Get("redirect_uri"),
		Scope:               q.Get("scope"),
		State:               q.Get("state"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
		Nonce:               q.Get("nonce"),
	}
}
