package oauth

import (
	"context"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

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

func normalizeUserCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}
