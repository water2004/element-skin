package identity

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	identitysvc "element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/util"
)

func (h Handler) StartAuthorization(w http.ResponseWriter, req *http.Request) {
	var body struct {
		ProviderID string `json:"provider_id"`
		Intent     string `json:"intent"`
		IdentityID string `json:"identity_id"`
		ReturnTo   string `json:"return_to"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	result, err := h.service.StartAuthorization(
		req.Context(),
		shared.CurrentActor(req),
		body.ProviderID,
		body.Intent,
		body.IdentityID,
		body.ReturnTo,
	)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, result)
}

func (h Handler) AuthorizationCallback(w http.ResponseWriter, req *http.Request) {
	result, err := h.service.CompleteAuthorization(
		req.Context(),
		req.URL.Query().Get("code"),
		req.URL.Query().Get("state"),
		req.URL.Query().Get("error"),
	)
	if err != nil {
		if query, ok := authorizationErrorQuery(err); ok {
			h.redirectToSite(w, req, "/dashboard/identities", query)
			return
		}
		util.Error(w, err)
		return
	}
	switch result.Intent {
	case identitysvc.AuthorizationIntentLink:
		h.redirectToSite(w, req, "/dashboard/identities", url.Values{
			"linked_identity_id": []string{result.IdentityID},
		})
	case identitysvc.AuthorizationIntentLogin:
		session, err := h.authSvc.IssueSessionForUser(req.Context(), result.UserID)
		if err != nil {
			util.Error(w, err)
			return
		}
		shared.SetWebSessionCookies(
			w,
			h.cfg,
			session["access_token"].(string),
			session["refresh_token"].(string),
			session["refresh_max_age_seconds"].(int),
		)
		path := result.ReturnTo
		if path == "" {
			path = "/dashboard"
		}
		h.redirectToSite(w, req, path, nil)
	case "registration":
		query := url.Values{
			"identity_ticket": []string{result.RegistrationTicket},
			"provider_id":     []string{result.ProviderID},
		}
		if result.ReturnTo != "" {
			query.Set("redirect", result.ReturnTo)
		}
		h.redirectToSite(w, req, "/register", query)
	default:
		util.Error(w, util.HTTPError{Status: http.StatusInternalServerError, Object: "identity", Operation: "authorize", Reason: "invalid"})
	}
}

func authorizationErrorQuery(err error) (url.Values, bool) {
	var apiErr util.HTTPError
	if !errors.As(err, &apiErr) || apiErr.Object != "identity" {
		return nil, false
	}
	redirectable := (apiErr.Operation == "authorize" &&
		(apiErr.Reason == "mismatch" || apiErr.Reason == "incomplete")) ||
		(apiErr.Operation == "link" &&
			(apiErr.Reason == "already_exists" || apiErr.Reason == "conflict"))
	if !redirectable {
		return nil, false
	}
	return url.Values{
		"error_object":    []string{apiErr.Object},
		"error_operation": []string{apiErr.Operation},
		"error_reason":    []string{apiErr.Reason},
	}, true
}

func (h Handler) redirectToSite(w http.ResponseWriter, req *http.Request, path string, query url.Values) {
	base := strings.TrimRight(strings.TrimSpace(h.cfg.SiteURL), "/")
	u, err := url.Parse(base + path)
	if err != nil || u.Scheme == "" || u.Host == "" {
		util.Error(w, util.HTTPError{Status: http.StatusInternalServerError, Object: "server", Operation: "handle", Reason: "failed"})
		return
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	http.Redirect(w, req, u.String(), http.StatusSeeOther)
}

type providerRequest struct {
	Name         string   `json:"name"`
	IssuerURL    string   `json:"issuer_url"`
	ClientID     string   `json:"client_id"`
	ClientSecret *string  `json:"client_secret"`
	Scopes       []string `json:"scopes"`
	Adapter      string   `json:"adapter"`
	IconURL      string   `json:"icon_url"`
	Enabled      bool     `json:"enabled"`
	LoginEnabled bool     `json:"login_enabled"`
	LinkEnabled  bool     `json:"link_enabled"`
	DisplayOrder int      `json:"display_order"`
}

func (r providerRequest) input() identitysvc.ProviderInput {
	return identitysvc.ProviderInput{
		Name:         r.Name,
		IssuerURL:    r.IssuerURL,
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		Scopes:       r.Scopes,
		Adapter:      r.Adapter,
		IconURL:      r.IconURL,
		Enabled:      r.Enabled,
		LoginEnabled: r.LoginEnabled,
		LinkEnabled:  r.LinkEnabled,
		DisplayOrder: r.DisplayOrder,
	}
}

func (h Handler) PublicProviders(w http.ResponseWriter, req *http.Request) {
	items, err := h.service.ListPublicProviders(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{
		"items":        items,
		"redirect_uri": h.service.RedirectURI(),
	})
}

func (h Handler) ListProviders(w http.ResponseWriter, req *http.Request) {
	items, err := h.service.ListProviders(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{
		"items":        items,
		"redirect_uri": h.service.RedirectURI(),
	})
}

func (h Handler) GetProvider(w http.ResponseWriter, req *http.Request) {
	item, err := h.service.GetProvider(req.Context(), shared.CurrentActor(req), req.PathValue("provider_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, item)
}

func (h Handler) CreateProvider(w http.ResponseWriter, req *http.Request) {
	var body providerRequest
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	item, err := h.service.CreateProvider(req.Context(), shared.CurrentActor(req), body.input())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, item)
}

func (h Handler) UpdateProvider(w http.ResponseWriter, req *http.Request) {
	var body providerRequest
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	item, err := h.service.UpdateProvider(req.Context(), shared.CurrentActor(req), req.PathValue("provider_id"), body.input())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, item)
}

func (h Handler) DeleteProvider(w http.ResponseWriter, req *http.Request) {
	if err := h.service.DeleteProvider(req.Context(), shared.CurrentActor(req), req.PathValue("provider_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) ListIdentities(w http.ResponseWriter, req *http.Request) {
	items, err := h.service.ListIdentities(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) UpdateIdentity(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if err := h.service.UpdateIdentityLabel(req.Context(), shared.CurrentActor(req), req.PathValue("identity_id"), body.Label); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func (h Handler) DeleteIdentity(w http.ResponseWriter, req *http.Request) {
	if err := h.service.DeleteIdentity(req.Context(), shared.CurrentActor(req), req.PathValue("identity_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}
