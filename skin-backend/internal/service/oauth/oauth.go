package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

const (
	ClientTypePublic       = "public"
	ClientTypeConfidential = "confidential"
	StatusActive           = "active"
	StatusDisabled         = "disabled"

	authorizationCodeTTL = 10 * time.Minute
	deviceCodeTTL        = 10 * time.Minute
	devicePollInterval   = 5 * time.Second
	accessTokenTTL       = time.Hour
	refreshTokenTTL      = 30 * 24 * time.Hour
)

type Service struct {
	DB *database.DB
}

type ClientInput struct {
	Name            string
	Description     string
	RedirectURI     string
	WebsiteURL      string
	ClientType      string
	PermissionCodes []string
}

type AuthorizationRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type DeviceAuthorizationRequest struct {
	ClientID     string
	ClientSecret string
	Scope        string
}

type DeviceAuthorizationResponse struct {
	DeviceCode  string   `json:"device_code"`
	UserCode    string   `json:"user_code"`
	ExpiresIn   int64    `json:"expires_in"`
	Interval    int64    `json:"interval"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

type DeviceAuthorizationDetails struct {
	Client    map[string]any   `json:"client"`
	Scopes    []map[string]any `json:"scopes"`
	ExpiresAt int64            `json:"expires_at"`
	Status    string           `json:"status"`
}

type DeviceDecisionRequest struct {
	UserCode string
	Approve  bool
}

type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
	RefreshToken string
	Scope        string
	DeviceCode   string
}

type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int64    `json:"expires_in"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	Scope        string   `json:"scope"`
	Permissions  []string `json:"permissions"`
}

type AuthorizationDetails struct {
	Client      map[string]any   `json:"client"`
	Scopes      []map[string]any `json:"scopes"`
	RedirectURI string           `json:"redirect_uri"`
	State       string           `json:"state,omitempty"`
}

func (s Service) CreateClient(ctx context.Context, actor permission.Actor, input ClientInput) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.create.owned")); err != nil {
		return nil, forbidden()
	}
	client, permissionIDs, permissionCodes, err := s.clientFromInput(actor, input)
	if err != nil {
		return nil, err
	}
	client.ID, err = util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	client.OwnerUserID = actor.UserID
	client.Status = StatusActive
	client.CreatedAt = database.NowMS()
	client.UpdatedAt = client.CreatedAt
	secret := ""
	if client.ClientType == ClientTypeConfidential {
		secret, client.SecretHash, err = generateSecret()
		if err != nil {
			return nil, err
		}
	}
	if err := s.DB.OAuth.CreateClient(ctx, client, permissionIDs); err != nil {
		return nil, err
	}
	return clientResponse(client, permissionCodes, secret), nil
}

func (s Service) ListClients(ctx context.Context, actor permission.Actor, limit int) ([]map[string]any, error) {
	var clients []model.OAuthClient
	var err error
	if actor.Has(permission.MustDefinitionByCode("oauth_app.read.any")) {
		clients, err = s.DB.OAuth.ListClients(ctx, limit)
	} else {
		if err := actor.Require(permission.MustDefinitionByCode("oauth_app.read.owned")); err != nil {
			return nil, forbidden()
		}
		clients, err = s.DB.OAuth.ListClientsByOwner(ctx, actor.UserID, limit)
	}
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(clients))
	for _, client := range clients {
		codes, err := s.clientPermissionCodes(ctx, client.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, clientResponse(client, codes, ""))
	}
	return out, nil
}

func (s Service) GetClient(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.read.owned", "oauth_app.read.any")
	if err != nil {
		return nil, err
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, ""), nil
}

func (s Service) UpdateClient(ctx context.Context, actor permission.Actor, clientID string, input ClientInput, status string) (map[string]any, error) {
	current, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	client, permissionIDs, permissionCodes, err := s.clientFromInput(actor, input)
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = current.Status
	}
	if status != StatusActive && status != StatusDisabled {
		return nil, badRequest("invalid status")
	}
	client.ID = current.ID
	client.OwnerUserID = current.OwnerUserID
	client.SecretHash = current.SecretHash
	client.Status = status
	client.CreatedAt = current.CreatedAt
	client.UpdatedAt = database.NowMS()
	updated, err := s.DB.OAuth.UpdateClient(ctx, client, permissionIDs)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, notFound("oauth client not found")
	}
	return clientResponse(client, permissionCodes, ""), nil
}

func (s Service) RotateClientSecret(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	if client.ClientType != ClientTypeConfidential {
		return nil, badRequest("public clients do not have secrets")
	}
	raw, hash, err := generateSecret()
	if err != nil {
		return nil, err
	}
	updatedAt := database.NowMS()
	ok, err := s.DB.OAuth.RotateClientSecret(ctx, client.ID, hash, updatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound("oauth client not found")
	}
	client.SecretHash = hash
	client.UpdatedAt = updatedAt
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, raw), nil
}

func (s Service) DeleteClient(ctx context.Context, actor permission.Actor, clientID string) error {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.delete.owned", "oauth_app.update.any")
	if err != nil {
		return err
	}
	owner := client.OwnerUserID
	if actor.Has(permission.MustDefinitionByCode("oauth_app.update.any")) {
		owner = ""
	}
	ok, err := s.DB.OAuth.DeleteClient(ctx, client.ID, owner)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("oauth client not found")
	}
	return nil
}

func (s Service) ListGrants(ctx context.Context, actor permission.Actor, limit int) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_grant.read.owned")); err != nil {
		return nil, forbidden()
	}
	grants, err := s.DB.OAuth.ListGrantsByUser(ctx, actor.UserID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(grants))
	for _, grant := range grants {
		codes, err := s.grantPermissionCodes(ctx, grant.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, grantResponse(grant, codes))
	}
	return out, nil
}

func (s Service) RevokeGrant(ctx context.Context, actor permission.Actor, grantID string) error {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_grant.revoke.owned")); err != nil {
		return forbidden()
	}
	ok, err := s.DB.OAuth.RevokeGrant(ctx, grantID, actor.UserID, database.NowMS())
	if err != nil {
		return err
	}
	if !ok {
		return notFound("oauth grant not found")
	}
	return nil
}

func (s Service) AuthorizationDetails(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (AuthorizationDetails, error) {
	client, codes, err := s.validAuthorizationRequest(ctx, actor, req)
	if err != nil {
		return AuthorizationDetails{}, err
	}
	return AuthorizationDetails{
		Client:      publicClient(client),
		Scopes:      permissionDetails(codes),
		RedirectURI: req.RedirectURI,
		State:       req.State,
	}, nil
}

func (s Service) ApproveAuthorization(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (map[string]any, error) {
	client, codes, err := s.validAuthorizationRequest(ctx, actor, req)
	if err != nil {
		return nil, err
	}
	permissionIDs := permissionIDsFromCodes(codes)
	now := database.NowMS()
	grantID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	grant := model.OAuthGrant{
		ID:        grantID,
		UserID:    actor.UserID,
		SubjectID: permissiondb.SubjectIDForUser(actor.UserID),
		ClientID:  client.ID,
		Status:    StatusActive,
		CreatedAt: now,
	}
	if err := s.DB.OAuth.CreateGrant(ctx, grant, permissionIDs); err != nil {
		return nil, err
	}
	rawCode, codeHash, err := generateToken()
	if err != nil {
		return nil, err
	}
	code := model.OAuthAuthorizationCode{
		CodeHash:            codeHash,
		ClientID:            client.ID,
		UserID:              actor.UserID,
		GrantID:             grantID,
		RedirectURI:         req.RedirectURI,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           now + int64(authorizationCodeTTL/time.Millisecond),
		CreatedAt:           now,
	}
	if err := s.DB.OAuth.CreateAuthorizationCode(ctx, code, permissionIDs); err != nil {
		return nil, err
	}
	redirectURL, err := authorizationRedirect(req.RedirectURI, rawCode, req.State)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"code":         rawCode,
		"redirect_url": redirectURL,
		"state":        req.State,
	}, nil
}

func (s Service) IssueToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	switch req.GrantType {
	case "authorization_code":
		return s.exchangeAuthorizationCode(ctx, req)
	case "refresh_token":
		return s.refreshToken(ctx, req)
	case "client_credentials":
		return s.clientCredentialsToken(ctx, req)
	case "urn:ietf:params:oauth:grant-type:device_code":
		return s.deviceCodeToken(ctx, req)
	default:
		return TokenResponse{}, badRequest("unsupported grant_type")
	}
}

func (s Service) StartDeviceAuthorization(ctx context.Context, req DeviceAuthorizationRequest) (DeviceAuthorizationResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return DeviceAuthorizationResponse{}, err
	}
	codes, err := parseScope(req.Scope)
	if err != nil {
		return DeviceAuthorizationResponse{}, err
	}
	clientIDs, err := s.DB.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		return DeviceAuthorizationResponse{}, err
	}
	clientAllowed := idSet(clientIDs)
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		if !clientAllowed[int64(def.ID)] {
			return DeviceAuthorizationResponse{}, badRequest("scope exceeds client permission limit")
		}
	}
	deviceCode, deviceHash, err := generateToken()
	if err != nil {
		return DeviceAuthorizationResponse{}, err
	}
	userCode, userHash, err := generateUserCode()
	if err != nil {
		return DeviceAuthorizationResponse{}, err
	}
	now := database.NowMS()
	record := model.OAuthDeviceCode{
		DeviceCodeHash: deviceHash,
		UserCodeHash:   userHash,
		ClientID:       client.ID,
		Status:         "pending",
		ExpiresAt:      now + int64(deviceCodeTTL/time.Millisecond),
		CreatedAt:      now,
	}
	if err := s.DB.OAuth.CreateDeviceCode(ctx, record, permissionIDsFromCodes(codes)); err != nil {
		return DeviceAuthorizationResponse{}, err
	}
	return DeviceAuthorizationResponse{
		DeviceCode:  deviceCode,
		UserCode:    userCode,
		ExpiresIn:   int64(deviceCodeTTL / time.Second),
		Interval:    int64(devicePollInterval / time.Second),
		Scope:       strings.Join(codes, " "),
		Permissions: codes,
	}, nil
}

func (s Service) DeviceAuthorizationDetails(ctx context.Context, actor permission.Actor, userCode string) (DeviceAuthorizationDetails, error) {
	if actor.UserID == "" {
		return DeviceAuthorizationDetails{}, forbidden()
	}
	code, permissionIDs, err := s.DB.OAuth.GetDeviceCodeByUserCodeHash(ctx, util.HashRefreshToken(normalizeUserCode(userCode)))
	if err != nil {
		return DeviceAuthorizationDetails{}, err
	}
	if code == nil || code.ExpiresAt <= database.NowMS() {
		return DeviceAuthorizationDetails{}, notFound("device code not found")
	}
	client, err := s.DB.OAuth.GetClient(ctx, code.ClientID)
	if err != nil {
		return DeviceAuthorizationDetails{}, err
	}
	if client == nil || client.Status != StatusActive {
		return DeviceAuthorizationDetails{}, notFound("oauth client not found")
	}
	return DeviceAuthorizationDetails{
		Client:    publicClient(*client),
		Scopes:    permissionDetails(permissionCodesFromIDs(permissionIDs)),
		ExpiresAt: code.ExpiresAt,
		Status:    code.Status,
	}, nil
}

func (s Service) DecideDeviceAuthorization(ctx context.Context, actor permission.Actor, req DeviceDecisionRequest) error {
	if actor.UserID == "" {
		return forbidden()
	}
	userHash := util.HashRefreshToken(normalizeUserCode(req.UserCode))
	code, permissionIDs, err := s.DB.OAuth.GetDeviceCodeByUserCodeHash(ctx, userHash)
	if err != nil {
		return err
	}
	if code == nil || code.ExpiresAt <= database.NowMS() {
		return notFound("device code not found")
	}
	if code.Status != "pending" {
		return badRequest("device code is not pending")
	}
	codes := permissionCodesFromIDs(permissionIDs)
	for _, scope := range codes {
		if !actor.Has(permission.MustDefinitionByCode(scope)) {
			return forbidden()
		}
	}
	now := database.NowMS()
	var ok bool
	if req.Approve {
		ok, err = s.DB.OAuth.ApproveDeviceCode(ctx, userHash, actor.UserID, actor.SubjectID, now)
	} else {
		ok, err = s.DB.OAuth.DenyDeviceCode(ctx, userHash, now)
	}
	if err != nil {
		return err
	}
	if !ok {
		return badRequest("device code is not pending")
	}
	return nil
}

func (s Service) RevokeToken(ctx context.Context, clientID, clientSecret, token string) error {
	client, err := s.authenticateClient(ctx, clientID, clientSecret)
	if err != nil {
		return err
	}
	tokenHash := util.HashRefreshToken(token)
	if access, err := s.DB.OAuth.GetAccessToken(ctx, tokenHash); err != nil {
		return err
	} else if access != nil {
		if access.ClientID != client.ID {
			return forbidden()
		}
		_, err = s.DB.OAuth.RevokeAccessToken(ctx, tokenHash, database.NowMS())
		return err
	}
	if refresh, err := s.DB.OAuth.GetRefreshToken(ctx, tokenHash); err != nil {
		return err
	} else if refresh != nil {
		if refresh.ClientID != client.ID {
			return forbidden()
		}
		_, err = s.DB.OAuth.RevokeRefreshToken(ctx, tokenHash, database.NowMS())
		return err
	}
	if clientAccess, _, err := s.DB.OAuth.GetClientAccessToken(ctx, tokenHash); err != nil {
		return err
	} else if clientAccess != nil {
		if clientAccess.ClientID != client.ID {
			return forbidden()
		}
		_, err = s.DB.OAuth.RevokeClientAccessToken(ctx, tokenHash, database.NowMS())
		return err
	}
	return nil
}

func (s Service) Introspect(ctx context.Context, actor permission.Actor, token string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_token.introspect.any")); err != nil {
		return nil, forbidden()
	}
	tokenHash := util.HashRefreshToken(token)
	access, err := s.DB.OAuth.GetAccessToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if access == nil || access.RevokedAt != nil || access.ExpiresAt <= database.NowMS() {
		clientAccess, permissionIDs, err := s.DB.OAuth.GetClientAccessToken(ctx, tokenHash)
		if err != nil {
			return nil, err
		}
		if clientAccess == nil || clientAccess.RevokedAt != nil || clientAccess.ExpiresAt <= database.NowMS() {
			return map[string]any{"active": false}, nil
		}
		codes := permissionCodesFromIDs(permissionIDs)
		return map[string]any{
			"active":      true,
			"client_id":   clientAccess.ClientID,
			"subject_id":  permissiondb.SubjectIDForClient(clientAccess.ClientID),
			"exp":         clientAccess.ExpiresAt / 1000,
			"scope":       strings.Join(codes, " "),
			"permissions": codes,
		}, nil
	}
	codes, err := s.grantPermissionCodes(ctx, access.GrantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"active":      true,
		"client_id":   access.ClientID,
		"user_id":     access.UserID,
		"grant_id":    access.GrantID,
		"exp":         access.ExpiresAt / 1000,
		"scope":       strings.Join(codes, " "),
		"permissions": codes,
	}, nil
}

func (s Service) ActorForBearer(ctx context.Context, bearer string) (permission.Actor, bool, error) {
	tokenHash := util.HashRefreshToken(bearer)
	token, err := s.DB.OAuth.GetAccessToken(ctx, tokenHash)
	if err != nil {
		return permission.Actor{}, false, err
	}
	if token != nil && token.RevokedAt == nil && token.ExpiresAt > database.NowMS() {
		actor, err := s.DB.Permissions.ActorForUser(ctx, token.UserID, permissiondb.EffectiveOptions{
			SessionKind:       permission.SessionKindDelegated,
			Entrypoint:        permission.EntrypointDashboard,
			DelegatedClientID: token.ClientID,
			DelegatedGrantID:  token.GrantID,
		})
		if err != nil {
			return permission.Actor{}, false, err
		}
		actor.SessionID = token.TokenHash
		return actor, true, nil
	}

	clientToken, permissionIDs, err := s.DB.OAuth.GetClientAccessToken(ctx, tokenHash)
	if err != nil {
		return permission.Actor{}, false, err
	}
	if clientToken == nil || clientToken.RevokedAt != nil || clientToken.ExpiresAt <= database.NowMS() {
		return permission.Actor{}, false, nil
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientToken.ClientID)
	if err != nil {
		return permission.Actor{}, false, err
	}
	if client == nil || client.Status != StatusActive {
		return permission.Actor{}, false, nil
	}
	actor, err := s.DB.Permissions.ActorForClient(ctx, clientToken.ClientID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindClient,
		Entrypoint:  permission.EntrypointAPI,
	})
	if err != nil {
		return permission.Actor{}, false, err
	}
	actor.SessionID = clientToken.TokenHash
	actor.Permissions = actor.Permissions.And(bitSetFromPermissionIDs(permissionIDs))
	return actor, true, nil
}

func (s Service) exchangeAuthorizationCode(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	codeHash := util.HashRefreshToken(req.Code)
	code, permissionIDs, err := s.DB.OAuth.ConsumeAuthorizationCode(ctx, codeHash, database.NowMS())
	if err != nil {
		return TokenResponse{}, err
	}
	if code == nil || code.ClientID != client.ID || code.RedirectURI != req.RedirectURI {
		return TokenResponse{}, badRequest("invalid authorization code")
	}
	if !validPKCE(req.CodeVerifier, code.CodeChallenge) {
		return TokenResponse{}, badRequest("invalid code_verifier")
	}
	return s.issueTokens(ctx, client.ID, code.UserID, code.GrantID, permissionCodesFromIDs(permissionIDs))
}

func (s Service) refreshToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	oldHash := util.HashRefreshToken(req.RefreshToken)
	old, err := s.DB.OAuth.GetRefreshToken(ctx, oldHash)
	if err != nil {
		return TokenResponse{}, err
	}
	if old == nil || old.ClientID != client.ID || old.RevokedAt != nil || old.ExpiresAt <= database.NowMS() {
		return TokenResponse{}, badRequest("invalid refresh_token")
	}
	codes, err := s.grantPermissionCodes(ctx, old.GrantID)
	if err != nil {
		return TokenResponse{}, err
	}
	accessRaw, accessHash, refreshRaw, refreshHash, err := tokenPair()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	access := model.OAuthToken{TokenHash: accessHash, ClientID: client.ID, UserID: old.UserID, GrantID: old.GrantID, ExpiresAt: now + int64(accessTokenTTL/time.Millisecond), CreatedAt: now}
	refresh := model.OAuthToken{TokenHash: refreshHash, ClientID: client.ID, UserID: old.UserID, GrantID: old.GrantID, ExpiresAt: now + int64(refreshTokenTTL/time.Millisecond), CreatedAt: now}
	ok, err := s.DB.OAuth.RotateRefreshToken(ctx, oldHash, access, refresh, now)
	if err != nil {
		return TokenResponse{}, err
	}
	if !ok {
		return TokenResponse{}, badRequest("invalid refresh_token")
	}
	return tokenResponse(accessRaw, refreshRaw, codes), nil
}

func (s Service) clientCredentialsToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	if client.ClientType != ClientTypeConfidential {
		return TokenResponse{}, badRequest("client_credentials requires a confidential client")
	}
	actor, err := s.DB.Permissions.ActorForClient(ctx, client.ID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindClient,
		Entrypoint:  permission.EntrypointAPI,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	codes, err := requestedOrDefaultClientScopes(req.Scope, actor)
	if err != nil {
		return TokenResponse{}, err
	}
	if len(codes) == 0 {
		return TokenResponse{}, forbidden()
	}
	raw, tokenHash, err := generateToken()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	token := model.OAuthClientAccessToken{
		TokenHash: tokenHash,
		ClientID:  client.ID,
		ExpiresAt: now + int64(accessTokenTTL/time.Millisecond),
		CreatedAt: now,
	}
	if err := s.DB.OAuth.CreateClientAccessToken(ctx, token, permissionIDsFromCodes(codes)); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: raw,
		TokenType:   "Bearer",
		ExpiresIn:   int64(accessTokenTTL / time.Second),
		Scope:       strings.Join(codes, " "),
		Permissions: codes,
	}, nil
}

func (s Service) deviceCodeToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return TokenResponse{}, err
	}
	deviceHash := util.HashRefreshToken(req.DeviceCode)
	code, permissionIDs, err := s.DB.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, deviceHash)
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	if code == nil || code.ClientID != client.ID {
		return TokenResponse{}, badRequest("invalid device_code")
	}
	if err := s.DB.OAuth.MarkDeviceCodePolled(ctx, deviceHash, now); err != nil {
		return TokenResponse{}, err
	}
	if code.ExpiresAt <= now {
		return TokenResponse{}, oauthError("expired_token")
	}
	switch code.Status {
	case "pending":
		return TokenResponse{}, oauthError("authorization_pending")
	case "denied":
		return TokenResponse{}, oauthError("access_denied")
	case "consumed":
		return TokenResponse{}, oauthError("invalid_grant")
	case "approved":
	default:
		return TokenResponse{}, oauthError("invalid_grant")
	}
	consumed, consumedPermissionIDs, err := s.DB.OAuth.ConsumeApprovedDeviceCode(ctx, deviceHash, now)
	if err != nil {
		return TokenResponse{}, err
	}
	if consumed == nil || consumed.UserID == nil || consumed.SubjectID == nil {
		return TokenResponse{}, oauthError("invalid_grant")
	}
	codes := permissionCodesFromIDs(consumedPermissionIDs)
	if len(codes) == 0 {
		codes = permissionCodesFromIDs(permissionIDs)
	}
	grantID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return TokenResponse{}, err
	}
	grant := model.OAuthGrant{
		ID:        grantID,
		UserID:    *consumed.UserID,
		SubjectID: *consumed.SubjectID,
		ClientID:  client.ID,
		Status:    StatusActive,
		CreatedAt: now,
	}
	if err := s.DB.OAuth.CreateGrant(ctx, grant, permissionIDsFromCodes(codes)); err != nil {
		return TokenResponse{}, err
	}
	return s.issueTokens(ctx, client.ID, *consumed.UserID, grantID, codes)
}

func (s Service) issueTokens(ctx context.Context, clientID, userID, grantID string, codes []string) (TokenResponse, error) {
	accessRaw, accessHash, refreshRaw, refreshHash, err := tokenPair()
	if err != nil {
		return TokenResponse{}, err
	}
	now := database.NowMS()
	access := model.OAuthToken{TokenHash: accessHash, ClientID: clientID, UserID: userID, GrantID: grantID, ExpiresAt: now + int64(accessTokenTTL/time.Millisecond), CreatedAt: now}
	refresh := model.OAuthToken{TokenHash: refreshHash, ClientID: clientID, UserID: userID, GrantID: grantID, ExpiresAt: now + int64(refreshTokenTTL/time.Millisecond), CreatedAt: now}
	if err := s.DB.OAuth.CreateTokens(ctx, access, refresh); err != nil {
		return TokenResponse{}, err
	}
	return tokenResponse(accessRaw, refreshRaw, codes), nil
}

func (s Service) validAuthorizationRequest(ctx context.Context, actor permission.Actor, req AuthorizationRequest) (model.OAuthClient, []string, error) {
	if req.ResponseType != "code" {
		return model.OAuthClient{}, nil, badRequest("response_type must be code")
	}
	client, err := s.DB.OAuth.GetClient(ctx, strings.TrimSpace(req.ClientID))
	if err != nil {
		return model.OAuthClient{}, nil, err
	}
	if client == nil || client.Status != StatusActive {
		return model.OAuthClient{}, nil, badRequest("invalid client_id")
	}
	if req.RedirectURI != client.RedirectURI {
		return model.OAuthClient{}, nil, badRequest("invalid redirect_uri")
	}
	if req.CodeChallengeMethod != "S256" || strings.TrimSpace(req.CodeChallenge) == "" {
		return model.OAuthClient{}, nil, badRequest("PKCE S256 is required")
	}
	codes, err := parseScope(req.Scope)
	if err != nil {
		return model.OAuthClient{}, nil, err
	}
	clientIDs, err := s.DB.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		return model.OAuthClient{}, nil, err
	}
	clientAllowed := idSet(clientIDs)
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		if !actor.Has(def) {
			return model.OAuthClient{}, nil, forbidden()
		}
		if !clientAllowed[int64(def.ID)] {
			return model.OAuthClient{}, nil, badRequest("scope exceeds client permission limit")
		}
	}
	return *client, codes, nil
}

func (s Service) clientFromInput(actor permission.Actor, input ClientInput) (model.OAuthClient, []int64, []string, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return model.OAuthClient{}, nil, nil, badRequest("invalid name")
	}
	redirectURI := strings.TrimSpace(input.RedirectURI)
	if !validHTTPURL(redirectURI) {
		return model.OAuthClient{}, nil, nil, badRequest("invalid redirect_uri")
	}
	websiteURL := strings.TrimSpace(input.WebsiteURL)
	if websiteURL != "" && !validHTTPURL(websiteURL) {
		return model.OAuthClient{}, nil, nil, badRequest("invalid website_url")
	}
	clientType := strings.TrimSpace(input.ClientType)
	if clientType == "" {
		clientType = ClientTypeConfidential
	}
	if clientType != ClientTypeConfidential && clientType != ClientTypePublic {
		return model.OAuthClient{}, nil, nil, badRequest("invalid client_type")
	}
	codes, err := validateCodes(input.PermissionCodes)
	if err != nil {
		return model.OAuthClient{}, nil, nil, err
	}
	ids := permissionIDsFromCodes(codes)
	for _, code := range codes {
		if !actor.Has(permission.MustDefinitionByCode(code)) {
			return model.OAuthClient{}, nil, nil, forbidden()
		}
	}
	client := model.OAuthClient{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		RedirectURI: redirectURI,
		WebsiteURL:  websiteURL,
		ClientType:  clientType,
	}
	return client, ids, codes, nil
}

func (s Service) clientForActor(ctx context.Context, actor permission.Actor, clientID, ownedCode, anyCode string) (*model.OAuthClient, error) {
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth client not found")
	}
	if actor.Has(permission.MustDefinitionByCode(anyCode)) {
		return client, nil
	}
	if client.OwnerUserID == actor.UserID && actor.Has(permission.MustDefinitionByCode(ownedCode)) {
		return client, nil
	}
	return nil, forbidden()
}

func (s Service) authenticateClient(ctx context.Context, clientID, secret string) (*model.OAuthClient, error) {
	client, err := s.DB.OAuth.GetClient(ctx, strings.TrimSpace(clientID))
	if err != nil {
		return nil, err
	}
	if client == nil || client.Status != StatusActive {
		return nil, badRequest("invalid client_id")
	}
	if client.ClientType == ClientTypeConfidential {
		if secret == "" || subtle.ConstantTimeCompare([]byte(client.SecretHash), []byte(util.HashRefreshToken(secret))) != 1 {
			return nil, badRequest("invalid client_secret")
		}
	}
	return client, nil
}

func (s Service) clientPermissionCodes(ctx context.Context, clientID string) ([]string, error) {
	ids, err := s.DB.OAuth.ClientPermissionIDs(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return permissionCodesFromIDs(ids), nil
}

func (s Service) grantPermissionCodes(ctx context.Context, grantID string) ([]string, error) {
	ids, err := s.DB.OAuth.GrantPermissionIDs(ctx, grantID)
	if err != nil {
		return nil, err
	}
	return permissionCodesFromIDs(ids), nil
}

func generateSecret() (string, string, error) {
	return generateToken()
}

func generateToken() (string, string, error) {
	raw, hash, err := util.GenerateRefreshToken()
	return raw, hash, err
}

func generateUserCode() (string, string, error) {
	raw, _, err := util.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	var compact strings.Builder
	for _, r := range strings.ToUpper(raw) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			compact.WriteRune(r)
		}
		if compact.Len() >= 8 {
			break
		}
	}
	code := compact.String()
	if len(code) < 8 {
		return "", "", badRequest("could not generate user_code")
	}
	formatted := code[:4] + "-" + code[4:]
	return formatted, util.HashRefreshToken(formatted), nil
}

func tokenPair() (string, string, string, string, error) {
	accessRaw, accessHash, err := generateToken()
	if err != nil {
		return "", "", "", "", err
	}
	refreshRaw, refreshHash, err := generateToken()
	if err != nil {
		return "", "", "", "", err
	}
	return accessRaw, accessHash, refreshRaw, refreshHash, nil
}

func tokenResponse(access, refresh string, codes []string) TokenResponse {
	return TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL / time.Second),
		RefreshToken: refresh,
		Scope:        strings.Join(codes, " "),
		Permissions:  codes,
	}
}

func parseScope(raw string) ([]string, error) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil, badRequest("scope is required")
	}
	return validateCodes(parts)
}

func validateCodes(codes []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		def, ok := permission.DefinitionByCode(code)
		if !ok || def.Scope.ID == permission.ScopeSystem {
			return nil, badRequest("invalid scope")
		}
		if !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out, nil
}

func permissionIDsFromCodes(codes []string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func permissionCodesFromIDs(ids []int64) []string {
	byID := map[int64]string{}
	for _, def := range permission.Definitions {
		byID[int64(def.ID)] = def.Code
	}
	codes := make([]string, 0, len(ids))
	for _, id := range ids {
		if code := byID[id]; code != "" {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	return codes
}

func bitSetFromPermissionIDs(ids []int64) permission.BitSet {
	byID := map[int64]int{}
	for _, def := range permission.Definitions {
		byID[int64(def.ID)] = def.BitIndex
	}
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, id := range ids {
		if bitIndex, ok := byID[id]; ok {
			bits.Set(bitIndex)
		}
	}
	return bits
}

func requestedOrDefaultClientScopes(raw string, actor permission.Actor) ([]string, error) {
	if strings.TrimSpace(raw) != "" {
		codes, err := parseScope(raw)
		if err != nil {
			return nil, err
		}
		for _, code := range codes {
			if !actor.Has(permission.MustDefinitionByCode(code)) {
				return nil, forbidden()
			}
		}
		return codes, nil
	}
	codes := make([]string, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		if actor.Has(def) {
			codes = append(codes, def.Code)
		}
	}
	sort.Strings(codes)
	return codes, nil
}

func idSet(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func validPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func authorizationRedirect(rawURL, code, state string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", badRequest("invalid redirect_uri")
	}
	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func normalizeUserCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func oauthError(detail string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Detail: detail}
}

func clientResponse(client model.OAuthClient, permissions []string, secret string) map[string]any {
	out := publicClient(client)
	out["permissions"] = permissions
	if secret != "" {
		out["client_secret"] = secret
	}
	return out
}

func publicClient(client model.OAuthClient) map[string]any {
	return map[string]any{
		"client_id":     client.ID,
		"owner_user_id": client.OwnerUserID,
		"name":          client.Name,
		"description":   client.Description,
		"redirect_uri":  client.RedirectURI,
		"website_url":   client.WebsiteURL,
		"client_type":   client.ClientType,
		"status":        client.Status,
		"created_at":    client.CreatedAt,
		"updated_at":    client.UpdatedAt,
	}
}

func grantResponse(grant model.OAuthGrant, permissions []string) map[string]any {
	return map[string]any{
		"id":          grant.ID,
		"user_id":     grant.UserID,
		"subject_id":  grant.SubjectID,
		"client_id":   grant.ClientID,
		"status":      grant.Status,
		"created_at":  grant.CreatedAt,
		"revoked_at":  grant.RevokedAt,
		"permissions": permissions,
	}
}

func permissionDetails(codes []string) []map[string]any {
	out := make([]map[string]any, 0, len(codes))
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		out = append(out, map[string]any{
			"code":                 def.Code,
			"description":          def.Description,
			"resource":             def.Resource.Code,
			"resource_description": def.Resource.Description,
			"action":               def.Action.Code,
			"action_description":   def.Action.Description,
			"scope":                def.Scope.Code,
			"scope_description":    def.Scope.Description,
		})
	}
	return out
}

func badRequest(detail string) error {
	return util.HTTPError{Status: http.StatusBadRequest, Detail: detail}
}

func forbidden() error {
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func notFound(detail string) error {
	return util.HTTPError{Status: http.StatusNotFound, Detail: detail}
}
