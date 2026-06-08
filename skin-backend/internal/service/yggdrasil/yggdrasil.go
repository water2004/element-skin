package yggdrasil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

type Yggdrasil struct {
	DB     *database.DB
	Cfg    config.Config
	Signer *Signer
}

func New(db *database.DB, cfg config.Config) (Yggdrasil, error) {
	signer, err := NewSigner(cfg)
	if err != nil {
		return Yggdrasil{}, err
	}
	return Yggdrasil{DB: db, Cfg: cfg, Signer: signer}, nil
}

func yggErr(status int, code, msg string) error {
	return util.HTTPError{Status: status, Detail: msg, YggError: code}
}

func (y Yggdrasil) Metadata(ctx context.Context) (map[string]any, error) {
	signer, err := y.signer()
	if err != nil {
		return nil, err
	}
	name, _ := y.DB.Settings.Get(ctx, "site_name", "皮肤站")
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
		"signaturePublickey": signer.PublicKeyPEM(),
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
		profiles, err = y.DB.Profiles.GetByUser(ctx, u.ID, 100)
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
	if err := y.DB.Tokens.Add(ctx, model.Token{AccessToken: access, ClientToken: clientToken, UserID: u.ID, ProfileID: pid, CreatedAt: database.NowMS()}); err != nil {
		return nil, err
	}
	_ = y.DB.Tokens.Cleanup(ctx, u.ID, database.NowMS()-15*24*3600*1000, 5)
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
	u, err := y.DB.Users.GetByEmail(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	var p *model.Profile
	if u == nil {
		p, err = y.DB.Profiles.GetByName(ctx, username)
		if err != nil {
			return nil, nil, err
		}
		if p != nil {
			u, err = y.DB.Users.GetByID(ctx, p.UserID)
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
	t, err := y.DB.Tokens.Get(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if t == nil || (clientToken != "" && clientToken != t.ClientToken) {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	_ = y.DB.Tokens.Delete(ctx, accessToken)
	newProfile := t.ProfileID
	var selected map[string]any
	if selectedID != "" {
		selectedID = util.StripUUIDDashes(selectedID)
		if t.ProfileID != nil {
			return nil, yggErr(400, "IllegalArgumentException", "Access token already has a profile assigned.")
		}
		ok, err := y.DB.Profiles.VerifyOwnership(ctx, t.UserID, selectedID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid profile.")
		}
		newProfile = &selectedID
	}
	if newProfile != nil {
		p, _ := y.DB.Profiles.GetByID(ctx, *newProfile)
		if p != nil {
			selected = map[string]any{"id": p.ID, "name": p.Name}
		}
	}
	newAccess, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if err := y.DB.Tokens.Add(ctx, model.Token{AccessToken: newAccess, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: newProfile, CreatedAt: database.NowMS()}); err != nil {
		return nil, err
	}
	resp := map[string]any{"accessToken": newAccess, "clientToken": t.ClientToken}
	if selected != nil {
		resp["selectedProfile"] = selected
	}
	if requestUser {
		u, _ := y.DB.Users.GetByID(ctx, t.UserID)
		if u != nil {
			resp["user"] = map[string]any{"id": u.ID, "properties": []map[string]any{{"name": "preferredLanguage", "value": u.PreferredLanguage}}}
		}
	}
	return resp, nil
}

func (y Yggdrasil) Validate(ctx context.Context, access, client string) error {
	t, err := y.DB.Tokens.Get(ctx, access)
	if err != nil {
		return err
	}
	if t == nil || (client != "" && client != t.ClientToken) || database.NowMS()-t.CreatedAt > 15*24*3600*1000 {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return nil
}

func (y Yggdrasil) Join(ctx context.Context, access, profileID, serverID, ip string) error {
	t, err := y.DB.Tokens.Get(ctx, access)
	if err != nil {
		return err
	}
	if t == nil || t.ProfileID == nil || *t.ProfileID != profileID {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return y.DB.Tokens.ReplaceSession(ctx, model.Session{ServerID: serverID, AccessToken: access, IP: &ip, CreatedAt: database.NowMS()})
}

func (y Yggdrasil) HasJoined(ctx context.Context, username, serverID string) (map[string]any, int, error) {
	s, err := y.DB.Tokens.GetSession(ctx, serverID)
	if err != nil {
		return nil, 0, err
	}
	if s == nil || database.NowMS()-s.CreatedAt > 30000 {
		return nil, 204, nil
	}
	t, err := y.DB.Tokens.Get(ctx, s.AccessToken)
	if err != nil {
		return nil, 0, err
	}
	if t == nil || t.ProfileID == nil {
		return nil, 204, nil
	}
	p, err := y.DB.Profiles.GetByID(ctx, *t.ProfileID)
	if err != nil {
		return nil, 0, err
	}
	if p == nil || p.Name != username {
		return nil, 204, nil
	}
	if banned, err := y.DB.Users.IsBanned(ctx, p.UserID); err != nil {
		return nil, 0, err
	} else if banned {
		return nil, 0, yggErr(403, "ForbiddenOperationException", "Account is banned. Please contact administrator.")
	}
	body, err := y.ProfileJSON(*p, true)
	if err != nil {
		return nil, 0, err
	}
	return body, 200, nil
}

func (y Yggdrasil) Profile(ctx context.Context, id string, unsigned bool) (map[string]any, int, error) {
	p, err := y.DB.Profiles.GetByID(ctx, util.StripUUIDDashes(id))
	if err != nil {
		return nil, 0, err
	}
	if p == nil {
		return nil, 204, nil
	}
	body, err := y.ProfileJSON(*p, !unsigned)
	if err != nil {
		return nil, 0, err
	}
	return body, 200, nil
}

func (y Yggdrasil) ProfileJSON(p model.Profile, sign bool) (map[string]any, error) {
	base := strings.TrimRight(y.publicTextureBaseURL(), "/") + "/static/textures/"
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
		signer, err := y.signer()
		if err != nil {
			return nil, err
		}
		signature, err := signer.SignPropertyValue(prop["value"].(string))
		if err != nil {
			return nil, err
		}
		prop["signature"] = signature
	}
	return map[string]any{"id": p.ID, "name": p.Name, "properties": []map[string]any{prop, {"name": "uploadableTextures", "value": "skin,cape"}}}, nil
}

func (y Yggdrasil) signer() (*Signer, error) {
	if y.Signer != nil {
		return y.Signer, nil
	}
	return NewSigner(y.Cfg)
}

func (y Yggdrasil) publicTextureBaseURL() string {
	if y.Cfg.APIURL != "" {
		return y.Cfg.APIURL
	}
	return y.Cfg.SiteURL
}

func (y Yggdrasil) LookupName(ctx context.Context, name string) (map[string]any, int, error) {
	p, err := y.DB.Profiles.GetByName(ctx, name)
	if err != nil {
		return nil, 0, err
	}
	if p == nil {
		return nil, 204, nil
	}
	return map[string]any{"id": p.ID, "name": p.Name}, 200, nil
}
