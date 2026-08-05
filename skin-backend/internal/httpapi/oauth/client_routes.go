package oauth

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (h Handler) ListApps(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.ListClients(req.Context(), shared.CurrentActor(req), util.ClampLimit(req.URL.Query().Get("limit")))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h Handler) ListAdminApps(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.ListClientsForAdmin(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("status"), util.ClampLimit(req.URL.Query().Get("limit")))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h Handler) CreateApp(w http.ResponseWriter, req *http.Request) {
	var body appBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.oauth.CreateClient(req.Context(), shared.CurrentActor(req), body.input())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, res)
}

func (h Handler) GetApp(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.GetClient(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) UpdateApp(w http.ResponseWriter, req *http.Request) {
	var body appBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.oauth.UpdateClient(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"), body.input(), body.Status)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) SubmitAppReview(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.SubmitClientForReview(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) ReviewApp(w http.ResponseWriter, req *http.Request) {
	var body appReviewBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.oauth.ReviewClient(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"), body.Status, body.Reason)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) RotateSecret(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.RotateClientSecret(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) DeleteApp(w http.ResponseWriter, req *http.Request) {
	if err := h.oauth.DeleteClient(req.Context(), shared.CurrentActor(req), req.PathValue("client_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

type appBody struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	RedirectURI     string   `json:"redirect_uri"`
	WebsiteURL      string   `json:"website_url"`
	ClientType      string   `json:"client_type"`
	Status          string   `json:"status"`
	PermissionCodes []string `json:"permissions"`
}

type appReviewBody struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (b appBody) input() oauthsvc.ClientInput {
	return oauthsvc.ClientInput{
		Name:            b.Name,
		Description:     b.Description,
		RedirectURI:     b.RedirectURI,
		WebsiteURL:      b.WebsiteURL,
		ClientType:      b.ClientType,
		PermissionCodes: b.PermissionCodes,
	}
}
