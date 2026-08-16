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

	AuthorizationAccountMismatchDetail = "authorized account does not match the external identity being reconnected"
	AuthorizationLinkIncompleteDetail  = "external identity authorization was not completed"
	AuthorizationAlreadyLinkedDetail   = "this external identity is already linked to this account"
	AuthorizationLinkedElsewhereDetail = "this external identity is already linked to another account"

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
}

func (s Service) StartAuthorization(ctx context.Context, actor permission.Actor, providerID, intent, identityID string) (AuthorizationStart, error) {
	provider, err := s.DB.Identities.GetProvider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return AuthorizationStart{}, err
	}
	if provider == nil || !provider.Enabled {
		return AuthorizationStart{}, notFound("identity provider not found")
	}
	intent = strings.TrimSpace(intent)
	identityID = strings.TrimSpace(identityID)
	var targetIdentity *model.ExternalIdentity
	switch intent {
	case AuthorizationIntentLogin:
		if identityID != "" {
			return AuthorizationStart{}, badRequest("identity_id is only allowed for link authorization")
		}
		if !provider.LoginEnabled {
			return AuthorizationStart{}, forbiddenDetail("login is disabled for this identity provider")
		}
	case AuthorizationIntentLink:
		if err := actor.Require(permission.MustDefinitionByCode("external_identity.create.owned")); err != nil || actor.UserID == "" {
			return AuthorizationStart{}, forbidden()
		}
		if !provider.LinkEnabled {
			return AuthorizationStart{}, forbiddenDetail("linking is disabled for this identity provider")
		}
		if identityID != "" {
			targetIdentity, err = s.DB.Identities.GetIdentity(ctx, identityID)
			if err != nil {
				return AuthorizationStart{}, err
			}
			if targetIdentity == nil || targetIdentity.UserID != actor.UserID || targetIdentity.ProviderID != provider.ID {
				return AuthorizationStart{}, notFound("external identity not found")
			}
		}
	default:
		return AuthorizationStart{}, badRequest("intent must be login or link")
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
		return AuthorizationResult{}, badRequest("state is required")
	}
	if s.Redis == nil {
		return AuthorizationResult{}, errors.New("identity state store is not configured")
	}
	stored, err := s.Redis.PopState(ctx, state)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return AuthorizationResult{}, badRequest("invalid or expired OIDC state")
	}
	if err != nil {
		return AuthorizationResult{}, err
	}
	if stateString(stored, "kind") != authorizationStateKind {
		return AuthorizationResult{}, badRequest("invalid or expired OIDC state")
	}
	if providerError != "" {
		if stateString(stored, "intent") == AuthorizationIntentLink {
			return AuthorizationResult{}, badRequest(AuthorizationLinkIncompleteDetail)
		}
		return AuthorizationResult{}, badRequest("OIDC authorization was denied")
	}
	if strings.TrimSpace(code) == "" {
		return AuthorizationResult{}, badRequest("authorization code is required")
	}
	providerID := stateString(stored, "provider_id")
	provider, err := s.DB.Identities.GetProvider(ctx, providerID)
	if err != nil {
		return AuthorizationResult{}, err
	}
	if provider == nil || !provider.Enabled {
		return AuthorizationResult{}, badRequest("identity provider is no longer available")
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
		return AuthorizationResult{}, badRequest(err.Error())
	}
	intent := stateString(stored, "intent")
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
				return AuthorizationResult{}, conflict(AuthorizationAccountMismatchDetail)
			}
			if err := s.updateIdentityAuthorization(ctx, *existing, claims, tokens); err != nil {
				return AuthorizationResult{}, err
			}
			return AuthorizationResult{Intent: intent, UserID: userID, IdentityID: existing.ID, ProviderID: provider.ID}, nil
		}
		if existing != nil {
			if existing.UserID == userID {
				return AuthorizationResult{}, conflict(AuthorizationAlreadyLinkedDetail)
			}
			return AuthorizationResult{}, conflict(AuthorizationLinkedElsewhereDetail)
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
			return AuthorizationResult{Intent: intent, UserID: existing.UserID, IdentityID: existing.ID, ProviderID: provider.ID}, nil
		}
		if !provider.LoginEnabled {
			return AuthorizationResult{}, forbiddenDetail("login is disabled for this identity provider")
		}
		ticket, err := s.createRegistrationTicket(ctx, *provider, claims, tokens)
		if err != nil {
			return AuthorizationResult{}, err
		}
		return AuthorizationResult{Intent: "registration", RegistrationTicket: ticket, ProviderID: provider.ID}, nil
	default:
		return AuthorizationResult{}, badRequest("invalid OIDC authorization intent")
	}
}

func AuthorizationLinkErrorCode(err error) (string, bool) {
	httpError, ok := err.(util.HTTPError)
	if !ok {
		return "", false
	}
	switch httpError.Detail {
	case AuthorizationAccountMismatchDetail:
		return "account_mismatch", true
	case AuthorizationLinkIncompleteDetail:
		return "authorization_incomplete", true
	case AuthorizationAlreadyLinkedDetail, AuthorizationLinkedElsewhereDetail:
		return "already_linked", true
	default:
		return "", false
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
			return "", conflict("this external identity is already linked")
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
		return notFound("external identity not found")
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

func forbiddenDetail(detail string) util.HTTPError {
	return util.HTTPError{Status: http.StatusForbidden, Detail: detail}
}
