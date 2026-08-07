package identity

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	AdapterGenericOIDC = "generic_oidc"
	AdapterMicrosoft   = "microsoft"
)

type Service struct {
	DB             *database.DB
	Config         config.Config
	Discovery      Discovery
	Redis          redisstore.Store
	OIDCClient     OIDCClient
	TokenRefresher TokenRefresher
}

type ProviderInput struct {
	Name                string
	IssuerURL           string
	ClientID            string
	ClientSecret        *string
	Scopes              []string
	Adapter             string
	IconURL             string
	Enabled             bool
	LoginEnabled        bool
	LinkEnabled         bool
	RegistrationEnabled bool
	DisplayOrder        int
}

func (s Service) ListPublicProviders(ctx context.Context, actor permission.Actor) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("identity_provider.read.public")); err != nil {
		return nil, forbidden()
	}
	items, err := s.DB.Identities.ListProviders(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, publicProviderResponse(item))
	}
	return out, nil
}

func (s Service) ListProviders(ctx context.Context, actor permission.Actor) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("identity_provider.read.any")); err != nil {
		return nil, forbidden()
	}
	items, err := s.DB.Identities.ListProviders(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, adminProviderResponse(item))
	}
	return out, nil
}

func (s Service) GetProvider(ctx context.Context, actor permission.Actor, id string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("identity_provider.read.any")); err != nil {
		return nil, forbidden()
	}
	item, err := s.DB.Identities.GetProvider(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, notFound("identity provider not found")
	}
	return adminProviderResponse(*item), nil
}

func (s Service) CreateProvider(ctx context.Context, actor permission.Actor, input ProviderInput) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("identity_provider.create.any")); err != nil {
		return nil, forbidden()
	}
	item, err := s.providerFromInput(ctx, input, nil)
	if err != nil {
		return nil, err
	}
	item.ID, err = util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	item.CreatedAt = database.NowMS()
	item.UpdatedAt = item.CreatedAt
	if err := s.DB.Identities.CreateProvider(ctx, item); err != nil {
		if isUniqueViolation(err) {
			return nil, conflict("an identity provider with this issuer and client_id already exists")
		}
		return nil, err
	}
	return adminProviderResponse(item), nil
}

func (s Service) UpdateProvider(ctx context.Context, actor permission.Actor, id string, input ProviderInput) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("identity_provider.update.any")); err != nil {
		return nil, forbidden()
	}
	current, err := s.DB.Identities.GetProvider(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, notFound("identity provider not found")
	}
	item, err := s.providerFromInput(ctx, input, current)
	if err != nil {
		return nil, err
	}
	item.ID = current.ID
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = database.NowMS()
	updated, err := s.DB.Identities.UpdateProvider(ctx, item)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, conflict("an identity provider with this issuer and client_id already exists")
		}
		return nil, err
	}
	if !updated {
		return nil, notFound("identity provider not found")
	}
	return adminProviderResponse(item), nil
}

func (s Service) DeleteProvider(ctx context.Context, actor permission.Actor, id string) error {
	if err := actor.Require(permission.MustDefinitionByCode("identity_provider.delete.any")); err != nil {
		return forbidden()
	}
	deleted, err := s.DB.Identities.DeleteProvider(ctx, strings.TrimSpace(id))
	if err != nil {
		if isForeignKeyViolation(err) {
			return conflict("identity provider is still referenced by external identities")
		}
		return err
	}
	if !deleted {
		return notFound("identity provider not found")
	}
	return nil
}

func (s Service) ListIdentities(ctx context.Context, actor permission.Actor) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("external_identity.read.owned")); err != nil {
		return nil, forbidden()
	}
	items, err := s.DB.Identities.ListIdentitiesByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		provider, err := s.DB.Identities.GetProvider(ctx, item.ProviderID)
		if err != nil {
			return nil, err
		}
		if provider == nil {
			return nil, errors.New("external identity references a missing provider")
		}
		credential, err := s.DB.Identities.GetCredential(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if credential == nil {
			return nil, errors.New("external identity references a missing credential")
		}
		out = append(out, identityResponse(item, *provider, *credential))
	}
	return out, nil
}

func (s Service) UpdateIdentityLabel(ctx context.Context, actor permission.Actor, id, label string) error {
	if err := actor.Require(permission.MustDefinitionByCode("external_identity.update.owned")); err != nil {
		return forbidden()
	}
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 80 {
		return badRequest("identity label must not exceed 80 characters")
	}
	updated, err := s.DB.Identities.UpdateIdentityLabel(ctx, strings.TrimSpace(id), actor.UserID, label, database.NowMS())
	if err != nil {
		return err
	}
	if !updated {
		return notFound("external identity not found")
	}
	return nil
}

func (s Service) DeleteIdentity(ctx context.Context, actor permission.Actor, id string) error {
	if err := actor.Require(permission.MustDefinitionByCode("external_identity.delete.owned")); err != nil {
		return forbidden()
	}
	deleted, err := s.DB.Identities.DeleteIdentity(ctx, strings.TrimSpace(id), actor.UserID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return conflict("external identity is still used by an official profile binding")
		}
		return err
	}
	if !deleted {
		return notFound("external identity not found")
	}
	return nil
}

func (s Service) providerFromInput(ctx context.Context, input ProviderInput, current *model.IdentityProvider) (model.IdentityProvider, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.IssuerURL = strings.TrimSpace(input.IssuerURL)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.IconURL = strings.TrimSpace(input.IconURL)
	input.Adapter = strings.TrimSpace(input.Adapter)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return model.IdentityProvider{}, badRequest("provider name is required and must not exceed 80 characters")
	}
	if input.ClientID == "" || len(input.ClientID) > 512 {
		return model.IdentityProvider{}, badRequest("client_id is required and must not exceed 512 characters")
	}
	if input.Adapter == "" {
		input.Adapter = AdapterGenericOIDC
	}
	if input.Adapter != AdapterGenericOIDC && input.Adapter != AdapterMicrosoft {
		return model.IdentityProvider{}, badRequest("invalid provider adapter")
	}
	if err := validateEndpointURL(input.IssuerURL); err != nil {
		return model.IdentityProvider{}, badRequest("invalid issuer_url")
	}
	if input.IconURL != "" {
		if err := validateEndpointURL(input.IconURL); err != nil {
			return model.IdentityProvider{}, badRequest("invalid icon_url")
		}
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return model.IdentityProvider{}, err
	}
	if input.Adapter == AdapterMicrosoft && (!hasScope(scopes, "XboxLive.signin") || !hasScope(scopes, "offline_access")) {
		return model.IdentityProvider{}, badRequest("Microsoft providers must request XboxLive.signin and offline_access scopes")
	}
	discovery := s.Discovery
	if discovery == nil {
		discovery = HTTPDiscovery{}
	}
	metadata, err := discovery.Discover(ctx, input.IssuerURL)
	if err != nil {
		return model.IdentityProvider{}, badRequest(err.Error())
	}
	if metadata.Issuer != input.IssuerURL {
		return model.IdentityProvider{}, badRequest("OIDC discovery issuer does not exactly match issuer_url")
	}
	for _, endpoint := range []string{metadata.AuthorizationEndpoint, metadata.TokenEndpoint, metadata.JWKSURI} {
		if err := validateEndpointURL(endpoint); err != nil {
			return model.IdentityProvider{}, badRequest("OIDC discovery document contains an invalid required endpoint")
		}
	}
	if metadata.UserInfoEndpoint != "" {
		if err := validateEndpointURL(metadata.UserInfoEndpoint); err != nil {
			return model.IdentityProvider{}, badRequest("OIDC discovery document contains an invalid userinfo endpoint")
		}
	}
	secretCiphertext := ""
	if current != nil {
		secretCiphertext = current.ClientSecretCiphertext
	}
	if input.ClientSecret != nil {
		box, err := util.NewSecretBox(s.Config.IdentityEncryptionKey)
		if err != nil {
			return model.IdentityProvider{}, err
		}
		secretCiphertext, err = box.Encrypt(strings.TrimSpace(*input.ClientSecret))
		if err != nil {
			return model.IdentityProvider{}, err
		}
	}
	return model.IdentityProvider{
		Name:                   input.Name,
		IssuerURL:              input.IssuerURL,
		AuthorizationEndpoint:  metadata.AuthorizationEndpoint,
		TokenEndpoint:          metadata.TokenEndpoint,
		UserInfoEndpoint:       metadata.UserInfoEndpoint,
		JWKSURI:                metadata.JWKSURI,
		ClientID:               input.ClientID,
		ClientSecretCiphertext: secretCiphertext,
		Scopes:                 scopes,
		Adapter:                input.Adapter,
		IconURL:                input.IconURL,
		Enabled:                input.Enabled,
		LoginEnabled:           input.LoginEnabled,
		LinkEnabled:            input.LinkEnabled,
		RegistrationEnabled:    input.RegistrationEnabled,
		DisplayOrder:           input.DisplayOrder,
	}, nil
}

func hasScope(scopes []string, expected string) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func normalizeScopes(scopes []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(scopes)+1)
	for _, scope := range scopes {
		for _, item := range strings.Fields(scope) {
			if len(item) > 128 || strings.ContainsAny(item, `"\\`) {
				return nil, badRequest("invalid OIDC scope")
			}
			if !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
	}
	if !seen["openid"] {
		out = append(out, "openid")
	}
	if len(out) > 32 {
		return nil, badRequest("too many OIDC scopes")
	}
	sort.Strings(out)
	return out, nil
}

func validateEndpointURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("invalid URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	host := u.Hostname()
	if u.Scheme == "http" && (host == "localhost" || net.ParseIP(host).IsLoopback()) {
		return nil
	}
	return errors.New("URL must use HTTPS except on loopback")
}

func publicProviderResponse(item model.IdentityProvider) map[string]any {
	return map[string]any{
		"id":                   item.ID,
		"name":                 item.Name,
		"adapter":              item.Adapter,
		"icon_url":             item.IconURL,
		"login_enabled":        item.LoginEnabled,
		"link_enabled":         item.LinkEnabled,
		"registration_enabled": item.RegistrationEnabled,
	}
}

func adminProviderResponse(item model.IdentityProvider) map[string]any {
	response := publicProviderResponse(item)
	response["issuer_url"] = item.IssuerURL
	response["authorization_endpoint"] = item.AuthorizationEndpoint
	response["token_endpoint"] = item.TokenEndpoint
	response["userinfo_endpoint"] = item.UserInfoEndpoint
	response["jwks_uri"] = item.JWKSURI
	response["client_id"] = item.ClientID
	response["has_client_secret"] = item.ClientSecretCiphertext != ""
	response["scopes"] = item.Scopes
	response["enabled"] = item.Enabled
	response["display_order"] = item.DisplayOrder
	response["created_at"] = item.CreatedAt
	response["updated_at"] = item.UpdatedAt
	return response
}

func identityResponse(item model.ExternalIdentity, provider model.IdentityProvider, credential model.ExternalIdentityCredential) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"provider_id":           item.ProviderID,
		"provider_name":         provider.Name,
		"provider_adapter":      provider.Adapter,
		"provider_icon_url":     provider.IconURL,
		"provider_enabled":      provider.Enabled,
		"provider_link_enabled": provider.LinkEnabled,
		"subject":               item.Subject,
		"label":                 item.Label,
		"email":                 item.Email,
		"email_verified":        item.EmailVerified,
		"display_name":          item.DisplayName,
		"avatar_url":            item.AvatarURL,
		"created_at":            item.CreatedAt,
		"updated_at":            item.UpdatedAt,
		"last_login_at":         item.LastLoginAt,
		"authorization_status":  credential.AuthorizationStatus,
		"last_refresh_at":       credential.LastRefreshAt,
		"last_refresh_error_at": credential.LastRefreshErrorAt,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001")
}

func badRequest(detail string) util.HTTPError { return util.HTTPError{Status: 400, Detail: detail} }
func forbidden() util.HTTPError               { return util.HTTPError{Status: 403, Detail: "permission denied"} }
func notFound(detail string) util.HTTPError   { return util.HTTPError{Status: 404, Detail: detail} }
func conflict(detail string) util.HTTPError   { return util.HTTPError{Status: 409, Detail: detail} }
