package microsoft

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	importsvc "element-skin/backend/internal/service/imports"
	"element-skin/backend/internal/util"
)

func (h Handler) ImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	session, err := h.popState(req.Context(), body["ms_token"], stateKindImport, "invalid import token")
	if err != nil {
		util.Error(w, err)
		return
	}
	userID := shared.CurrentUserID(req)
	if err := requireStateOwner(session, userID, "not allowed"); err != nil {
		util.Error(w, err)
		return
	}
	profile, ok := session["profile"].(map[string]any)
	if !ok {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	profileID, _ := profile["id"].(string)
	profileName, _ := profile["name"].(string)
	res, err := (importsvc.ImportService{DB: h.db}).ImportProfile(req.Context(), userID, profileID, profileName, microsoftProfileAssets(profile))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func microsoftProfileAssets(profile map[string]any) []importsvc.TextureAsset {
	var assets []importsvc.TextureAsset
	assets = appendTextureAssets(assets, profile["skins"], "skin")
	assets = appendTextureAssets(assets, profile["capes"], "cape")
	return assets
}

func appendTextureAssets(assets []importsvc.TextureAsset, raw any, kind string) []importsvc.TextureAsset {
	if typed, ok := raw.([]map[string]string); ok {
		for _, item := range typed {
			assets = append(assets, importsvc.TextureAsset{URL: item["url"], Kind: kind, Variant: item["variant"]})
		}
		return assets
	}
	items, ok := raw.([]any)
	if !ok {
		return assets
	}
	for _, item := range items {
		asset, ok := textureAssetFromMap(item, kind)
		if ok {
			assets = append(assets, asset)
		}
	}
	return assets
}

func textureAssetFromMap(raw any, kind string) (importsvc.TextureAsset, bool) {
	item, ok := raw.(map[string]any)
	if !ok {
		return importsvc.TextureAsset{}, false
	}
	u, _ := item["url"].(string)
	variant, _ := item["variant"].(string)
	return importsvc.TextureAsset{URL: u, Kind: kind, Variant: variant}, true
}
