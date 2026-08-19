package emailpolicy

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

const MaxSuffixesPerList = 256

var (
	siteSettingsReadPermission   = permission.MustDefinitionByCode("site_settings.read.any")
	siteSettingsUpdatePermission = permission.MustDefinitionByCode("site_settings.update.any")
)

type Service struct {
	DB    *database.DB
	Redis redisstore.Store
}

func (s Service) Read(ctx context.Context, actor permission.Actor) (model.EmailSuffixPolicy, error) {
	if err := actor.Require(siteSettingsReadPermission); err != nil {
		return model.EmailSuffixPolicy{}, permissionDenied()
	}
	return s.DB.EmailPolicies.Get(ctx)
}

func (s Service) Update(ctx context.Context, actor permission.Actor, input model.EmailSuffixPolicy) error {
	if err := actor.Require(siteSettingsUpdatePermission); err != nil {
		return permissionDenied()
	}
	policy, err := Normalize(input)
	if err != nil {
		return err
	}
	if err := s.DB.EmailPolicies.Replace(ctx, policy); err != nil {
		return err
	}
	if s.Redis != nil {
		return s.Redis.InvalidatePublicSettings(ctx)
	}
	return nil
}

func (s Service) Public(ctx context.Context) (map[string]any, error) {
	policy, err := s.DB.EmailPolicies.Get(ctx)
	if err != nil {
		return nil, err
	}
	suffixes := policy.Allowlist
	if policy.Mode == model.EmailSuffixModeDenylist {
		suffixes = policy.Denylist
	}
	if policy.Mode == model.EmailSuffixModeDisabled {
		suffixes = []string{}
	}
	return map[string]any{"mode": policy.Mode, "suffixes": suffixes}, nil
}

func (s Service) RequireAllowed(ctx context.Context, email string) error {
	policy, err := s.DB.EmailPolicies.Get(ctx)
	if err != nil {
		return err
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	switch policy.Mode {
	case model.EmailSuffixModeDisabled:
		return nil
	case model.EmailSuffixModeAllowlist:
		if matchesAny(normalizedEmail, policy.Allowlist) {
			return nil
		}
	case model.EmailSuffixModeDenylist:
		if !matchesAny(normalizedEmail, policy.Denylist) {
			return nil
		}
	}
	return util.HTTPError{Status: http.StatusBadRequest, Object: "email", Operation: "validate", Reason: "denied"}
}

func Normalize(input model.EmailSuffixPolicy) (model.EmailSuffixPolicy, error) {
	if input.Mode != model.EmailSuffixModeDisabled && input.Mode != model.EmailSuffixModeAllowlist && input.Mode != model.EmailSuffixModeDenylist {
		return model.EmailSuffixPolicy{}, util.HTTPError{Status: http.StatusBadRequest, Object: "email_policy", Operation: "configure", Reason: "invalid"}
	}
	allowlist, err := normalizeList("allowlist", input.Allowlist)
	if err != nil {
		return model.EmailSuffixPolicy{}, err
	}
	denylist, err := normalizeList("denylist", input.Denylist)
	if err != nil {
		return model.EmailSuffixPolicy{}, err
	}
	if input.Mode == model.EmailSuffixModeAllowlist && len(allowlist) == 0 {
		return model.EmailSuffixPolicy{}, util.HTTPError{Status: http.StatusBadRequest, Object: "email_allowlist", Operation: "configure", Reason: "required"}
	}
	return model.EmailSuffixPolicy{Mode: input.Mode, Allowlist: allowlist, Denylist: denylist}, nil
}

func normalizeList(name string, values []string) ([]string, error) {
	if len(values) > MaxSuffixesPerList {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "email_suffix", Operation: "configure", Reason: "exceeded", Params: map[string]any{"list": name, "max": MaxSuffixesPerList}}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		suffix := strings.ToLower(strings.TrimSpace(value))
		if !strings.HasPrefix(suffix, "@") {
			suffix = "@" + suffix
		}
		if strings.Count(suffix, "@") != 1 || !util.ValidEmail("policy"+suffix) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "email_suffix", Operation: "validate", Reason: "invalid", Params: map[string]any{"suffix": value}}
		}
		if _, ok := seen[suffix]; ok {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "email_suffix", Operation: "configure", Reason: "conflict", Params: map[string]any{"suffix": suffix, "list": name}}
		}
		seen[suffix] = struct{}{}
		out = append(out, suffix)
	}
	sort.Strings(out)
	return out, nil
}

func matchesAny(email string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(email, suffix) {
			return true
		}
	}
	return false
}

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}
