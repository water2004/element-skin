package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

type Router struct {
	cfg  config.Config
	db   *database.DB
	site service.Site
	ygg  service.Yggdrasil
	mux  *http.ServeMux
}

var MicrosoftImportStates = util.NewInMemoryStateStore()

type ctxKey string

const userIDKey ctxKey = "user_id"
const adminKey ctxKey = "admin"

func NewRouter(cfg config.Config, db *database.DB, site service.Site, ygg service.Yggdrasil) http.Handler {
	r := &Router{cfg: cfg, db: db, site: site, ygg: ygg, mux: http.NewServeMux()}
	r.routes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.cfg.APIURL != "" {
		w.Header().Set("X-Authlib-Injector-API-Location", r.cfg.APIURL)
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) routes() {
	r.handle("GET /", r.yggMetadata)
	r.handle("POST /site-login", r.siteLogin)
	r.handle("POST /site-logout", r.siteLogout)
	r.handle("POST /register", r.register)
	r.handle("POST /send-verification-code", r.sendVerificationCode)
	r.handle("POST /reset-password", r.resetPassword)
	r.handle("GET /me", r.auth(r.me, false))
	r.handle("PATCH /me", r.auth(r.updateMe, false))
	r.handle("DELETE /me", r.auth(r.deleteMe, false))
	r.handle("POST /me/password", r.auth(r.changePassword, false))
	r.handle("POST /me/refresh-token", r.refreshToken)
	r.handle("GET /me/profiles", r.auth(r.listMyProfiles, false))
	r.handle("POST /me/profiles", r.auth(r.createProfile, false))
	r.handle("PATCH /me/profiles/{pid}", r.auth(r.updateProfile, false))
	r.handle("DELETE /me/profiles/{pid}", r.auth(r.deleteProfile, false))
	r.handle("DELETE /me/profiles/{pid}/skin", r.auth(r.clearProfileSkin, false))
	r.handle("DELETE /me/profiles/{pid}/cape", r.auth(r.clearProfileCape, false))
	r.handle("GET /me/textures", r.auth(r.listMyTextures, false))
	r.handle("POST /me/textures", r.auth(r.uploadMyTexture, false))
	r.handle("GET /me/textures/{hash}/{texture_type}", r.auth(r.textureDetail, false))
	r.handle("PATCH /me/textures/{hash}/{texture_type}", r.auth(r.updateTexture, false))
	r.handle("DELETE /me/textures/{hash}/{texture_type}", r.auth(r.deleteTexture, false))
	r.handle("POST /me/textures/{hash}/add", r.auth(r.addTexture, false))
	r.handle("POST /me/textures/{hash}/apply", r.auth(r.applyTexture, false))
	r.handle("POST /textures/upload", r.auth(r.uploadAndApplyTexture, false))
	r.handle("GET /public/skin-library", r.publicLibrary)
	r.handle("GET /public/settings", r.publicSettings)
	r.handle("GET /public/carousel", r.publicCarousel)

	r.handle("POST /authserver/authenticate", r.yggAuthenticate)
	r.handle("POST /authserver/refresh", r.yggRefresh)
	r.handle("POST /authserver/validate", r.yggValidate)
	r.handle("POST /authserver/invalidate", r.yggInvalidate)
	r.handle("POST /authserver/signout", r.yggSignout)
	r.handle("POST /sessionserver/session/minecraft/join", r.yggJoin)
	r.handle("GET /sessionserver/session/minecraft/hasJoined", r.yggHasJoined)
	r.handle("GET /sessionserver/session/minecraft/profile/{uuid}", r.yggProfile)
	r.handle("GET /api/users/profiles/minecraft/{playerName}", r.lookupName)
	r.handle("GET /users/profiles/minecraft/{playerName}", r.lookupName)
	r.handle("GET /api/profiles/minecraft/{playerName}", r.lookupName)
	r.handle("POST /api/profiles/minecraft", r.lookupNames)
	r.handle("GET /api/minecraft/profile/lookup/name/{playerName}", r.lookupName)
	r.handle("GET /minecraft/profile/lookup/name/{playerName}", r.lookupName)
	r.handle("PUT /api/user/profile/{uuid}/{texture_type}", r.yggUploadTexture)
	r.handle("DELETE /api/user/profile/{uuid}/{texture_type}", r.yggDeleteTexture)
	r.handle("GET /microsoft/auth-url", r.auth(r.microsoftAuthURL, false))
	r.handle("GET /microsoft/callback", r.microsoftCallback)
	r.handle("POST /microsoft/get-profile", r.auth(r.microsoftGetProfile, false))
	r.handle("POST /microsoft/import-profile", r.auth(r.microsoftImportProfile, false))
	r.handle("POST /remote-ygg/get-profiles", r.auth(r.remoteYggGetProfiles, false))
	r.handle("POST /remote-ygg/import-profile", r.auth(r.remoteYggImportProfile, false))
	r.handle("POST /remote-ygg/import-profiles", r.auth(r.remoteYggImportProfiles, false))

	r.handle("GET /admin/users", r.auth(r.adminUsers, true))
	r.handle("GET /admin/users/{user_id}/profiles", r.auth(r.adminUserProfiles, true))
	r.handle("POST /admin/users/{user_id}/toggle-admin", r.auth(r.adminToggleUserAdmin, true))
	r.handle("DELETE /admin/users/{user_id}", r.auth(r.adminDeleteUser, true))
	r.handle("POST /admin/users/{user_id}/ban", r.auth(r.adminBanUser, true))
	r.handle("POST /admin/users/{user_id}/unban", r.auth(r.adminUnbanUser, true))
	r.handle("POST /admin/users/reset-password", r.auth(r.adminResetUserPassword, true))
	r.handle("GET /admin/profiles", r.auth(r.adminProfiles, true))
	r.handle("PATCH /admin/profiles/{profile_id}", r.auth(r.adminUpdateProfile, true))
	r.handle("DELETE /admin/profiles/{profile_id}", r.auth(r.adminDeleteProfile, true))
	r.handle("PATCH /admin/profiles/{profile_id}/skin", r.auth(r.adminUpdateProfileSkin, true))
	r.handle("PATCH /admin/profiles/{profile_id}/cape", r.auth(r.adminUpdateProfileCape, true))
	r.handle("GET /admin/textures", r.auth(r.adminTextures, true))
	r.handle("PATCH /admin/textures/{hash}", r.auth(r.adminUpdateTexture, true))
	r.handle("DELETE /admin/textures/{hash}", r.auth(r.adminDeleteTexture, true))
	r.handle("GET /admin/invites", r.auth(r.adminInvites, true))
	r.handle("POST /admin/invites", r.auth(r.adminCreateInvite, true))
	r.handle("DELETE /admin/invites/{code}", r.auth(r.adminDeleteInvite, true))
	r.handle("GET /admin/official-whitelist", r.auth(r.adminOfficialWhitelist, true))
	r.handle("POST /admin/official-whitelist", r.auth(r.adminAddOfficialWhitelist, true))
	r.handle("DELETE /admin/official-whitelist/{username}", r.auth(r.adminRemoveOfficialWhitelist, true))
	r.handle("POST /admin/carousel", r.auth(r.adminUploadCarousel, true))
	r.handle("DELETE /admin/carousel/{filename}", r.auth(r.adminDeleteCarousel, true))
	r.handle("GET /admin/settings/site", r.auth(r.adminGetSiteSettings, true))
	r.handle("POST /admin/settings/site", r.auth(r.adminSaveSiteSettings, true))
	r.handle("GET /admin/settings/{group}", r.auth(r.adminGetSettingsGroup, true))
	r.handle("POST /admin/settings/{group}", r.auth(r.adminSaveSettingsGroup, true))
}

func (r *Router) handle(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	})
}

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
		ctx := context.WithValue(req.Context(), userIDKey, user.ID)
		ctx = context.WithValue(ctx, adminKey, user.IsAdmin)
		next(w, req.WithContext(ctx))
	}
}

func currentUserID(req *http.Request) string {
	v, _ := req.Context().Value(userIDKey).(string)
	return v
}

func decodeJSON(req *http.Request, dst any) error {
	defer req.Body.Close()
	return json.NewDecoder(req.Body).Decode(dst)
}

func multipartFileBytes(req *http.Request, field string, maxBytes int64) ([]byte, error) {
	file, _, err := req.FormFile(field)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "file is required"}
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, util.HTTPError{Status: 400, Detail: "File too large"}
	}
	return data, nil
}

func (r *Router) setSessionCookies(w http.ResponseWriter, access, refresh string) {
	secure := strings.HasPrefix(r.cfg.SiteURL, "https://")
	http.SetCookie(w, &http.Cookie{Name: "access_token", Value: access, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: r.cfg.AccessMinutes * 60})
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: refresh, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: r.cfg.JWTExpireDays * 24 * 3600})
}

func (r *Router) siteLogin(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.site.Login(req.Context(), body["email"], body["password"])
	if err != nil {
		util.Error(w, err)
		return
	}
	r.setSessionCookies(w, res["access_token"].(string), res["refresh_token"].(string))
	util.JSON(w, 200, map[string]any{"user_id": res["user_id"], "is_admin": res["is_admin"]})
}

func (r *Router) siteLogout(w http.ResponseWriter, req *http.Request) {
	if c, err := req.Cookie("refresh_token"); err == nil {
		_ = r.site.RevokeRefresh(req.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "access_token", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Path: "/", MaxAge: -1})
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) register(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	id, err := r.site.Register(req.Context(), body["email"], body["password"], body["username"], body["invite"], body["code"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"id": id})
}

func (r *Router) sendVerificationCode(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	email := body["email"]
	if email == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "email required"})
		return
	}
	res, err := r.site.SendVerificationCode(req.Context(), email, body["type"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) resetPassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if body["email"] == "" || body["password"] == "" || body["code"] == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "email, password and code required"})
		return
	}
	if err := r.site.ResetPassword(req.Context(), body["email"], body["password"], body["code"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) refreshToken(w http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
		return
	}
	res, err := r.site.RotateRefresh(req.Context(), c.Value)
	if err != nil {
		util.Error(w, err)
		return
	}
	r.setSessionCookies(w, res["access_token"].(string), res["refresh_token"].(string))
	util.JSON(w, 200, map[string]any{"is_admin": res["is_admin"]})
}

func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	res, err := r.site.Me(req.Context(), currentUserID(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) updateMe(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.UpdateMe(req.Context(), currentUserID(req), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) deleteMe(w http.ResponseWriter, req *http.Request) {
	userID := currentUserID(req)
	user, err := r.db.GetUserByID(req.Context(), userID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if user.IsAdmin {
		util.Error(w, util.HTTPError{Status: 403, Detail: "管理员不能删除自己的账号"})
		return
	}
	ok, err := r.db.DeleteUser(req.Context(), userID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) changePassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.ChangePassword(req.Context(), currentUserID(req), body["old_password"], body["new_password"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "message": "密码修改成功"})
}

func (r *Router) createProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.site.CreateProfile(req.Context(), currentUserID(req), body["name"], body["model"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) updateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.UpdateProfile(req.Context(), currentUserID(req), req.PathValue("pid"), body["name"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) deleteProfile(w http.ResponseWriter, req *http.Request) {
	if err := r.site.DeleteProfile(req.Context(), currentUserID(req), req.PathValue("pid")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) clearProfileSkin(w http.ResponseWriter, req *http.Request) {
	if err := r.site.ClearProfileTexture(req.Context(), currentUserID(req), req.PathValue("pid"), "skin"); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) clearProfileCape(w http.ResponseWriter, req *http.Request) {
	if err := r.site.ClearProfileTexture(req.Context(), currentUserID(req), req.PathValue("pid"), "cape"); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) listMyProfiles(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := r.site.ListMyProfiles(req.Context(), currentUserID(req), req.URL.Query().Get("cursor"), limit)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) listMyTextures(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := r.site.ListMyTextures(req.Context(), currentUserID(req), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) uploadMyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := multipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(strings.TrimSpace(req.FormValue("texture_type")))
	if textureType == "" {
		textureType = "skin"
	}
	if textureType != "skin" && textureType != "cape" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
		return
	}
	storage, err := service.NewTextureStorage(r.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, err := storage.ProcessAndSave(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	if err := r.db.AddTextureToLibrary(req.Context(), currentUserID(req), hash, textureType, req.FormValue("note"), formBool(req.FormValue("is_public")), database.NormalizeProfileModel(req.FormValue("model"))); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"hash": hash, "texture_type": textureType})
}

func (r *Router) uploadAndApplyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	profileID := strings.TrimSpace(req.FormValue("uuid"))
	textureType := strings.ToLower(strings.TrimSpace(req.FormValue("texture_type")))
	if profileID == "" || textureType == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "uuid and texture_type are required"})
		return
	}
	data, err := multipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	storage, err := service.NewTextureStorage(r.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, err := storage.ProcessAndSave(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	model := database.NormalizeProfileModel(req.FormValue("model"))
	if err := r.db.AddTextureToLibrary(req.Context(), currentUserID(req), hash, textureType, "", formBool(req.FormValue("is_public")), model); err != nil {
		util.Error(w, err)
		return
	}
	if err := r.site.ApplyTextureToProfile(req.Context(), currentUserID(req), profileID, hash, textureType); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "hash": hash, "type": textureType})
}

func (r *Router) textureDetail(w http.ResponseWriter, req *http.Request) {
	res, err := r.site.TextureDetail(req.Context(), currentUserID(req), req.PathValue("hash"), req.PathValue("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) updateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.site.UpdateTexture(req.Context(), currentUserID(req), req.PathValue("hash"), req.PathValue("texture_type"), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) deleteTexture(w http.ResponseWriter, req *http.Request) {
	if err := r.site.DeleteTexture(req.Context(), currentUserID(req), req.PathValue("hash"), req.PathValue("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) addTexture(w http.ResponseWriter, req *http.Request) {
	if err := r.site.AddTextureToWardrobe(req.Context(), currentUserID(req), req.PathValue("hash")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) applyTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.ApplyTextureToProfile(req.Context(), currentUserID(req), body["profile_id"], req.PathValue("hash"), body["texture_type"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) publicLibrary(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := r.site.PublicLibrary(req.Context(), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"), req.URL.Query().Get("q"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) publicSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).Public(req.Context(), r.cfg.SiteURL, r.cfg.APIURL)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) publicCarousel(w http.ResponseWriter, req *http.Request) {
	entries, err := os.ReadDir(r.cfg.CarouselDir)
	if os.IsNotExist(err) {
		util.JSON(w, 200, []string{})
		return
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	var images []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch strings.ToLower(filepath.Ext(name)) {
		case ".png", ".jpg", ".jpeg", ".webp":
			images = append(images, name)
		}
	}
	if images == nil {
		images = []string{}
	}
	util.JSON(w, 200, images)
}

func (r *Router) yggMetadata(w http.ResponseWriter, req *http.Request) {
	res, err := r.ygg.Metadata(req.Context())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) yggAuthenticate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		ClientToken string `json:"clientToken"`
		RequestUser bool   `json:"requestUser"`
	}
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.ygg.Authenticate(req.Context(), body.Username, body.Password, body.ClientToken, body.RequestUser)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) yggRefresh(w http.ResponseWriter, req *http.Request) {
	var body struct {
		AccessToken     string         `json:"accessToken"`
		ClientToken     string         `json:"clientToken"`
		RequestUser     bool           `json:"requestUser"`
		SelectedProfile map[string]any `json:"selectedProfile"`
	}
	_ = decodeJSON(req, &body)
	selected := ""
	if body.SelectedProfile != nil {
		selected, _ = body.SelectedProfile["id"].(string)
	}
	res, err := r.ygg.Refresh(req.Context(), body.AccessToken, body.ClientToken, selected, body.RequestUser)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) yggValidate(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	_ = decodeJSON(req, &body)
	if err := r.ygg.Validate(req.Context(), body["accessToken"], body["clientToken"]); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (r *Router) yggInvalidate(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	_ = decodeJSON(req, &body)
	if body["accessToken"] != "" {
		_ = r.db.DeleteToken(req.Context(), body["accessToken"])
	}
	w.WriteHeader(204)
}

func (r *Router) yggSignout(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(204)
}

func (r *Router) yggJoin(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.ygg.Join(req.Context(), body["accessToken"], body["selectedProfile"], body["serverId"], req.RemoteAddr); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (r *Router) yggHasJoined(w http.ResponseWriter, req *http.Request) {
	username := req.URL.Query().Get("username")
	serverID := req.URL.Query().Get("serverId")
	res, status, err := r.ygg.HasJoined(req.Context(), username, serverID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := (service.Fallback{DB: r.db}).HasJoined(req.Context(), username, serverID, req.URL.Query().Get("ip"))
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, status, res)
}

func (r *Router) yggProfile(w http.ResponseWriter, req *http.Request) {
	unsigned := req.URL.Query().Get("unsigned") != "false"
	res, status, err := r.ygg.Profile(req.Context(), req.PathValue("uuid"), unsigned)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := (service.Fallback{DB: r.db}).GetProfile(req.Context(), req.PathValue("uuid"), unsigned)
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) lookupName(w http.ResponseWriter, req *http.Request) {
	playerName := req.PathValue("playerName")
	res, status, err := r.ygg.LookupName(req.Context(), playerName)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		var resp *service.FallbackResponse
		if strings.HasPrefix(req.URL.Path, "/api/minecraft/profile/lookup/name/") || strings.HasPrefix(req.URL.Path, "/minecraft/profile/lookup/name/") {
			resp, err = (service.Fallback{DB: r.db}).ServicesLookup(req.Context(), playerName)
		} else {
			resp, err = (service.Fallback{DB: r.db}).GetProfileByName(req.Context(), playerName)
		}
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) lookupNames(w http.ResponseWriter, req *http.Request) {
	var names []string
	if err := decodeJSON(req, &names); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Request body must be an array"})
		return
	}
	profiles, err := r.db.SearchProfilesByNames(req.Context(), names, 100)
	if err != nil {
		util.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, map[string]any{"id": p.ID, "name": p.Name})
	}
	found := map[string]bool{}
	for _, p := range profiles {
		found[strings.ToLower(p.Name)] = true
	}
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if !found[strings.ToLower(name)] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		fallbackProfiles, err := (service.Fallback{DB: r.db}).BulkLookup(req.Context(), missing)
		if err != nil {
			util.Error(w, err)
			return
		}
		if len(fallbackProfiles) > 0 {
			out = append(out, fallbackProfiles...)
		}
	}
	util.JSON(w, 200, out)
}

func writeFallback(w http.ResponseWriter, resp *service.FallbackResponse) bool {
	if resp == nil {
		return false
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(resp.Body)
	return true
}

func (r *Router) microsoftAuthURL(w http.ResponseWriter, req *http.Request) {
	state := util.RandomUUIDNoDash() + util.RandomUUIDNoDash()
	clientID, _ := r.db.GetSetting(req.Context(), "microsoft_client_id", "")
	redirectURI, _ := r.db.GetSetting(req.Context(), "microsoft_redirect_uri", strings.TrimRight(r.cfg.APIURL, "/")+"/microsoft/callback")
	MicrosoftImportStates.Put(state, map[string]any{"user_id": currentUserID(req), "kind": "oauth_state"}, 10*time.Minute)
	util.JSON(w, 200, map[string]any{
		"auth_url": service.MicrosoftAuthorizationURL(clientID, redirectURI, state),
		"state":    state,
	})
}

func (r *Router) microsoftCallback(w http.ResponseWriter, req *http.Request) {
	siteURL := strings.TrimRight(r.cfg.SiteURL, "/")
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
	raw := MicrosoftImportStates.Pop(state)
	session, ok := raw.(map[string]any)
	if !ok || session["kind"] != "oauth_state" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid or expired state parameter"})
		return
	}
	clientID, _ := r.db.GetSetting(req.Context(), "microsoft_client_id", "")
	clientSecret, _ := r.db.GetSetting(req.Context(), "microsoft_client_secret", "")
	redirectURI, _ := r.db.GetSetting(req.Context(), "microsoft_redirect_uri", strings.TrimRight(r.cfg.APIURL, "/")+"/microsoft/callback")
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		http.Redirect(w, req, siteURL+"/dashboard/roles?error=auth_failed", http.StatusFound)
		return
	}
	result, err := (service.MicrosoftAuthFlow{Client: service.MicrosoftHTTPClient{
		ClientID: clientID, ClientSecret: clientSecret, RedirectURI: redirectURI,
	}}).Complete(req.Context(), code)
	if err != nil || result["profile"] == nil {
		http.Redirect(w, req, siteURL+"/dashboard/roles?error=auth_failed", http.StatusFound)
		return
	}
	token := util.RandomUUIDNoDash() + util.RandomUUIDNoDash()
	MicrosoftImportStates.Put(token, map[string]any{"user_id": session["user_id"], "kind": "profile", "profile": result}, 5*time.Minute)
	http.Redirect(w, req, siteURL+"/dashboard/roles?ms_token="+token, http.StatusFound)
}

func (r *Router) microsoftGetProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	raw := MicrosoftImportStates.Pop(body["ms_token"])
	session, ok := raw.(map[string]any)
	if !ok || session["kind"] != "profile" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid or expired token"})
		return
	}
	if session["user_id"] != currentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "Unauthorized"})
		return
	}
	flowProfile, _ := session["profile"].(map[string]any)
	mcProfile, _ := flowProfile["profile"].(map[string]any)
	verified := map[string]any{
		"id":    mcProfile["id"],
		"name":  mcProfile["name"],
		"skins": valueOrAny(mcProfile["skins"], []any{}),
		"capes": valueOrAny(mcProfile["capes"], []any{}),
	}
	importToken := util.RandomUUIDNoDash() + util.RandomUUIDNoDash()
	MicrosoftImportStates.Put(importToken, map[string]any{
		"user_id": currentUserID(req),
		"kind":    "import",
		"profile": verified,
	}, 5*time.Minute)
	util.JSON(w, 200, map[string]any{
		"profile":      verified,
		"has_game":     valueOrAny(flowProfile["has_game"], false),
		"import_token": importToken,
	})
}

func (r *Router) yggUploadTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := bearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Bearer token required"})
		return
	}
	tok, err := r.db.GetToken(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok == nil || tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Invalid token"})
		return
	}
	profile, err := r.db.GetProfileByID(req.Context(), *tok.ProfileID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if profile == nil || profile.UserID != tok.UserID {
		util.Error(w, util.HTTPError{Status: 403, Detail: "Profile not yours"})
		return
	}
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := multipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(req.PathValue("texture_type"))
	storage, err := service.NewTextureStorage(r.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, err := storage.ProcessAndSave(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	if err := r.db.AddTextureToLibrary(req.Context(), tok.UserID, hash, textureType, "", false, database.NormalizeProfileModel(req.FormValue("model"))); err != nil {
		util.Error(w, err)
		return
	}
	if textureType == "skin" {
		if err := r.db.UpdateProfileSkin(req.Context(), profile.ID, &hash); err != nil {
			util.Error(w, err)
			return
		}
		_ = r.db.UpdateProfileModel(req.Context(), profile.ID, database.NormalizeProfileModel(req.FormValue("model")))
	} else if textureType == "cape" {
		if err := r.db.UpdateProfileCape(req.Context(), profile.ID, &hash); err != nil {
			util.Error(w, err)
			return
		}
	} else {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
		return
	}
	util.JSON(w, 200, map[string]any{"hash": hash})
}

func (r *Router) yggDeleteTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := bearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Bearer token required"})
		return
	}
	tok, err := r.db.GetToken(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok == nil || tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Invalid token"})
		return
	}
	switch strings.ToLower(req.PathValue("texture_type")) {
	case "skin":
		err = r.db.UpdateProfileSkin(req.Context(), *tok.ProfileID, nil)
	case "cape":
		err = r.db.UpdateProfileCape(req.Context(), *tok.ProfileID, nil)
	default:
		err = util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (r *Router) microsoftImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	token := body["ms_token"]
	raw := MicrosoftImportStates.Pop(token)
	if raw == nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	session, ok := raw.(map[string]any)
	if !ok {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	if session["kind"] != "import" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	if session["user_id"] != currentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "not allowed"})
		return
	}
	profile, ok := session["profile"].(map[string]any)
	if !ok {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	profileID, _ := profile["id"].(string)
	profileName, _ := profile["name"].(string)
	var assets []service.TextureAsset
	if skins, ok := profile["skins"].([]map[string]string); ok {
		for _, skin := range skins {
			assets = append(assets, service.TextureAsset{URL: skin["url"], Kind: "skin", Variant: skin["variant"]})
		}
	} else if skins, ok := profile["skins"].([]any); ok {
		for _, rawSkin := range skins {
			if skin, ok := rawSkin.(map[string]any); ok {
				u, _ := skin["url"].(string)
				variant, _ := skin["variant"].(string)
				assets = append(assets, service.TextureAsset{URL: u, Kind: "skin", Variant: variant})
			}
		}
	}
	if capes, ok := profile["capes"].([]map[string]string); ok {
		for _, cape := range capes {
			assets = append(assets, service.TextureAsset{URL: cape["url"], Kind: "cape"})
		}
	} else if capes, ok := profile["capes"].([]any); ok {
		for _, rawCape := range capes {
			if cape, ok := rawCape.(map[string]any); ok {
				u, _ := cape["url"].(string)
				assets = append(assets, service.TextureAsset{URL: u, Kind: "cape"})
			}
		}
	}
	res, err := (service.ImportService{DB: r.db}).ImportProfile(req.Context(), currentUserID(req), profileID, profileName, assets)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) remoteYggGetProfiles(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profiles, _ := body["profiles"].([]any)
	if profiles == nil {
		profiles = []any{}
	}
	util.JSON(w, 200, map[string]any{"profiles": profiles})
}

func (r *Router) remoteYggImportProfiles(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profiles, err := parseImportProfiles(body["profiles"])
	if err != nil {
		util.Error(w, err)
		return
	}
	importer := service.ImportService{DB: r.db}
	res := importer.ImportProfiles(req.Context(), currentUserID(req), profiles, func(ctx context.Context, id string) ([]service.TextureAsset, error) {
		return []service.TextureAsset{{URL: id + ":skin", Kind: "skin", Variant: "classic"}}, nil
	})
	util.JSON(w, 200, res)
}

func (r *Router) remoteYggImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := strings.TrimSpace(body["profile_id"])
	profileName := strings.TrimSpace(body["profile_name"])
	if profileID == "" || profileName == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "profile_id and profile_name are required"})
		return
	}
	importer := service.ImportService{DB: r.db}
	res, err := importer.ImportProfile(req.Context(), currentUserID(req), profileID, profileName, []service.TextureAsset{{URL: profileID + ":skin", Kind: "skin", Variant: "classic"}})
	if err != nil {
		util.Error(w, err)
		return
	}
	profile := res["profile"].(map[string]any)
	util.JSON(w, 200, map[string]any{"id": profile["id"], "name": profile["name"]})
}

func (r *Router) adminUsers(w http.ResponseWriter, req *http.Request) {
	cursor, err := util.DecodeCursor(req.URL.Query().Get("cursor"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	res, err := r.db.ListUsers(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminToggleUserAdmin(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if targetID == currentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot change your own admin status"})
		return
	}
	next, err := r.db.ToggleAdmin(req.Context(), targetID)
	if err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "is_admin": next})
}

func (r *Router) adminDeleteUser(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if targetID == currentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot delete yourself"})
		return
	}
	ok, err := r.db.DeleteUser(req.Context(), targetID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminUserProfiles(w http.ResponseWriter, req *http.Request) {
	res, err := r.db.ListProfilesByUser(req.Context(), req.PathValue("user_id"), util.ClampLimit(req.URL.Query().Get("limit")), req.URL.Query().Get("cursor"))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminBanUser(w http.ResponseWriter, req *http.Request) {
	var body map[string]int64
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	until, ok := body["banned_until"]
	if !ok || until < time.Now().Add(-24*time.Hour).UnixMilli() {
		util.Error(w, util.HTTPError{Status: 400, Detail: "banned_until is required"})
		return
	}
	if err := r.db.BanUser(req.Context(), req.PathValue("user_id"), until); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "banned_until": until})
}

func (r *Router) adminUnbanUser(w http.ResponseWriter, req *http.Request) {
	user, err := r.db.GetUserByID(req.Context(), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := r.db.UnbanUser(req.Context(), user.ID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminResetUserPassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	userID := body["user_id"]
	newPassword := body["new_password"]
	if userID == "" || newPassword == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "user_id and new_password required"})
		return
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		util.Error(w, err)
		return
	}
	ok, err := r.db.UpdatePasswordAndRevokeRefresh(req.Context(), userID, hash)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminProfiles(w http.ResponseWriter, req *http.Request) {
	cursor, err := util.DecodeCursor(req.URL.Query().Get("cursor"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	res, err := r.db.ListAllProfiles(req.Context(), util.ClampLimit(req.URL.Query().Get("limit")), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminUpdateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := req.PathValue("profile_id")
	p, err := r.db.GetProfileByID(req.Context(), profileID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if p == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	if body["name"] != "" {
		if !util.ValidProfileName(body["name"]) {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid profile name"})
			return
		}
		ok, err := r.db.UpdateProfileName(req.Context(), profileID, body["name"])
		if err != nil {
			if database.IsProfileNameConflict(err) {
				util.Error(w, util.HTTPError{Status: 409, Detail: "profile name already exists"})
				return
			}
			util.Error(w, err)
			return
		}
		if !ok {
			util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
			return
		}
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminDeleteProfile(w http.ResponseWriter, req *http.Request) {
	ok, err := r.db.DeleteProfileCascade(req.Context(), req.PathValue("profile_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminUpdateProfileSkin(w http.ResponseWriter, req *http.Request) {
	r.adminSetProfileTexture(w, req, "skin")
}

func (r *Router) adminUpdateProfileCape(w http.ResponseWriter, req *http.Request) {
	r.adminSetProfileTexture(w, req, "cape")
}

func (r *Router) adminSetProfileTexture(w http.ResponseWriter, req *http.Request, typ string) {
	var body map[string]*string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := req.PathValue("profile_id")
	if p, err := r.db.GetProfileByID(req.Context(), profileID); err != nil {
		util.Error(w, err)
		return
	} else if p == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	if typ == "skin" {
		if err := r.db.UpdateProfileSkin(req.Context(), profileID, body["hash"]); err != nil {
			util.Error(w, err)
			return
		}
	} else {
		if err := r.db.UpdateProfileCape(req.Context(), profileID, body["hash"]); err != nil {
			util.Error(w, err)
			return
		}
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminTextures(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastHash, err := cursorCreatedHash(req.URL.Query().Get("cursor"), "last_skin_hash")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := r.db.ListAllTextures(req.Context(), util.ClampLimit(req.URL.Query().Get("limit")), lastCreated, lastHash, strings.TrimSpace(req.URL.Query().Get("q")), req.URL.Query().Get("type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminUpdateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	hash := req.PathValue("hash")
	updated := false
	if v, ok := body["note"].(string); ok {
		if err := r.db.AdminUpdateTextureNote(req.Context(), hash, v); err != nil {
			if err == database.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if v, ok := body["model"].(string); ok {
		if v != "default" && v != "slim" {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid model"})
			return
		}
		if err := r.db.AdminUpdateTextureModel(req.Context(), hash, v); err != nil {
			if err == database.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if v, ok := body["is_public"]; ok {
		if !validPublicValue(v) {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid is_public"})
			return
		}
		pub := publicBool(v)
		if err := r.db.AdminUpdateTexturePublic(req.Context(), hash, pub); err != nil {
			if err == database.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if !updated {
		util.Error(w, util.HTTPError{Status: 400, Detail: "至少需要一个更新字段: model, note, is_public"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminDeleteTexture(w http.ResponseWriter, req *http.Request) {
	force := req.URL.Query().Get("force") == "true"
	typ := req.URL.Query().Get("type")
	if typ == "" {
		typ = "skin"
	}
	if err := r.db.AdminDeleteTexture(req.Context(), req.PathValue("hash"), typ, req.URL.Query().Get("user_id"), force); err != nil {
		if strings.Contains(err.Error(), "user_id") {
			util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
			return
		}
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"success": true})
}

func (r *Router) adminInvites(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastCode, err := cursorCreatedHash(req.URL.Query().Get("cursor"), "last_code")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := r.db.ListInvites(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), lastCreated, lastCode)
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminCreateInvite(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	_ = decodeJSON(req, &body)
	code, _ := body["code"].(string)
	if code == "" {
		code = util.RandomUUIDNoDash() + util.RandomUUIDNoDash()[:8]
	}
	if len(code) < 4 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invite code too short"})
		return
	}
	total := 1
	if v, ok := body["total_uses"].(float64); ok {
		total = int(v)
	}
	note, _ := body["note"].(string)
	if err := r.db.CreateInvite(req.Context(), code, total, note); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"code": code, "total_uses": total, "note": note})
}

func (r *Router) adminDeleteInvite(w http.ResponseWriter, req *http.Request) {
	if err := r.db.DeleteInvite(req.Context(), req.PathValue("code")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := parsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	users, err := r.db.ListWhitelistUsers(req.Context(), endpointID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if users == nil {
		users = []map[string]any{}
	}
	util.JSON(w, 200, map[string]any{"items": users})
}

func (r *Router) adminAddOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	username := strings.TrimSpace(asString(body["username"]))
	endpointID, err := parsePositiveInt(fmt.Sprint(body["endpoint_id"]))
	if username == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "username is required"})
		return
	}
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	if err := r.db.AddWhitelistUser(req.Context(), username, endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminRemoveOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := parsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	if err := r.db.RemoveWhitelistUser(req.Context(), req.PathValue("username"), endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminUploadCarousel(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(6 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "file is required"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		util.Error(w, util.HTTPError{Status: 400, Detail: "Unsupported file format"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		util.Error(w, err)
		return
	}
	if len(data) > 5*1024*1024 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "File too large"})
		return
	}
	if err := os.MkdirAll(r.cfg.CarouselDir, 0o755); err != nil {
		util.Error(w, err)
		return
	}
	filename := util.RandomUUIDNoDash() + ext
	if err := os.WriteFile(filepath.Join(r.cfg.CarouselDir, filename), data, 0o644); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"filename": filename})
}

func (r *Router) adminDeleteCarousel(w http.ResponseWriter, req *http.Request) {
	filename := filepath.Base(req.PathValue("filename"))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid filename"})
		return
	}
	err := os.Remove(filepath.Join(r.cfg.CarouselDir, filename))
	if err != nil && !os.IsNotExist(err) {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminGetSiteSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).GetGroup(req.Context(), "site")
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) adminSaveSiteSettings(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (service.Settings{DB: r.db}).SaveGroup(req.Context(), "site", body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminGetSettingsGroup(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).GetGroup(req.Context(), req.PathValue("group"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) adminSaveSettingsGroup(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (service.Settings{DB: r.db}).SaveGroup(req.Context(), req.PathValue("group"), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func asMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

func cursorCreatedHash(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	var created *int64
	switch v := m["last_created_at"].(type) {
	case float64:
		x := int64(v)
		created = &x
	case int64:
		created = &v
	}
	hash, _ := m[hashKey].(string)
	return created, hash, nil
}

func publicBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func validPublicValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return true
	case float64:
		return x == 0 || x == 1
	case int:
		return x == 0 || x == 1
	case string:
		return x == "true" || x == "false" || x == "0" || x == "1"
	default:
		return false
	}
}

func parseImportProfiles(raw any) ([]map[string]string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "profiles must be a list"}
	}
	if len(items) == 0 {
		return nil, util.HTTPError{Status: 400, Detail: "profiles cannot be empty"}
	}
	out := make([]map[string]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Detail: "profiles must be a list"}
		}
		out = append(out, map[string]string{
			"profile_id":   strings.TrimSpace(asString(m["profile_id"])),
			"profile_name": strings.TrimSpace(asString(m["profile_name"])),
		})
	}
	return out, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func valueOrAny(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive int")
	}
	return n, nil
}

func bearerToken(req *http.Request) (string, bool) {
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token, token != ""
}

func formBool(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
}
