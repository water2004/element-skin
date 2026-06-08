package yggdrasil

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

type Yggdrasil struct {
	DB  *database.DB
	Cfg config.Config
}

func yggErr(status int, code, msg string) error {
	return util.HTTPError{Status: status, Detail: msg, YggError: code}
}

func (y Yggdrasil) Metadata(ctx context.Context) (map[string]any, error) {
	name, _ := y.DB.GetSetting(ctx, "site_name", "皮肤站")
	site := strings.TrimRight(y.Cfg.SiteURL, "/")
	host := strings.TrimPrefix(strings.TrimPrefix(site, "https://"), "http://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return map[string]any{
		"meta": map[string]any{
			"serverName": name, "implementationName": "element-skin", "implementationVersion": "go",
			"links":                   map[string]any{"homepage": site + "/", "register": site + "/register/"},
			"feature.non_email_login": true,
		},
		"skinDomains":        append(y.Cfg.FallbackDomains, host),
		"signaturePublickey": "element-skin-go-signature-placeholder",
	}, nil
}

func (y Yggdrasil) Authenticate(ctx context.Context, username, password, clientToken string, requestUser bool) (map[string]any, error) {
	u, loginProfile, err := y.verifyCredentials(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	}
	access, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if clientToken == "" {
		clientToken = access
	}
	var profiles []model.Profile
	var selected *model.Profile
	if loginProfile != nil {
		profiles = []model.Profile{*loginProfile}
		selected = loginProfile
	} else {
		profiles, err = y.DB.GetProfilesByUser(ctx, u.ID, 100)
		if err != nil {
			return nil, err
		}
		if len(profiles) == 1 {
			selected = &profiles[0]
		}
	}
	var pid *string
	if selected != nil {
		pid = &selected.ID
	}
	if err := y.DB.AddToken(ctx, model.Token{AccessToken: access, ClientToken: clientToken, UserID: u.ID, ProfileID: pid, CreatedAt: database.NowMS()}); err != nil {
		return nil, err
	}
	_ = y.DB.CleanupTokens(ctx, u.ID, database.NowMS()-15*24*3600*1000, 5)
	available := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		available = append(available, map[string]any{"id": p.ID, "name": p.Name})
	}
	resp := map[string]any{"accessToken": access, "clientToken": clientToken, "availableProfiles": available}
	if selected != nil {
		resp["selectedProfile"] = map[string]any{"id": selected.ID, "name": selected.Name}
	}
	if requestUser {
		resp["user"] = map[string]any{"id": u.ID, "properties": []map[string]any{{"name": "preferredLanguage", "value": u.PreferredLanguage}}}
	}
	return resp, nil
}

func (y Yggdrasil) verifyCredentials(ctx context.Context, username, password string) (*model.User, *model.Profile, error) {
	u, err := y.DB.GetUserByEmail(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	var p *model.Profile
	if u == nil {
		p, err = y.DB.GetProfileByName(ctx, username)
		if err != nil {
			return nil, nil, err
		}
		if p != nil {
			u, err = y.DB.GetUserByID(ctx, p.UserID)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if u == nil || !util.VerifyPassword(password, u.Password) {
		return nil, nil, nil
	}
	return u, p, nil
}

func (y Yggdrasil) Refresh(ctx context.Context, accessToken, clientToken, selectedID string, requestUser bool) (map[string]any, error) {
	t, err := y.DB.GetToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if t == nil || (clientToken != "" && clientToken != t.ClientToken) {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	_ = y.DB.DeleteToken(ctx, accessToken)
	newProfile := t.ProfileID
	var selected map[string]any
	if selectedID != "" {
		selectedID = util.StripUUIDDashes(selectedID)
		if t.ProfileID != nil {
			return nil, yggErr(400, "IllegalArgumentException", "Access token already has a profile assigned.")
		}
		ok, err := y.DB.VerifyProfileOwnership(ctx, t.UserID, selectedID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid profile.")
		}
		newProfile = &selectedID
	}
	if newProfile != nil {
		p, _ := y.DB.GetProfileByID(ctx, *newProfile)
		if p != nil {
			selected = map[string]any{"id": p.ID, "name": p.Name}
		}
	}
	newAccess, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if err := y.DB.AddToken(ctx, model.Token{AccessToken: newAccess, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: newProfile, CreatedAt: database.NowMS()}); err != nil {
		return nil, err
	}
	resp := map[string]any{"accessToken": newAccess, "clientToken": t.ClientToken}
	if selected != nil {
		resp["selectedProfile"] = selected
	}
	if requestUser {
		u, _ := y.DB.GetUserByID(ctx, t.UserID)
		if u != nil {
			resp["user"] = map[string]any{"id": u.ID, "properties": []map[string]any{{"name": "preferredLanguage", "value": u.PreferredLanguage}}}
		}
	}
	return resp, nil
}

func (y Yggdrasil) Validate(ctx context.Context, access, client string) error {
	t, err := y.DB.GetToken(ctx, access)
	if err != nil {
		return err
	}
	if t == nil || (client != "" && client != t.ClientToken) || database.NowMS()-t.CreatedAt > 15*24*3600*1000 {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return nil
}

func (y Yggdrasil) Join(ctx context.Context, access, profileID, serverID, ip string) error {
	t, err := y.DB.GetToken(ctx, access)
	if err != nil {
		return err
	}
	if t == nil || t.ProfileID == nil || *t.ProfileID != profileID {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return y.DB.ReplaceSession(ctx, model.Session{ServerID: serverID, AccessToken: access, IP: &ip, CreatedAt: database.NowMS()})
}

func (y Yggdrasil) HasJoined(ctx context.Context, username, serverID string) (map[string]any, int, error) {
	s, err := y.DB.GetSession(ctx, serverID)
	if err != nil {
		return nil, 0, err
	}
	if s == nil || database.NowMS()-s.CreatedAt > 30000 {
		return nil, 204, nil
	}
	t, err := y.DB.GetToken(ctx, s.AccessToken)
	if err != nil {
		return nil, 0, err
	}
	if t == nil || t.ProfileID == nil {
		return nil, 204, nil
	}
	p, err := y.DB.GetProfileByID(ctx, *t.ProfileID)
	if err != nil {
		return nil, 0, err
	}
	if p == nil || p.Name != username {
		return nil, 204, nil
	}
	if banned, err := y.DB.IsBanned(ctx, p.UserID); err != nil {
		return nil, 0, err
	} else if banned {
		return nil, 0, yggErr(403, "ForbiddenOperationException", "Account is banned. Please contact administrator.")
	}
	return y.ProfileJSON(*p, true), 200, nil
}

func (y Yggdrasil) Profile(ctx context.Context, id string, unsigned bool) (map[string]any, int, error) {
	p, err := y.DB.GetProfileByID(ctx, util.StripUUIDDashes(id))
	if err != nil {
		return nil, 0, err
	}
	if p == nil {
		return nil, 204, nil
	}
	return y.ProfileJSON(*p, !unsigned), 200, nil
}

func (y Yggdrasil) ProfileJSON(p model.Profile, sign bool) map[string]any {
	base := strings.TrimRight(y.Cfg.SiteURL, "/") + "/static/textures/"
	textures := map[string]any{}
	if p.SkinHash != nil {
		skin := map[string]any{"url": base + *p.SkinHash + ".png"}
		if p.TextureModel == "slim" {
			skin["metadata"] = map[string]any{"model": "slim"}
		}
		textures["SKIN"] = skin
	}
	if p.CapeHash != nil {
		textures["CAPE"] = map[string]any{"url": base + *p.CapeHash + ".png"}
	}
	payload := map[string]any{"timestamp": time.Now().UnixMilli(), "profileId": p.ID, "profileName": p.Name, "textures": textures}
	b, _ := json.Marshal(payload)
	prop := map[string]any{"name": "textures", "value": base64.StdEncoding.EncodeToString(b)}
	if sign {
		sum := sha256.Sum256(b)
		prop["signature"] = hex.EncodeToString(sum[:])
	}
	return map[string]any{"id": p.ID, "name": p.Name, "properties": []map[string]any{prop, {"name": "uploadableTextures", "value": "skin,cape"}}}
}

func (y Yggdrasil) LookupName(ctx context.Context, name string) (map[string]any, int, error) {
	p, err := y.DB.GetProfileByName(ctx, name)
	if err != nil {
		return nil, 0, err
	}
	if p == nil {
		return nil, 204, nil
	}
	return map[string]any{"id": p.ID, "name": p.Name}, 200, nil
}
