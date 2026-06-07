package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

type Site struct {
	DB  *database.DB
	Cfg config.Config
}

func (s Site) Login(ctx context.Context, email, password string) (map[string]any, error) {
	user, err := s.DB.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil || !util.VerifyPassword(password, user.Password) {
		return nil, util.HTTPError{Status: 401, Detail: "Invalid credentials"}
	}
	return s.issueSession(ctx, user.ID, user.IsAdmin, map[string]any{"user_id": user.ID})
}

func (s Site) Register(ctx context.Context, email, password, username, invite, code string) (string, error) {
	email = strings.TrimSpace(email)
	username = strings.TrimSpace(username)
	if username == "" {
		return "", util.HTTPError{Status: 400, Detail: "Username is required"}
	}
	if !validEmail(email) {
		return "", util.HTTPError{Status: 400, Detail: "Invalid email format"}
	}
	if taken, err := s.DB.IsDisplayNameTaken(ctx, username, ""); err != nil {
		return "", err
	} else if taken {
		return "", util.HTTPError{Status: 400, Detail: "Username already exists"}
	}
	if existing, err := s.DB.GetUserByEmail(ctx, email); err != nil {
		return "", err
	} else if existing != nil {
		return "", util.HTTPError{Status: 400, Detail: "Email already registered"}
	}
	if strong, _ := s.DB.GetSetting(ctx, "enable_strong_password_check", "false"); strong == "true" {
		if errs := util.ValidateStrongPassword(password); len(errs) > 0 {
			return "", util.HTTPError{Status: 400, Detail: util.JoinPasswordErrors(errs)}
		}
	}
	if allow, _ := s.DB.GetSetting(ctx, "allow_register", "true"); allow != "true" {
		return "", util.HTTPError{Status: 403, Detail: "registration is disabled"}
	}
	if enabled, _ := s.DB.GetSetting(ctx, "email_verify_enabled", "false"); enabled == "true" {
		if code == "" {
			return "", util.HTTPError{Status: 400, Detail: "Verification code required"}
		}
		ok, err := s.VerifyCode(ctx, email, code, "register")
		if err != nil {
			return "", err
		}
		if !ok {
			return "", util.HTTPError{Status: 400, Detail: "Invalid or expired verification code"}
		}
		defer s.DB.DeleteVerificationCode(ctx, email, "register")
	}
	requireInvite, _ := s.DB.GetSetting(ctx, "require_invite", "false")
	inviteCode := ""
	if requireInvite == "true" {
		if invite == "" {
			return "", util.HTTPError{Status: 400, Detail: "invite code required"}
		}
		inv, err := s.DB.GetInvite(ctx, invite)
		if err != nil {
			return "", err
		}
		if inv == nil {
			return "", util.HTTPError{Status: 400, Detail: "invalid invite code"}
		}
		if inv.TotalUses != nil && inv.UsedCount >= *inv.TotalUses {
			return "", util.HTTPError{Status: 400, Detail: "invite code has no remaining uses"}
		}
		inviteCode = invite
	}
	count, err := s.DB.CountUsers(ctx)
	if err != nil {
		return "", err
	}
	mode, _ := s.DB.GetSetting(ctx, "profile_uuid_mode", "random")
	base := regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(strings.Split(email, "@")[0], "_")
	if len(base) > 12 {
		base = base[:12]
	}
	if base == "" {
		base = "Player"
	}
	profileName, err := s.uniqueProfileName(ctx, base)
	if err != nil {
		return "", err
	}
	profileID := util.RandomUUIDNoDash()
	if mode == "offline" {
		profileID = util.OfflineUUIDNoDash(profileName)
	}
	if p, err := s.DB.GetProfileByID(ctx, profileID); err != nil {
		return "", err
	} else if p != nil {
		return "", util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
	}
	hash, err := util.HashPassword(password)
	if err != nil {
		return "", err
	}
	userID := util.RandomUUIDNoDash()
	u := model.User{ID: userID, Email: email, Password: hash, IsAdmin: count == 0, DisplayName: username}
	p := model.Profile{ID: profileID, UserID: userID, Name: profileName, TextureModel: "default"}
	if err := s.DB.CreateUserWithProfile(ctx, u, p, inviteCode, email); err != nil {
		if err == database.ErrInviteExhausted {
			return "", util.HTTPError{Status: 400, Detail: "invite code has no remaining uses"}
		}
		return "", err
	}
	return userID, nil
}

func (s Site) SendVerificationCode(ctx context.Context, email, typ string) (map[string]any, error) {
	email = strings.TrimSpace(email)
	if typ == "" {
		typ = "register"
	}
	if enabled, _ := s.DB.GetSetting(ctx, "email_verify_enabled", "false"); enabled != "true" {
		return nil, util.HTTPError{Status: 400, Detail: "Email verification is disabled"}
	}
	if !validEmail(email) {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid email format"}
	}
	switch typ {
	case "register":
		existing, err := s.DB.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, util.HTTPError{Status: 400, Detail: "Email already registered"}
		}
	case "reset":
		existing, err := s.DB.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return map[string]any{"ok": true, "ttl": 0}, nil
		}
	default:
		return nil, util.HTTPError{Status: 400, Detail: "invalid verification type"}
	}
	ttl, _ := s.DB.SettingInt(ctx, "email_verify_ttl", 300)
	code, err := randomVerificationCode(8)
	if err != nil {
		return nil, err
	}
	if err := s.DB.CreateVerificationCode(ctx, email, code, typ, ttl); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "ttl": ttl}, nil
}

func (s Site) VerifyCode(ctx context.Context, email, code, typ string) (bool, error) {
	stored, expiresAt, ok, err := s.DB.GetVerificationCode(ctx, email, typ)
	if err != nil || !ok {
		return false, err
	}
	if database.NowMS() > expiresAt {
		return false, nil
	}
	return strings.EqualFold(stored, code), nil
}

func (s Site) ResetPassword(ctx context.Context, email, newPassword, code string) error {
	if strong, _ := s.DB.GetSetting(ctx, "enable_strong_password_check", "false"); strong == "true" {
		if errs := util.ValidateStrongPassword(newPassword); len(errs) > 0 {
			return util.HTTPError{Status: 400, Detail: util.JoinPasswordErrors(errs)}
		}
	}
	if enabled, _ := s.DB.GetSetting(ctx, "email_verify_enabled", "false"); enabled != "true" {
		return util.HTTPError{Status: 403, Detail: "Password reset via email is disabled"}
	}
	ok, err := s.VerifyCode(ctx, email, code, "reset")
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 400, Detail: "Invalid or expired verification code"}
	}
	user, err := s.DB.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user == nil {
		return util.HTTPError{Status: 404, Detail: "User not found"}
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	updated, err := s.DB.UpdatePasswordAndRevokeRefresh(ctx, user.ID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: 404, Detail: "User not found"}
	}
	return s.DB.DeleteVerificationCode(ctx, email, "reset")
}

func (s Site) uniqueProfileName(ctx context.Context, base string) (string, error) {
	for i := 0; i < 100; i++ {
		name := base
		if i > 0 {
			name = base + "_" + strconvI(i)
		}
		if len(name) > 16 {
			name = name[:16]
		}
		p, err := s.DB.GetProfileByName(ctx, name)
		if err != nil {
			return "", err
		}
		if p == nil {
			return name, nil
		}
	}
	return "", util.HTTPError{Status: 500, Detail: "无法生成唯一角色名"}
}

func (s Site) issueSession(ctx context.Context, userID string, isAdmin bool, extra map[string]any) (map[string]any, error) {
	expireDays, _ := s.DB.SettingInt(ctx, "jwt_expire_days", s.Cfg.JWTExpireDays)
	access, err := util.CreateAccessToken(s.Cfg.JWTSecret, userID, isAdmin, time.Duration(s.Cfg.AccessMinutes)*time.Minute)
	if err != nil {
		return nil, err
	}
	rawRefresh, refreshHash, err := util.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	now := database.NowMS()
	if err := s.DB.AddRefreshToken(ctx, refreshHash, userID, now+int64(expireDays)*24*3600*1000, now); err != nil {
		return nil, err
	}
	out := map[string]any{"access_token": access, "refresh_token": rawRefresh, "is_admin": isAdmin}
	for k, v := range extra {
		out[k] = v
	}
	return out, nil
}

func (s Site) RotateRefresh(ctx context.Context, raw string) (map[string]any, error) {
	row, err := s.DB.ConsumeRefreshToken(ctx, util.HashRefreshToken(raw))
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, util.HTTPError{Status: 401, Detail: "invalid refresh token"}
	}
	if database.NowMS() >= row["expires_at"].(int64) {
		return nil, util.HTTPError{Status: 401, Detail: "refresh token expired"}
	}
	user, err := s.DB.GetUserByID(ctx, row["user_id"].(string))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, util.HTTPError{Status: 401, Detail: "invalid refresh token"}
	}
	return s.issueSession(ctx, user.ID, user.IsAdmin, nil)
}

func (s Site) RevokeRefresh(ctx context.Context, raw string) error {
	return s.DB.DeleteRefreshToken(ctx, util.HashRefreshToken(raw))
}

func (s Site) Me(ctx context.Context, userID string) (map[string]any, error) {
	u, err := s.DB.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, util.HTTPError{Status: 404, Detail: "user not found"}
	}
	pc, _ := s.DB.CountProfilesByUser(ctx, userID)
	tc, _ := s.DB.CountTexturesForUser(ctx, userID)
	return map[string]any{
		"id": u.ID, "email": u.Email, "lang": u.PreferredLanguage, "display_name": u.DisplayName,
		"is_admin": u.IsAdmin, "banned_until": u.BannedUntil, "avatar_hash": u.AvatarHash,
		"profile_count": pc, "texture_count": tc,
	}, nil
}

func (s Site) UpdateMe(ctx context.Context, userID string, body map[string]any) error {
	fields := map[string]any{}
	if v, ok := body["email"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if !validEmail(v) {
			return util.HTTPError{Status: 400, Detail: "Invalid email format"}
		}
		existing, err := s.DB.GetUserByEmail(ctx, v)
		if err != nil {
			return err
		}
		if existing != nil && existing.ID != userID {
			return util.HTTPError{Status: 400, Detail: "Email already in use"}
		}
		fields["email"] = v
	}
	if v, ok := body["display_name"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if v == "" {
			return util.HTTPError{Status: 400, Detail: "Username cannot be empty"}
		}
		if taken, err := s.DB.IsDisplayNameTaken(ctx, v, userID); err != nil {
			return err
		} else if taken {
			return util.HTTPError{Status: 400, Detail: "Username already exists"}
		}
		fields["display_name"] = v
	}
	if v, ok := body["preferred_language"].(string); ok && v != "" {
		fields["preferred_language"] = v
	}
	if v, ok := body["avatar_hash"]; ok {
		fields["avatar_hash"] = v
	}
	return s.DB.UpdateUser(ctx, userID, fields)
}

func (s Site) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	u, err := s.DB.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil {
		return util.HTTPError{Status: 404, Detail: "用户不存在"}
	}
	if !util.VerifyPassword(oldPassword, u.Password) {
		return util.HTTPError{Status: 403, Detail: "旧密码错误"}
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.DB.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.DB.DeleteRefreshTokensByUser(ctx, userID)
}

func (s Site) CreateProfile(ctx context.Context, userID, name, mdl string) (map[string]any, error) {
	if name == "" {
		return nil, util.HTTPError{Status: 400, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return nil, util.HTTPError{Status: 400, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p, err := s.DB.GetProfileByName(ctx, name); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色名已被占用，请换一个名称"}
	}
	id := util.RandomUUIDNoDash()
	mode, _ := s.DB.GetSetting(ctx, "profile_uuid_mode", "random")
	if mode == "offline" {
		id = util.OfflineUUIDNoDash(name)
	}
	if p, err := s.DB.GetProfileByID(ctx, id); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
	}
	mdl = database.NormalizeProfileModel(mdl)
	if err := s.DB.CreateProfile(ctx, model.Profile{ID: id, UserID: userID, Name: name, TextureModel: mdl}); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "model": mdl}, nil
}

func (s Site) PublicLibrary(ctx context.Context, cursor string, limit int, typ, q string) (map[string]any, error) {
	if enabled, _ := s.DB.GetSetting(ctx, "enable_skin_library", "true"); enabled != "true" {
		return nil, util.HTTPError{Status: 403, Detail: "Skin library is disabled by administrator"}
	}
	lastCreated, lastHash, err := textureCursor(cursor, "last_skin_hash")
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.ListPublicLibrary(ctx, limit, typ, strings.TrimSpace(q), lastCreated, lastHash)
}

func (s Site) ListMyProfiles(ctx context.Context, userID, cursor string, limit int) (map[string]any, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	res, err := s.DB.ListProfilesByUser(ctx, userID, limit, last)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Site) ListMyTextures(ctx context.Context, userID, cursor string, limit int, typ string) (map[string]any, error) {
	lastCreated, lastHash, err := textureCursor(cursor, "last_hash")
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.ListUserTextures(ctx, userID, typ, limit, lastCreated, lastHash)
}

func (s Site) AddTextureToWardrobe(ctx context.Context, userID, hash string) error {
	ok, err := s.DB.AddTextureToWardrobe(ctx, userID, hash)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 404, Detail: "Texture not found in library"}
	}
	return nil
}

func (s Site) UpdateProfile(ctx context.Context, userID, profileID, name string) error {
	p, err := s.DB.GetProfileByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	if name == "" {
		return util.HTTPError{Status: 400, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return util.HTTPError{Status: 400, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p.Name != name {
		existing, err := s.DB.GetProfileByName(ctx, name)
		if err != nil {
			return err
		}
		if existing != nil {
			return util.HTTPError{Status: 400, Detail: "角色名已被占用"}
		}
	}
	_, err = s.DB.UpdateProfileName(ctx, profileID, name)
	return err
}

func (s Site) DeleteProfile(ctx context.Context, userID, profileID string) error {
	p, err := s.DB.GetProfileByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	_, err = s.DB.DeleteProfileCascade(ctx, profileID)
	return err
}

func (s Site) ClearProfileTexture(ctx context.Context, userID, profileID, textureType string) error {
	ok, err := s.DB.VerifyProfileOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		return s.DB.UpdateProfileSkin(ctx, profileID, nil)
	case "cape":
		return s.DB.UpdateProfileCape(ctx, profileID, nil)
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
}

func (s Site) ApplyTextureToProfile(ctx context.Context, userID, profileID, hash, textureType string) error {
	owns, err := s.DB.VerifyTextureOwnership(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !owns {
		return util.HTTPError{Status: 403, Detail: "Texture not found in your library"}
	}
	profileOwner, err := s.DB.VerifyProfileOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !profileOwner {
		return util.HTTPError{Status: 403, Detail: "Profile not yours"}
	}
	info, err := s.DB.GetTextureInfo(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if info == nil {
		return util.HTTPError{Status: 403, Detail: "Texture info not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		if err := s.DB.UpdateProfileSkin(ctx, profileID, &hash); err != nil {
			return err
		}
		model, _ := info["model"].(string)
		return s.DB.UpdateProfileModel(ctx, profileID, database.NormalizeProfileModel(model))
	case "cape":
		return s.DB.UpdateProfileCape(ctx, profileID, &hash)
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
}

func (s Site) TextureDetail(ctx context.Context, userID, hash, textureType string) (map[string]any, error) {
	info, err := s.DB.GetTextureInfo(ctx, userID, hash, textureType)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, util.HTTPError{Status: 404, Detail: "Texture not found"}
	}
	return info, nil
}

func (s Site) UpdateTexture(ctx context.Context, userID, hash, textureType string, body map[string]any) (map[string]any, error) {
	if v, ok := body["note"].(string); ok {
		if err := s.DB.UpdateTextureNote(ctx, userID, hash, textureType, v); err != nil {
			return nil, err
		}
	}
	if v, ok := body["model"].(string); ok {
		if err := s.DB.UpdateTextureModel(ctx, userID, hash, textureType, v); err != nil {
			return nil, err
		}
	}
	if v, ok := body["is_public"]; ok {
		pub := false
		switch x := v.(type) {
		case bool:
			pub = x
		case float64:
			pub = x != 0
		case int:
			pub = x != 0
		}
		if err := s.DB.UpdateTexturePublic(ctx, userID, hash, textureType, pub); err != nil {
			return nil, err
		}
	}
	info, err := s.TextureDetail(ctx, userID, hash, textureType)
	if err != nil {
		return nil, err
	}
	info["ok"] = true
	return info, nil
}

func (s Site) DeleteTexture(ctx context.Context, userID, hash, textureType string) error {
	_, err := s.DB.DeleteTextureFromLibrary(ctx, userID, hash, textureType)
	return err
}

func textureCursor(cursor, hashKey string) (*int64, string, error) {
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
	h, _ := m[hashKey].(string)
	return created, h, nil
}

func validEmail(s string) bool {
	if strings.ContainsAny(s, "\r\n") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return strings.Contains(parts[1], ".")
}

func TextureHashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func strconvI(i int) string {
	if i == 0 {
		return "0"
	}
	digits := "0123456789"
	var out []byte
	for i > 0 {
		out = append([]byte{digits[i%10]}, out...)
		i /= 10
	}
	return string(out)
}

func randomVerificationCode(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, length)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}

var ErrUnauthorized = errors.New("unauthorized")

func asCursorMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}
