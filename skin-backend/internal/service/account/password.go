package account

import (
	"context"
	"net/http"

	"element-skin/backend/internal/util"
)

func (s AccountService) validatePassword(ctx context.Context, password string) error {
	settings := s.Settings
	if settings.DB == nil {
		settings.DB = s.DB
	}
	if settings.Redis == nil {
		settings.Redis = s.Redis
	}
	enabled, err := settings.Get(ctx, "enable_strong_password_check", "false")
	if err != nil {
		return err
	}
	if rules := util.ValidatePassword(password, enabled == "true"); len(rules) > 0 {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "password", Operation: "validate", Reason: "invalid", Params: map[string]any{"rules": rules}}
	}
	return nil
}
