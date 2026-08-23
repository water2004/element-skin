package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

type PendingRegistration struct {
	Ticket   string
	Provider model.IdentityProvider
	Claims   OIDCClaims
	Tokens   OIDCTokens
}

func (s Service) ConsumeRegistration(ctx context.Context, ticket string) (PendingRegistration, error) {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return PendingRegistration{}, badRequest("identity_ticket", "validate", "required")
	}
	if s.Redis == nil {
		return PendingRegistration{}, errors.New("identity state store is not configured")
	}
	state, err := s.Redis.PopState(ctx, ticket)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return PendingRegistration{}, badRequest("identity_ticket", "verify", "invalid")
	}
	if err != nil {
		return PendingRegistration{}, err
	}
	if stateString(state, "kind") != registrationStateKind {
		return PendingRegistration{}, badRequest("identity_ticket", "verify", "invalid")
	}
	provider, err := s.DB.Identities.GetProvider(ctx, stateString(state, "provider_id"))
	if err != nil {
		return PendingRegistration{}, err
	}
	if provider == nil || !provider.Enabled || !provider.LoginEnabled {
		return PendingRegistration{}, forbiddenCode("registration", "create", "disabled")
	}
	claims := OIDCClaims{
		Subject:       stateString(state, "subject"),
		Email:         stateString(state, "email"),
		EmailVerified: stateBool(state, "email_verified"),
		DisplayName:   stateString(state, "display_name"),
		AvatarURL:     stateString(state, "avatar_url"),
	}
	if claims.Subject == "" {
		return PendingRegistration{}, badRequest("identity_ticket", "verify", "invalid")
	}
	if existing, err := s.DB.Identities.GetByProviderSubject(ctx, provider.ID, claims.Subject); err != nil {
		return PendingRegistration{}, err
	} else if existing != nil {
		return PendingRegistration{}, conflict("identity", "link", "already_exists")
	}
	tokens := OIDCTokens{
		AccessToken:  stateString(state, "access_token"),
		RefreshToken: stateString(state, "refresh_token"),
		TokenType:    stateString(state, "token_type"),
		Scopes:       stateStrings(state, "scopes"),
	}
	if expiresAt := stateInt64(state, "expires_at"); expiresAt > 0 {
		tokens.Expiry = time.UnixMilli(expiresAt)
	}
	return PendingRegistration{Ticket: ticket, Provider: *provider, Claims: claims, Tokens: tokens}, nil
}

func (s Service) RestoreRegistration(ctx context.Context, pending PendingRegistration) error {
	expiresAt := int64(0)
	if !pending.Tokens.Expiry.IsZero() {
		expiresAt = pending.Tokens.Expiry.UnixMilli()
	}
	return s.Redis.SetState(ctx, pending.Ticket, map[string]any{
		"kind":           registrationStateKind,
		"provider_id":    pending.Provider.ID,
		"subject":        pending.Claims.Subject,
		"email":          pending.Claims.Email,
		"email_verified": pending.Claims.EmailVerified,
		"display_name":   pending.Claims.DisplayName,
		"avatar_url":     pending.Claims.AvatarURL,
		"access_token":   pending.Tokens.AccessToken,
		"refresh_token":  pending.Tokens.RefreshToken,
		"token_type":     pending.Tokens.TokenType,
		"expires_at":     expiresAt,
		"scopes":         pending.Tokens.Scopes,
	}, registrationStateTTL)
}

func (s Service) RegistrationRecords(userID string, pending PendingRegistration) (model.ExternalIdentity, model.ExternalIdentityCredential, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return model.ExternalIdentity{}, model.ExternalIdentityCredential{}, err
	}
	now := database.NowMS()
	item := model.ExternalIdentity{
		ID: id, UserID: userID, ProviderID: pending.Provider.ID, Subject: pending.Claims.Subject,
		Email: pending.Claims.Email, EmailVerified: pending.Claims.EmailVerified,
		DisplayName: pending.Claims.DisplayName, AvatarURL: pending.Claims.AvatarURL,
		CreatedAt: now, UpdatedAt: now, LastLoginAt: &now,
	}
	credential, err := s.credential(id, pending.Tokens, now)
	return item, credential, err
}

func (s Service) CacheRegistrationAccess(ctx context.Context, identityID string, tokens OIDCTokens) error {
	return s.cacheAccessToken(ctx, identityID, tokens)
}
