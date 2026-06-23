package microsoft

import (
	"net/http"
	"time"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) GetProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	session, err := h.popState(req.Context(), body["ms_token"], stateKindProfile, "Invalid or expired token")
	if err != nil {
		util.Error(w, err)
		return
	}
	userID := shared.CurrentUserID(req)
	if err := requireStateOwner(session, userID, "Unauthorized"); err != nil {
		util.Error(w, err)
		return
	}
	flowProfile, _ := session["profile"].(map[string]any)
	verified := verifiedMicrosoftProfile(flowProfile)
	importToken, err := randomToken(64)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.states.SetState(req.Context(), importToken, map[string]any{
		"user_id": userID,
		"kind":    stateKindImport,
		"profile": verified,
	}, 5*time.Minute); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{
		"profile":      verified,
		"has_game":     shared.ValueOrAny(flowProfile["has_game"], false),
		"import_token": importToken,
	})
}

func verifiedMicrosoftProfile(flowProfile map[string]any) map[string]any {
	mcProfile, _ := flowProfile["profile"].(map[string]any)
	return map[string]any{
		"id":    mcProfile["id"],
		"name":  mcProfile["name"],
		"skins": shared.ValueOrAny(mcProfile["skins"], []any{}),
		"capes": shared.ValueOrAny(mcProfile["capes"], []any{}),
	}
}
