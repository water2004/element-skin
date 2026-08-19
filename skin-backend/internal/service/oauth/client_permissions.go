package oauth

import (
	"context"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

func (s Service) ClientPermissions(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("permission.read.any")); err != nil {
		return nil, forbidden()
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth_client", "resolve", "not_found")
	}
	subjectID := permissiondb.SubjectIDForClient(client.ID)
	effective, err := s.DB.Permissions.EffectivePermissionsForClient(ctx, client.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		return nil, err
	}
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	clientScopes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	overrideItems := make([]map[string]any, 0, len(overrides))
	for _, item := range overrides {
		overrideItems = append(overrideItems, map[string]any{
			"permission_code": item.PermissionCode,
			"effect":          item.Effect,
			"created_at":      item.CreatedAt,
		})
	}
	return map[string]any{
		"subject_id":             subjectID,
		"client":                 publicClient(*client),
		"effective_permissions":  permissionCodesFromBitSet(effective),
		"overrides":              overrideItems,
		"client_allowed_scopes":  clientScopes,
		"session_allowed_scopes": clientCredentialsPolicyCodes(),
	}, nil
}

func (s Service) SetClientPermissionOverride(ctx context.Context, actor permission.Actor, clientID, code, effect string) error {
	if effect != "allow" && effect != "deny" {
		return badRequest("permission_override", "configure", "invalid")
	}
	if effect == "allow" {
		if err := actor.Require(permission.MustDefinitionByCode("permission.grant.any")); err != nil {
			return forbidden()
		}
	} else {
		if err := actor.Require(permission.MustDefinitionByCode("permission.revoke.any")); err != nil {
			return forbidden()
		}
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return notFound("oauth_client", "resolve", "not_found")
	}
	def, ok := permission.DefinitionByCode(code)
	if !ok || def.Scope.ID == permission.ScopeSystem {
		return badRequest("permission", "validate", "invalid")
	}
	subjectID := permissiondb.SubjectIDForClient(client.ID)
	if err := s.DB.Permissions.InvalidateSubjectCache(ctx, subjectID); err != nil {
		return err
	}
	if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID); err != nil {
		return err
	}
	if err := s.DB.Permissions.SetPermissionOverrideForSubject(ctx, subjectID, def, effect, actor.SubjectID); err != nil {
		return err
	}
	reportPostCommitError("invalidate client permission override cache race", s.DB.Permissions.InvalidateSubjectCache(ctx, subjectID))
	reportPostCommitError("remove client permission override access tokens", s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID))
	return nil
}

func (s Service) ClearClientPermissionOverride(ctx context.Context, actor permission.Actor, clientID, code string) error {
	if err := actor.Require(permission.MustDefinitionByCode("permission.revoke.any")); err != nil {
		return forbidden()
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return notFound("oauth_client", "resolve", "not_found")
	}
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return badRequest("permission", "validate", "invalid")
	}
	subjectID := permissiondb.SubjectIDForClient(client.ID)
	if err := s.DB.Permissions.InvalidateSubjectCache(ctx, subjectID); err != nil {
		return err
	}
	if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID); err != nil {
		return err
	}
	ok, err = s.DB.Permissions.ClearPermissionOverrideForSubject(ctx, subjectID, def)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("permission_override", "resolve", "not_found")
	}
	reportPostCommitError("invalidate cleared client permission cache race", s.DB.Permissions.InvalidateSubjectCache(ctx, subjectID))
	reportPostCommitError("remove cleared client permission access tokens", s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID))
	return nil
}

func reviewedClientPermissionIDs(codes []string) ([]int64, error) {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		def, ok := permission.DefinitionByCode(code)
		if !ok || def.Scope.ID == permission.ScopeSystem {
			return nil, badRequest("permission", "validate", "invalid")
		}
		if !isAppOnlyPermission(def) {
			continue
		}
		ids = append(ids, int64(def.ID))
	}
	return ids, nil
}

func isAppOnlyPermission(def permission.Definition) bool {
	return def.Scope.ID == permission.ScopeAny || def.Scope.ID == permission.ScopePublic || def.Scope.ID == permission.ScopeServer
}
