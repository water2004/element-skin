package identity

import (
	"context"

	"element-skin/backend/internal/model"
)

type IdentityView struct {
	Identity   model.ExternalIdentity
	Provider   model.IdentityProvider
	Credential model.ExternalIdentityCredential
}

func (s Store) ListIdentityViewsByUser(ctx context.Context, userID string) ([]IdentityView, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT
			ei.id,ei.user_id,ei.provider_id,ei.subject,ei.label,ei.email,ei.email_verified,
			ei.display_name,ei.avatar_url,ei.created_at,ei.updated_at,ei.last_login_at,
			ip.id,ip.name,ip.issuer_url,ip.authorization_endpoint,ip.token_endpoint,
			ip.userinfo_endpoint,ip.jwks_uri,ip.client_id,ip.client_secret_ciphertext,ip.scopes,
			ip.adapter,ip.icon_url,ip.enabled,ip.login_enabled,ip.link_enabled,ip.display_order,
			ip.created_at,ip.updated_at,
			credential.identity_id,credential.refresh_token_ciphertext,credential.granted_scopes,
			credential.authorization_status,credential.last_refresh_at,
			credential.last_refresh_error_at,credential.updated_at
		FROM external_identities ei
		JOIN identity_providers ip ON ip.id=ei.provider_id
		JOIN external_identity_credentials credential ON credential.identity_id=ei.id
		WHERE ei.user_id=$1
		ORDER BY ei.created_at ASC,ei.id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]IdentityView, 0)
	for rows.Next() {
		var item IdentityView
		if err := rows.Scan(
			&item.Identity.ID, &item.Identity.UserID, &item.Identity.ProviderID,
			&item.Identity.Subject, &item.Identity.Label, &item.Identity.Email,
			&item.Identity.EmailVerified, &item.Identity.DisplayName, &item.Identity.AvatarURL,
			&item.Identity.CreatedAt, &item.Identity.UpdatedAt, &item.Identity.LastLoginAt,
			&item.Provider.ID, &item.Provider.Name, &item.Provider.IssuerURL,
			&item.Provider.AuthorizationEndpoint, &item.Provider.TokenEndpoint,
			&item.Provider.UserInfoEndpoint, &item.Provider.JWKSURI, &item.Provider.ClientID,
			&item.Provider.ClientSecretCiphertext, &item.Provider.Scopes, &item.Provider.Adapter,
			&item.Provider.IconURL, &item.Provider.Enabled, &item.Provider.LoginEnabled,
			&item.Provider.LinkEnabled, &item.Provider.DisplayOrder, &item.Provider.CreatedAt,
			&item.Provider.UpdatedAt,
			&item.Credential.IdentityID, &item.Credential.RefreshTokenCiphertext,
			&item.Credential.GrantedScopes, &item.Credential.AuthorizationStatus,
			&item.Credential.LastRefreshAt, &item.Credential.LastRefreshErrorAt,
			&item.Credential.UpdatedAt,
		); err != nil {
			return nil, err
		}
		normalizeCredential(&item.Credential)
		items = append(items, item)
	}
	return items, rows.Err()
}
