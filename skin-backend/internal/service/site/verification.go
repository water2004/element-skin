package site

import (
	"context"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

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
