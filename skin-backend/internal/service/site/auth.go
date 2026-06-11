package site

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	invitestore "element-skin/backend/internal/database/invite"
	profilestore "element-skin/backend/internal/database/profile"
	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

// Site contains user-facing account, profile, and texture operations.
type Site struct {
	DB       *database.DB
	Cfg      config.Config
	Redis    redisstore.Store
	Settings settingssvc.Settings
}

func (s Site) settings() settingssvc.Settings {
	return s.Settings
}

func (s Site) Login(ctx context.Context, email, password string) (map[string]any, error) {
	user, err := s.DB.Users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil || !util.VerifyPassword(password, user.Password) {
		return nil, util.HTTPError{Status: 401, Detail: "Invalid credentials"}
	}
	return s.issueSession(ctx, user.ID, user.IsAdmin, user.IsSuperAdmin, map[string]any{"user_id": user.ID})
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
	if taken, err := s.DB.Users.IsDisplayNameTaken(ctx, username, ""); err != nil {
		return "", err
	} else if taken {
		return "", util.HTTPError{Status: 400, Detail: "Username already exists"}
	}
	if existing, err := s.DB.Users.GetByEmail(ctx, email); err != nil {
		return "", err
	} else if existing != nil {
		return "", util.HTTPError{Status: 400, Detail: "Email already registered"}
	}
	settings := s.settings()
	strong, err := settings.Get(ctx, "enable_strong_password_check", "false")
	if err != nil {
		return "", err
	}
	if strong == "true" {
		if errs := util.ValidateStrongPassword(password); len(errs) > 0 {
			return "", util.HTTPError{Status: 400, Detail: util.JoinPasswordErrors(errs)}
		}
	}
	allow, err := settings.Get(ctx, "allow_register", "true")
	if err != nil {
		return "", err
	}
	if allow != "true" {
		return "", util.HTTPError{Status: 403, Detail: "registration is disabled"}
	}
	enabled, err := settings.Get(ctx, "email_verify_enabled", "false")
	if err != nil {
		return "", err
	}
	verifiedEmail := false
	if enabled == "true" {
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
		verifiedEmail = true
	}
	requireInvite, err := settings.Get(ctx, "require_invite", "false")
	if err != nil {
		return "", err
	}
	inviteCode := ""
	if requireInvite == "true" {
		if invite == "" {
			return "", util.HTTPError{Status: 400, Detail: "invite code required"}
		}
		inv, err := s.DB.Invites.Get(ctx, invite)
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
	count, err := s.DB.Users.Count(ctx)
	if err != nil {
		return "", err
	}
	mode, err := settings.Get(ctx, "profile_uuid_mode", "random")
	if err != nil {
		return "", err
	}
	base := regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(strings.Split(email, "@")[0], "_")
	if len(base) > 12 {
		base = base[:12]
	}
	if base == "" {
		base = "Player"
	}
	hash, err := util.HashPassword(password)
	if err != nil {
		return "", err
	}
	userID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return "", err
	}
	firstUser := count == 0
	u := model.User{ID: userID, Email: email, Password: hash, IsAdmin: firstUser, IsSuperAdmin: firstUser, DisplayName: username}
	for attempt := 0; attempt < 100; attempt++ {
		profileName := util.ProfileNameCandidate(base, attempt)
		if existing, err := s.DB.Profiles.GetByName(ctx, profileName); err != nil {
			return "", err
		} else if existing != nil {
			continue
		}
		profileID, err := util.GenerateUUIDNoDash()
		if err != nil {
			return "", err
		}
		if mode == "offline" {
			profileID = util.OfflineUUIDNoDash(profileName)
		}
		if existing, err := s.DB.Profiles.GetByID(ctx, profileID); err != nil {
			return "", err
		} else if existing != nil {
			return "", util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
		}
		p := model.Profile{ID: profileID, UserID: userID, Name: profileName, TextureModel: "default"}
		err = s.DB.Users.CreateWithProfile(ctx, u, p, inviteCode, email)
		if profilestore.IsNameConflict(err) || (mode == "offline" && profilestore.IsIDConflict(err)) {
			continue
		}
		if err == nil {
			if verifiedEmail {
				_ = s.Redis.DeleteVerificationCode(ctx, email, "register")
			}
			return userID, nil
		}
		if err == invitestore.ErrExhausted {
			return "", util.HTTPError{Status: 400, Detail: "invite code has no remaining uses"}
		}
		if errors.Is(err, userstore.ErrDisplayNameConflict) {
			return "", util.HTTPError{Status: 400, Detail: "Username already exists"}
		}
		if userstore.IsEmailConflict(err) {
			return "", util.HTTPError{Status: 400, Detail: "Email already registered"}
		}
		if profilestore.IsIDConflict(err) {
			return "", util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
		}
		return "", err
	}
	return "", util.HTTPError{Status: 500, Detail: "无法生成唯一角色名"}
}
