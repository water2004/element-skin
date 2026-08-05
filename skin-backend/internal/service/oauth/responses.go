package oauth

import "element-skin/backend/internal/model"

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

func adminClientSummary(client model.OAuthClient) map[string]any {
	return map[string]any{
		"client_id":     client.ID,
		"owner_user_id": client.OwnerUserID,
		"name":          client.Name,
		"description":   client.Description,
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
		"oidc_scopes": append([]string{}, grant.OIDCScopes...),
	}
}
