package identity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

const (
	AuthorizationIntentLogin = "login"
	AuthorizationIntentLink  = "link"

	authorizationStateKind = "oidc_authorization"
	registrationStateKind  = "oidc_registration"
	authorizationStateTTL  = 10 * time.Minute
	registrationStateTTL   = 15 * time.Minute
)

type AuthorizationStart struct {
	AuthorizationURL string `json:"authorization_url"`
	ExpiresIn        int    `json:"expires_in"`
}

type AuthorizationResult struct {
	Intent             string
	UserID             string
	IdentityID         string
	RegistrationTicket string
	ProviderID         string
	ReturnTo           string
}

func (s Service) StartAuthorization(ctx context.Context, actor permission.Actor, providerID, intent, identityID, returnTo string) (AuthorizationStart, error) {
	provider, err := s.DB.Identities.GetProvider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return AuthorizationStart{}, err
	}
	if provider == nil || !provider.Enabled {
		return AuthorizationStart{}, notFound("identity_provider", "resolve", "not_found")
	}
	intent = strings.TrimSpace(intent)
	identityID = strings.TrimSpace(identityID)
	var targetIdentity *model.ExternalIdentity
	switch intent {
	case AuthorizationIntentLogin:
		if identityID != "" {
			return AuthorizationStart{}, badRequest("identity_id", "validate", "invalid")
		}
		if !provider.LoginEnabled {
			return AuthorizationStart{}, forbiddenCode("identity_provider", "login", "disabled")
		}
		returnTo, err = normalizedReturnTo(returnTo)
		if err != nil {
			return AuthorizationStart{}, err
		}
	case AuthorizationIntentLink:
		if err := actor.Require(permission.MustDefinitionByCode("external_identity.create.owned")); err != nil || actor.UserID == "" {
			return AuthorizationStart{}, forbidden()
		}
		if !provider.LinkEnabled {
			return AuthorizationStart{}, forbiddenCode("identity_provider", "link", "disabled")
		}
		if identityID != "" {
			targetIdentity, err = s.DB.Identities.GetIdentity(ctx, identityID)
			if err != nil {
				return AuthorizationStart{}, err
			}
			if targetIdentity == nil || targetIdentity.UserID != actor.UserID || targetIdentity.ProviderID != provider.ID {
				return AuthorizationStart{}, notFound("identity", "resolve", "not_found")
			}
		}
	default:
		return AuthorizationStart{}, badRequest("authorization_intent", "validate", "invalid")
	}
	state, err := opaqueToken()
	if err != nil {
		return AuthorizationStart{}, err
	}
	nonce, err := opaqueToken()
	if err != nil {
		return AuthorizationStart{}, err
	}
	pkceVerifier, err := opaqueToken()
	if err != nil {
		return AuthorizationStart{}, err
	}
	if s.Redis == nil {
		return AuthorizationStart{}, errors.New("identity state store is not configured")
	}
	storedState := map[string]any{
		"kind":          authorizationStateKind,
		"provider_id":   provider.ID,
		"intent":        intent,
		"user_id":       actor.UserID,
		"nonce":         nonce,
		"pkce_verifier": pkceVerifier,
		"return_to":     returnTo,
	}
	loginHint := ""
	if targetIdentity != nil {
		storedState["target_identity_id"] = targetIdentity.ID
		storedState["target_subject"] = targetIdentity.Subject
		loginHint = targetIdentity.Email
	}
	if err := s.Redis.SetState(ctx, state, storedState, authorizationStateTTL); err != nil {
		return AuthorizationStart{}, err
	}
	authorizationURL, err := buildAuthorizationURL(*provider, s.RedirectURI(), state, nonce, pkceVerifier, intent, loginHint)
	if err != nil {
		_ = s.Redis.DeleteState(ctx, state)
		return AuthorizationStart{}, err
	}
	return AuthorizationStart{AuthorizationURL: authorizationURL, ExpiresIn: int(authorizationStateTTL / time.Second)}, nil
}

func (s Service) CompleteAuthorization(ctx context.Context, code, state, providerError string) (AuthorizationResult, error) {
	if strings.TrimSpace(state) == "" {
		return AuthorizationResult{}, badRequest("oidc_state", "validate", "required")
	}
	if s.Redis == nil {
		return AuthorizationResult{}, errors.New("identity state store is not configured")
	}
	stored, err := s.Redis.PopState(ctx, state)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return AuthorizationResult{}, badRequest("oidc_state", "verify", "invalid")
	}
	if err != nil {
		return AuthorizationResult{}, err
	}
	if stateString(stored, "kind") != authorizationStateKind {
		return AuthorizationResult{}, badRequest("oidc_state", "verify", "invalid")
	}
	if providerError != "" {
		if stateString(stored, "intent") == AuthorizationIntentLink {
			return AuthorizationResult{}, badRequest("identity", "authorize", "incomplete")
		}
		return AuthorizationResult{}, badRequest("identity", "authorize", "denied")
	}
	if strings.TrimSpace(code) == "" {
		return AuthorizationResult{}, badRequest("authorization_code", "validate", "required")
	}
	providerID := stateString(stored, "provider_id")
	provider, err := s.DB.Identities.GetProvider(ctx, providerID)
	if err != nil {
		return AuthorizationResult{}, err
	}
	if provider == nil || !provider.Enabled {
		return AuthorizationResult{}, badRequest("identity_provider", "use", "unavailable")
	}
	box, err := util.NewSecretBox(s.Config.IdentityEncryptionKey)
	if err != nil {
		return AuthorizationResult{}, err
	}
	clientSecret, err := box.Decrypt(provider.ClientSecretCiphertext)
	if err != nil {
		return AuthorizationResult{}, err
	}
	client := s.OIDCClient
	if client == nil {
		client = StandardOIDCClient{}
	}
	claims, tokens, err := client.ExchangeAndVerify(
		ctx,
		*provider,
		clientSecret,
		code,
		s.RedirectURI(),
		stateString(stored, "pkce_verifier"),
		stateString(stored, "nonce"),
	)
	if err != nil {
		object, operation, reason, params := util.ErrorClassification(err)
		if object == util.InternalErrorObject {
			object, operation, reason = "identity", "authorize", "failed"
		}
		return AuthorizationResult{}, badRequest(object, operation, reason, params)
	}
	intent := stateString(stored, "intent")
	returnTo := stateString(stored, "return_to")
	existing, err := s.DB.Identities.GetByProviderSubject(ctx, provider.ID, claims.Subject)
	if err != nil {
		return AuthorizationResult{}, err
	}
	switch intent {
	case AuthorizationIntentLink:
		userID := stateString(stored, "user_id")
		if userID == "" {
			return AuthorizationResult{}, forbidden()
		}
		targetIdentityID := stateString(stored, "target_identity_id")
		if targetIdentityID != "" {
			if existing == nil || existing.ID != targetIdentityID || existing.UserID != userID ||
				claims.Subject != stateString(stored, "target_subject") {
				return AuthorizationResult{}, conflict("identity", "authorize", "mismatch")
			}
			if err := s.updateIdentityAuthorization(ctx, *existing, claims, tokens); err != nil {
				return AuthorizationResult{}, err
			}
			return AuthorizationResult{Intent: intent, UserID: userID, IdentityID: existing.ID, ProviderID: provider.ID}, nil
		}
		if existing != nil {
			if existing.UserID == userID {
				return AuthorizationResult{}, conflict("identity", "link", "already_exists")
			}
			return AuthorizationResult{}, conflict("identity", "link", "conflict")
		}
		identityID, err := s.createIdentity(ctx, userID, *provider, claims, tokens)
		if err != nil {
			return AuthorizationResult{}, err
		}
		return AuthorizationResult{Intent: intent, UserID: userID, IdentityID: identityID, ProviderID: provider.ID}, nil
	case AuthorizationIntentLogin:
		if existing != nil {
			if err := s.updateIdentityAuthorization(ctx, *existing, claims, tokens); err != nil {
				return AuthorizationResult{}, err
			}
			return AuthorizationResult{Intent: intent, UserID: existing.UserID, IdentityID: existing.ID, ProviderID: provider.ID, ReturnTo: returnTo}, nil
		}
		if !provider.LoginEnabled {
			return AuthorizationResult{}, forbiddenCode("identity_provider", "login", "disabled")
		}
		ticket, err := s.createRegistrationTicket(ctx, *provider, claims, tokens)
		if err != nil {
			return AuthorizationResult{}, err
		}
		return AuthorizationResult{Intent: "registration", RegistrationTicket: ticket, ProviderID: provider.ID, ReturnTo: returnTo}, nil
	default:
		return AuthorizationResult{}, badRequest("authorization_intent", "validate", "invalid")
	}
}

func (s Service) createIdentity(ctx context.Context, userID string, provider model.IdentityProvider, claims OIDCClaims, tokens OIDCTokens) (string, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return "", err
	}
	now := database.NowMS()
	identity := model.ExternalIdentity{
		ID: id, UserID: userID, ProviderID: provider.ID, Subject: claims.Subject,
		Email: claims.Email, EmailVerified: claims.EmailVerified, DisplayName: claims.DisplayName,
		AvatarURL: claims.AvatarURL, CreatedAt: now, UpdatedAt: now, LastLoginAt: &now,
	}
	credential, err := s.credential(id, tokens, now)
	if err != nil {
		return "", err
	}
	if err := s.DB.Identities.CreateIdentity(ctx, identity, credential); err != nil {
		if isUniqueViolation(err) {
			return "", conflict("identity", "link", "already_exists")
		}
		return "", err
	}
	if err := s.cacheAccessToken(ctx, id, tokens); err != nil {
		_, _ = s.DB.Identities.DeleteIdentity(ctx, id, userID)
		return "", err
	}
	return id, nil
}

func (s Service) updateIdentityAuthorization(ctx context.Context, item model.ExternalIdentity, claims OIDCClaims, tokens OIDCTokens) error {
	now := database.NowMS()
	item.Email = claims.Email
	item.EmailVerified = claims.EmailVerified
	item.DisplayName = claims.DisplayName
	item.AvatarURL = claims.AvatarURL
	item.UpdatedAt = now
	item.LastLoginAt = &now
	credential, err := s.credential(item.ID, tokens, now)
	if err != nil {
		return err
	}
	if err := s.cacheAccessToken(ctx, item.ID, tokens); err != nil {
		return err
	}
	updated, err := s.DB.Identities.UpdateIdentityAuthorization(ctx, item, credential)
	if err != nil || !updated {
		_ = s.Redis.DeleteExternalAccessToken(ctx, item.ID)
		if err != nil {
			return err
		}
		return notFound("identity", "resolve", "not_found")
	}
	return nil
}

func (s Service) credential(identityID string, tokens OIDCTokens, now int64) (model.ExternalIdentityCredential, error) {
	box, err := util.NewSecretBox(s.Config.IdentityEncryptionKey)
	if err != nil {
		return model.ExternalIdentityCredential{}, err
	}
	refreshCiphertext, err := box.Encrypt(tokens.RefreshToken)
	if err != nil {
		return model.ExternalIdentityCredential{}, err
	}
	return model.ExternalIdentityCredential{
		IdentityID: identityID, RefreshTokenCiphertext: refreshCiphertext,
		GrantedScopes:       append([]string(nil), tokens.Scopes...),
		AuthorizationStatus: model.ExternalIdentityAuthorizationActive, UpdatedAt: now,
	}, nil
}

func (s Service) cacheAccessToken(ctx context.Context, identityID string, tokens OIDCTokens) error {
	if tokens.AccessToken == "" {
		return nil
	}
	ttl := time.Until(tokens.Expiry)
	expiresAt := tokens.Expiry.UnixMilli()
	if tokens.Expiry.IsZero() || ttl <= 0 {
		ttl = 5 * time.Minute
		expiresAt = time.Now().Add(ttl).UnixMilli()
	}
	return s.Redis.SetExternalAccessToken(ctx, redisstore.ExternalAccessToken{
		IdentityID: identityID, AccessToken: tokens.AccessToken, TokenType: tokens.TokenType,
		ExpiresAt: expiresAt,
	}, ttl)
}

func (s Service) createRegistrationTicket(ctx context.Context, provider model.IdentityProvider, claims OIDCClaims, tokens OIDCTokens) (string, error) {
	ticket, err := opaqueToken()
	if err != nil {
		return "", err
	}
	expiresAt := int64(0)
	if !tokens.Expiry.IsZero() {
		expiresAt = tokens.Expiry.UnixMilli()
	}
	if err := s.Redis.SetState(ctx, ticket, map[string]any{
		"kind":           registrationStateKind,
		"provider_id":    provider.ID,
		"subject":        claims.Subject,
		"email":          claims.Email,
		"email_verified": claims.EmailVerified,
		"display_name":   claims.DisplayName,
		"avatar_url":     claims.AvatarURL,
		"access_token":   tokens.AccessToken,
		"refresh_token":  tokens.RefreshToken,
		"token_type":     tokens.TokenType,
		"expires_at":     expiresAt,
		"scopes":         tokens.Scopes,
	}, registrationStateTTL); err != nil {
		return "", err
	}
	return ticket, nil
}

func buildAuthorizationURL(provider model.IdentityProvider, redirectURI, state, nonce, pkceVerifier, intent, loginHint string) (string, error) {
	u, err := url.Parse(provider.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	challengeSum := sha256.Sum256([]byte(pkceVerifier))
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", provider.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(provider.Scopes, " "))
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challengeSum[:]))
	query.Set("code_challenge_method", "S256")
	if intent == AuthorizationIntentLink && provider.Adapter == AdapterMicrosoft {
		query.Set("prompt", "select_account")
	}
	if strings.TrimSpace(loginHint) != "" {
		query.Set("login_hint", strings.TrimSpace(loginHint))
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (s Service) RedirectURI() string {
	base := strings.TrimRight(strings.TrimSpace(s.Config.APIURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(s.Config.SiteURL), "/")
	}
	return base + "/v2/auth/oidc/callback"
}

func normalizedReturnTo(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, `\`) || strings.IndexFunc(raw, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", badRequest("return_to", "validate", "invalid")
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || u.Path == "/login" {
		return "", badRequest("return_to", "validate", "invalid")
	}
	return u.String(), nil
}

func opaqueToken() (string, error) {
	raw, _, err := util.GenerateRefreshToken()
	return raw, err
}

func stateString(state map[string]any, key string) string {
	value, _ := state[key].(string)
	return value
}

func stateBool(state map[string]any, key string) bool {
	value, _ := state[key].(bool)
	return value
}

func stateInt64(state map[string]any, key string) int64 {
	switch value := state[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	default:
		return 0
	}
}

func stateStrings(state map[string]any, key string) []string {
	values, ok := state[key].([]any)
	if !ok {
		if stringsValue, ok := state[key].([]string); ok {
			return append([]string(nil), stringsValue...)
		}
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if item, ok := value.(string); ok {
			out = append(out, item)
		}
	}
	return out
}

func forbiddenCode(object, operation, reason string) util.HTTPError {
	return util.HTTPError{Status: http.StatusForbidden, Object: object, Operation: operation, Reason: reason}
}
