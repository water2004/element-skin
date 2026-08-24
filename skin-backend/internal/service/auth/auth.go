package auth

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
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	settingssvc "element-skin/backend/internal/service/settings"
	verificationsvc "element-skin/backend/internal/service/verification"
	"element-skin/backend/internal/util"
)

type Service struct {
	DB           *database.DB
	Cfg          config.Config
	Redis        redisstore.Store
	Settings     settingssvc.Settings
	Verification verificationsvc.Service
	EmailPolicy  emailpolicysvc.Service
}

func (s Service) settings() settingssvc.Settings {
	return s.Settings
}

func (s Service) Login(ctx context.Context, email, password string) (map[string]any, error) {
	user, err := s.DB.Users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil || !util.VerifyPassword(password, user.Password) {
		return nil, util.HTTPError{Status: 401, Object: "credentials", Operation: "verify", Reason: "invalid"}
	}
	return s.issueSession(ctx, user.ID, map[string]any{"user_id": user.ID})
}

func (s Service) IssueSessionForUser(ctx context.Context, userID string) (map[string]any, error) {
	user, err := s.DB.Users.GetByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, util.HTTPError{Status: 404, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	return s.issueSession(ctx, user.ID, map[string]any{"user_id": user.ID})
}

func (s Service) Register(ctx context.Context, email, password, username, invite, code string) (string, error) {
	email = strings.TrimSpace(email)
	username = strings.TrimSpace(username)
	if username == "" {
		return "", util.HTTPError{Status: 400, Object: "username", Operation: "validate", Reason: "required"}
	}
	if !validEmail(email) {
		return "", util.HTTPError{Status: 400, Object: "email", Operation: "validate", Reason: "invalid"}
	}
	if err := s.emailPolicy().RequireAllowed(ctx, email); err != nil {
		return "", err
	}
	if password == "" {
		return "", util.HTTPError{Status: 400, Object: "password", Operation: "validate", Reason: "required"}
	}
	if taken, err := s.DB.Users.IsDisplayNameTaken(ctx, username, ""); err != nil {
		return "", err
	} else if taken {
		return "", util.HTTPError{Status: 400, Object: "username", Operation: "reserve", Reason: "conflict"}
	}
	if existing, err := s.DB.Users.GetByEmail(ctx, email); err != nil {
		return "", err
	} else if existing != nil {
		return "", util.HTTPError{Status: 400, Object: "email", Operation: "register", Reason: "already_exists"}
	}
	settings := s.settings()
	strong, err := settings.Get(ctx, "enable_strong_password_check", "false")
	if err != nil {
		return "", err
	}
	if strong == "true" {
		if errs := util.ValidateStrongPassword(password); len(errs) > 0 {
			return "", util.HTTPError{Status: 400, Object: "password", Operation: "validate", Reason: "invalid", Params: map[string]any{"rules": errs}}
		}
	}
	allow, err := settings.Get(ctx, "allow_register", "true")
	if err != nil {
		return "", err
	}
	if allow != "true" {
		return "", util.HTTPError{Status: 403, Object: "registration", Operation: "create", Reason: "disabled"}
	}
	enabled, err := settings.Get(ctx, "email_verify_enabled", "false")
	if err != nil {
		return "", err
	}
	verifiedEmail := false
	if enabled == "true" {
		if code == "" {
			return "", util.HTTPError{Status: 400, Object: "verification_code", Operation: "validate", Reason: "required"}
		}
		ok, err := s.VerifyCode(ctx, email, code, "register")
		if err != nil {
			return "", err
		}
		if !ok {
			return "", util.HTTPError{Status: 400, Object: "verification_code", Operation: "verify", Reason: "invalid"}
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
			return "", util.HTTPError{Status: 400, Object: "invite", Operation: "validate", Reason: "required"}
		}
		inv, err := s.DB.Invites.Get(ctx, invite)
		if err != nil {
			return "", err
		}
		if inv == nil {
			return "", util.HTTPError{Status: 400, Object: "invite", Operation: "consume", Reason: "invalid"}
		}
		if inv.TotalUses != nil && inv.UsedCount >= *inv.TotalUses {
			return "", util.HTTPError{Status: 400, Object: "invite", Operation: "consume", Reason: "exhausted"}
		}
		inviteCode = invite
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
	u := model.User{ID: userID, Email: email, Password: hash, DisplayName: username}
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
			return "", util.HTTPError{Status: 400, Object: "profile_uuid", Operation: "reserve", Reason: "conflict"}
		}
		p := model.Profile{ID: profileID, UserID: userID, Name: profileName, TextureModel: "default"}
		err = s.DB.Users.CreateWithProfile(ctx, u, p, inviteCode, email)
		if profilestore.IsNameConflict(err) || (mode == "offline" && profilestore.IsIDConflict(err)) {
			continue
		}
		if err == nil {
			if err := s.DB.Permissions.EnsureUserSubject(ctx, userID); err != nil {
				return "", err
			}
			if _, err := s.DB.Permissions.GrantInitialProtectedManagerIfNone(ctx, userID); err != nil {
				return "", err
			}
			if verifiedEmail {
				_ = s.Redis.DeleteVerificationCode(ctx, email, "register")
			}
			return userID, nil
		}
		if err == invitestore.ErrExhausted {
			return "", util.HTTPError{Status: 400, Object: "invite", Operation: "consume", Reason: "exhausted"}
		}
		if errors.Is(err, userstore.ErrDisplayNameConflict) {
			return "", util.HTTPError{Status: 400, Object: "username", Operation: "reserve", Reason: "conflict"}
		}
		if userstore.IsEmailConflict(err) {
			return "", util.HTTPError{Status: 400, Object: "email", Operation: "register", Reason: "already_exists"}
		}
		if profilestore.IsIDConflict(err) {
			return "", util.HTTPError{Status: 400, Object: "profile_uuid", Operation: "reserve", Reason: "conflict"}
		}
		return "", err
	}
	return "", util.HTTPError{Status: 500, Object: "profile_name", Operation: "allocate", Reason: "failed"}
}
