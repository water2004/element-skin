package site

import (
	"context"
	"errors"
	"strings"
	"time"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (s Site) SendVerificationCode(ctx context.Context, email, typ string) (map[string]any, error) {
	email = strings.TrimSpace(email)
	if typ == "" {
		typ = "register"
	}
	settings := s.settings()
	enabled, err := settings.Get(ctx, "email_verify_enabled", "false")
	if err != nil {
		return nil, err
	}
	if enabled != "true" {
		return nil, util.HTTPError{Status: 400, Detail: "Email verification is disabled"}
	}
	if !validEmail(email) {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid email format"}
	}
	switch typ {
	case "register":
		existing, err := s.DB.Users.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, util.HTTPError{Status: 400, Detail: "Email already registered"}
		}
	case "reset":
		existing, err := s.DB.Users.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return map[string]any{"ok": true, "ttl": 0}, nil
		}
	default:
		return nil, util.HTTPError{Status: 400, Detail: "invalid verification type"}
	}
	ttl, err := settings.Int(ctx, "email_verify_ttl", 300)
	if err != nil {
		return nil, err
	}
	code, err := randomVerificationCode(8)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.SetVerificationCode(ctx, email, typ, code, time.Duration(ttl)*time.Second); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "ttl": ttl}, nil
}

func (s Site) VerifyCode(ctx context.Context, email, code, typ string) (bool, error) {
	stored, err := s.Redis.GetVerificationCode(ctx, email, typ)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(stored, code), nil
}

func (s Site) ResetPassword(ctx context.Context, email, newPassword, code string) error {
	settings := s.settings()
	strong, err := settings.Get(ctx, "enable_strong_password_check", "false")
	if err != nil {
		return err
	}
	if strong == "true" {
		if errs := util.ValidateStrongPassword(newPassword); len(errs) > 0 {
			return util.HTTPError{Status: 400, Detail: util.JoinPasswordErrors(errs)}
		}
	}
	enabled, err := settings.Get(ctx, "email_verify_enabled", "false")
	if err != nil {
		return err
	}
	if enabled != "true" {
		return util.HTTPError{Status: 403, Detail: "Password reset via email is disabled"}
	}
	ok, err := s.VerifyCode(ctx, email, code, "reset")
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 400, Detail: "Invalid or expired verification code"}
	}
	user, err := s.DB.Users.GetByEmail(ctx, email)
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
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, user.ID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: 404, Detail: "User not found"}
	}
	return s.Redis.DeleteVerificationCode(ctx, email, "reset")
}
