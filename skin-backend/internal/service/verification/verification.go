package verification

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	mailsvc "element-skin/backend/internal/service/mail"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

const (
	PurposeRegister    = "register"
	PurposeReset       = "reset"
	PurposeEmailChange = "email_change"
)

type Service struct {
	DB          *database.DB
	Redis       redisstore.Store
	Settings    settingssvc.Settings
	Sender      mailsvc.Sender
	EmailPolicy emailpolicysvc.Service
}

func (s Service) SendPublic(ctx context.Context, email, purpose string) (map[string]any, error) {
	email = strings.TrimSpace(email)
	if purpose == "" {
		purpose = PurposeRegister
	}
	if err := s.ensureEnabled(ctx); err != nil {
		return nil, err
	}
	if !util.ValidEmail(email) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid email format"}
	}
	existing, err := s.DB.Users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	switch purpose {
	case PurposeRegister:
		policy := s.EmailPolicy
		if policy.DB == nil {
			policy.DB = s.DB
		}
		if err := policy.RequireAllowed(ctx, email); err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Email already registered"}
		}
	case PurposeReset:
		if existing == nil {
			return map[string]any{"ttl": 0}, nil
		}
	default:
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid verification type"}
	}
	return s.issue(ctx, email, purpose)
}

func (s Service) SendEmailChange(ctx context.Context, email string) (map[string]any, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return nil, err
	}
	return s.issue(ctx, strings.TrimSpace(email), PurposeEmailChange)
}

func (s Service) Verify(ctx context.Context, email, code, purpose string) (bool, error) {
	stored, err := s.Redis.GetVerificationCode(ctx, email, purpose)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(stored, strings.TrimSpace(code)), nil
}

func (s Service) Consume(ctx context.Context, email, code, purpose string) (bool, error) {
	return s.Redis.ConsumeVerificationCode(ctx, email, purpose, strings.TrimSpace(code))
}

func (s Service) Restore(ctx context.Context, email, code, purpose string) error {
	ttl, err := s.TTL(ctx)
	if err != nil {
		return err
	}
	_, err = s.Redis.SetVerificationCodeIfAbsent(ctx, email, purpose, code, time.Duration(ttl)*time.Second)
	return err
}

func (s Service) TTL(ctx context.Context) (int, error) {
	return s.Settings.Int(ctx, "email_verify_ttl", 300)
}

func (s Service) issue(ctx context.Context, email, purpose string) (map[string]any, error) {
	if s.Sender == nil {
		return nil, errors.New("verification email sender is not configured")
	}
	ttl, err := s.TTL(ctx)
	if err != nil {
		return nil, err
	}
	code, err := randomCode(8)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.SetVerificationCode(ctx, email, purpose, code, time.Duration(ttl)*time.Second); err != nil {
		return nil, err
	}
	if err := s.Sender.SendVerificationCode(ctx, email, code, purpose); err != nil {
		_ = s.Redis.DeleteVerificationCode(ctx, email, purpose)
		return nil, err
	}
	return map[string]any{"ttl": ttl}, nil
}

func (s Service) ensureEnabled(ctx context.Context) error {
	enabled, err := s.Settings.Get(ctx, "email_verify_enabled", "false")
	if err != nil {
		return err
	}
	if enabled != "true" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "Email verification is disabled"}
	}
	return nil
}
