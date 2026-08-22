package admin

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"unicode/utf8"

	"element-skin/backend/internal/httpapi/shared"
	invitesvc "element-skin/backend/internal/service/invite"
	"element-skin/backend/internal/util"
)

func (h Handler) Invites(w http.ResponseWriter, req *http.Request) {
	res, err := h.invites.List(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), util.ClampLimit(req.URL.Query().Get("limit"), 15))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) CreateInvite(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if _, exists := body["code"]; exists {
		util.Error(w, invalidInviteCodeError())
		return
	}
	code := ""
	codeBase64, codeSet := body["code_base64"]
	if codeSet {
		encoded, ok := codeBase64.(string)
		if !ok {
			util.Error(w, invalidInviteCodeError())
			return
		}
		var err error
		code, err = decodeInviteCode(encoded)
		if err != nil {
			util.Error(w, err)
			return
		}
	}
	note, _ := body["note"].(string)
	totalUses, totalUsesSet := body["total_uses"]
	res, err := h.invites.Create(req.Context(), shared.CurrentActor(req), invitesvc.CreateInput{
		Code:         code,
		CodeSet:      codeSet,
		TotalUses:    totalUses,
		TotalUsesSet: totalUsesSet,
		Note:         note,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusCreated, res)
}

func (h Handler) DeleteInvite(w http.ResponseWriter, req *http.Request) {
	code, err := decodeInviteCode(req.PathValue("code_base64"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.invites.Delete(req.Context(), shared.CurrentActor(req), code); err != nil {
		util.Error(w, err)
		return
	}
	util.NoContent(w)
}

func decodeInviteCode(encoded string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil ||
		len(decoded) == 0 ||
		base64.RawURLEncoding.EncodeToString(decoded) != encoded ||
		!utf8.Valid(decoded) ||
		bytes.IndexByte(decoded, 0) >= 0 {
		return "", invalidInviteCodeError()
	}
	return string(decoded), nil
}

func invalidInviteCodeError() util.HTTPError {
	return util.HTTPError{Status: http.StatusBadRequest, Object: "invite_code", Operation: "decode", Reason: "invalid"}
}
