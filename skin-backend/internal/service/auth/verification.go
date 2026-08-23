package auth

import (
	"context"

	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	verificationsvc "element-skin/backend/internal/service/verification"
	"element-skin/backend/internal/util"
)

func (s Service) SendVerificationCode(ctx context.Context, email, typ string) (map[string]any, error) {
	return s.verification().SendPublic(ctx, email, typ)
}

func (s Service) VerifyCode(ctx context.Context, email, code, typ string) (bool, error) {
	return s.verification().Verify(ctx, email, code, typ)
}

func (s Service) ResetPassword(ctx context.Context, email, newPassword, code string) error {
	settings := s.settings()
	strong, err := settings.Get(ctx, "enable_strong_password_check", "false")
	if err != nil {
		return err
	}
	if strong == "true" {
		if errs := util.ValidateStrongPassword(newPassword); len(errs) > 0 {
			return util.HTTPError{Status: 400, Object: "password", Operation: "validate", Reason: "invalid", Params: map[string]any{"rules": errs}}
		}
	}
	enabled, err := settings.Get(ctx, "email_verify_enabled", "false")
	if err != nil {
		return err
	}
	if enabled != "true" {
		return util.HTTPError{Status: 403, Object: "password_reset", Operation: "create", Reason: "disabled"}
	}
	ok, err := s.VerifyCode(ctx, email, code, "reset")
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 400, Object: "verification_code", Operation: "verify", Reason: "invalid"}
	}
	user, err := s.DB.Users.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user == nil {
		return util.HTTPError{Status: 404, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	verification := s.verification()
	consumed, err := verification.Consume(ctx, email, code, verificationsvc.PurposeReset)
	if err != nil {
		return err
	}
	if !consumed {
		return util.HTTPError{Status: 400, Object: "verification_code", Operation: "verify", Reason: "invalid"}
	}
	restoreCode := func() {
		_ = verification.Restore(ctx, email, code, verificationsvc.PurposeReset)
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, user.ID); err != nil {
		restoreCode()
		return err
	}
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, user.ID, hash)
	if err != nil {
		restoreCode()
		return err
	}
	if !updated {
		restoreCode()
		return util.HTTPError{Status: 404, Object: "user", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}

func (s Service) verification() verificationsvc.Service {
	verification := s.Verification
	if verification.DB == nil {
		verification.DB = s.DB
	}
	if verification.Redis == nil {
		verification.Redis = s.Redis
	}
	if verification.Settings.DB == nil {
		verification.Settings = s.Settings
	}
	return verification
}

func (s Service) emailPolicy() emailpolicysvc.Service {
	policy := s.EmailPolicy
	if policy.DB == nil {
		policy.DB = s.DB
	}
	if policy.Redis == nil {
		policy.Redis = s.Redis
	}
	return policy
}
