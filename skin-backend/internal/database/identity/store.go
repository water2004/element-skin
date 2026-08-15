package identity

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

type rowScanner interface {
	Scan(dest ...any) error
}

func normalizeCredential(item *model.ExternalIdentityCredential) {
	if item.GrantedScopes == nil {
		item.GrantedScopes = []string{}
	}
	if item.AuthorizationStatus == "" {
		item.AuthorizationStatus = model.ExternalIdentityAuthorizationActive
	}
}

func scanProvider(row rowScanner) (*model.IdentityProvider, error) {
	var item model.IdentityProvider
	err := row.Scan(
		&item.ID,
		&item.Name,
		&item.IssuerURL,
		&item.AuthorizationEndpoint,
		&item.TokenEndpoint,
		&item.UserInfoEndpoint,
		&item.JWKSURI,
		&item.ClientID,
		&item.ClientSecretCiphertext,
		&item.Scopes,
		&item.Adapter,
		&item.IconURL,
		&item.Enabled,
		&item.LoginEnabled,
		&item.LinkEnabled,
		&item.DisplayOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanIdentity(row rowScanner) (*model.ExternalIdentity, error) {
	var item model.ExternalIdentity
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.ProviderID,
		&item.Subject,
		&item.Label,
		&item.Email,
		&item.EmailVerified,
		&item.DisplayName,
		&item.AvatarURL,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

const providerColumns = `
	id, name, issuer_url, authorization_endpoint, token_endpoint, userinfo_endpoint,
	jwks_uri, client_id, client_secret_ciphertext, scopes, adapter, icon_url,
	enabled, login_enabled, link_enabled, display_order, created_at, updated_at
`

const identityColumns = `
	id, user_id, provider_id, subject, label, email, email_verified, display_name,
	avatar_url, created_at, updated_at, last_login_at
`

func (s Store) CreateProvider(ctx context.Context, item model.IdentityProvider) error {
	if item.Scopes == nil {
		item.Scopes = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO identity_providers (
			id, name, issuer_url, authorization_endpoint, token_endpoint, userinfo_endpoint,
			jwks_uri, client_id, client_secret_ciphertext, scopes, adapter, icon_url,
			enabled, login_enabled, link_enabled, display_order, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, item.ID, item.Name, item.IssuerURL, item.AuthorizationEndpoint, item.TokenEndpoint,
		item.UserInfoEndpoint, item.JWKSURI, item.ClientID, item.ClientSecretCiphertext,
		item.Scopes, item.Adapter, item.IconURL, item.Enabled, item.LoginEnabled,
		item.LinkEnabled, item.DisplayOrder, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s Store) UpdateProvider(ctx context.Context, item model.IdentityProvider) (bool, error) {
	if item.Scopes == nil {
		item.Scopes = []string{}
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE identity_providers
		SET name=$2, issuer_url=$3, authorization_endpoint=$4, token_endpoint=$5,
			userinfo_endpoint=$6, jwks_uri=$7, client_id=$8, client_secret_ciphertext=$9,
			scopes=$10, adapter=$11, icon_url=$12, enabled=$13, login_enabled=$14,
			link_enabled=$15, display_order=$16, updated_at=$17
		WHERE id=$1
	`, item.ID, item.Name, item.IssuerURL, item.AuthorizationEndpoint, item.TokenEndpoint,
		item.UserInfoEndpoint, item.JWKSURI, item.ClientID, item.ClientSecretCiphertext,
		item.Scopes, item.Adapter, item.IconURL, item.Enabled, item.LoginEnabled,
		item.LinkEnabled, item.DisplayOrder, item.UpdatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) GetProvider(ctx context.Context, id string) (*model.IdentityProvider, error) {
	item, err := scanProvider(s.Pool.QueryRow(ctx, `SELECT `+providerColumns+` FROM identity_providers WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s Store) GetProviderByIssuerClient(ctx context.Context, issuerURL, clientID string) (*model.IdentityProvider, error) {
	item, err := scanProvider(s.Pool.QueryRow(ctx, `
		SELECT `+providerColumns+`
		FROM identity_providers
		WHERE issuer_url=$1 AND client_id=$2
	`, issuerURL, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s Store) ListProviders(ctx context.Context, publicOnly bool) ([]model.IdentityProvider, error) {
	query := `SELECT ` + providerColumns + ` FROM identity_providers`
	if publicOnly {
		query += ` WHERE enabled=TRUE AND (login_enabled=TRUE OR link_enabled=TRUE)`
	}
	query += ` ORDER BY display_order ASC, created_at ASC, id ASC`
	rows, err := s.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.IdentityProvider, 0)
	for rows.Next() {
		item, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s Store) ListIdentityIDsByProvider(ctx context.Context, providerID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id
		FROM external_identities
		WHERE provider_id=$1
		ORDER BY id
	`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	identityIDs := make([]string, 0)
	for rows.Next() {
		var identityID string
		if err := rows.Scan(&identityID); err != nil {
			return nil, err
		}
		identityIDs = append(identityIDs, identityID)
	}
	return identityIDs, rows.Err()
}

func (s Store) DeleteProvider(ctx context.Context, id string) ([]string, bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM identity_providers WHERE id=$1 FOR UPDATE
	`, id).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM external_identities
		WHERE provider_id=$1
		ORDER BY id
		FOR UPDATE
	`, id)
	if err != nil {
		return nil, false, err
	}
	identityIDs := make([]string, 0)
	for rows.Next() {
		var identityID string
		if err := rows.Scan(&identityID); err != nil {
			rows.Close()
			return nil, false, err
		}
		identityIDs = append(identityIDs, identityID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()

	if _, err := tx.Exec(ctx, `
		DELETE FROM official_profile_bindings binding
		USING external_identities identity
		WHERE binding.identity_id=identity.id AND identity.provider_id=$1
	`, id); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM external_identities WHERE provider_id=$1`, id); err != nil {
		return nil, false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM identity_providers WHERE id=$1`, id)
	if err != nil {
		return nil, false, err
	}
	if tag.RowsAffected() != 1 {
		return nil, false, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return identityIDs, true, nil
}

func (s Store) CreateIdentity(ctx context.Context, item model.ExternalIdentity, credential model.ExternalIdentityCredential) error {
	normalizeCredential(&credential)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO external_identities (
			id, user_id, provider_id, subject, label, email, email_verified,
			display_name, avatar_url, created_at, updated_at, last_login_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, item.ID, item.UserID, item.ProviderID, item.Subject, item.Label, item.Email,
		item.EmailVerified, item.DisplayName, item.AvatarURL, item.CreatedAt, item.UpdatedAt,
		item.LastLoginAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO external_identity_credentials
			(identity_id, refresh_token_ciphertext, granted_scopes, authorization_status,
			 last_refresh_at, last_refresh_error_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, item.ID, credential.RefreshTokenCiphertext, credential.GrantedScopes,
		credential.AuthorizationStatus, credential.LastRefreshAt, credential.LastRefreshErrorAt,
		credential.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) GetIdentity(ctx context.Context, id string) (*model.ExternalIdentity, error) {
	item, err := scanIdentity(s.Pool.QueryRow(ctx, `SELECT `+identityColumns+` FROM external_identities WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s Store) GetByProviderSubject(ctx context.Context, providerID, subject string) (*model.ExternalIdentity, error) {
	item, err := scanIdentity(s.Pool.QueryRow(ctx, `
		SELECT `+identityColumns+`
		FROM external_identities
		WHERE provider_id=$1 AND subject=$2
	`, providerID, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s Store) ListIdentitiesByUser(ctx context.Context, userID string) ([]model.ExternalIdentity, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+identityColumns+`
		FROM external_identities
		WHERE user_id=$1
		ORDER BY created_at ASC, id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.ExternalIdentity, 0)
	for rows.Next() {
		item, err := scanIdentity(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s Store) UpdateIdentityClaims(ctx context.Context, item model.ExternalIdentity) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE external_identities
		SET email=$3, email_verified=$4, display_name=$5, avatar_url=$6,
			updated_at=$7, last_login_at=$8
		WHERE id=$1 AND user_id=$2
	`, item.ID, item.UserID, item.Email, item.EmailVerified, item.DisplayName,
		item.AvatarURL, item.UpdatedAt, item.LastLoginAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) UpdateIdentityAuthorization(ctx context.Context, item model.ExternalIdentity, credential model.ExternalIdentityCredential) (bool, error) {
	normalizeCredential(&credential)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE external_identities
		SET email=$3, email_verified=$4, display_name=$5, avatar_url=$6,
			updated_at=$7, last_login_at=$8
		WHERE id=$1 AND user_id=$2
	`, item.ID, item.UserID, item.Email, item.EmailVerified, item.DisplayName,
		item.AvatarURL, item.UpdatedAt, item.LastLoginAt)
	if err != nil || tag.RowsAffected() != 1 {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE external_identity_credentials
		SET refresh_token_ciphertext=CASE WHEN $2='' THEN refresh_token_ciphertext ELSE $2 END,
			granted_scopes=$3, authorization_status=$4, last_refresh_at=$5,
			last_refresh_error_at=$6, updated_at=$7
		WHERE identity_id=$1
	`, item.ID, credential.RefreshTokenCiphertext, credential.GrantedScopes,
		credential.AuthorizationStatus, credential.LastRefreshAt, credential.LastRefreshErrorAt,
		credential.UpdatedAt); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) UpdateIdentityLabel(ctx context.Context, id, userID, label string, updatedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE external_identities SET label=$3, updated_at=$4 WHERE id=$1 AND user_id=$2
	`, id, userID, label, updatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) DeleteIdentity(ctx context.Context, id, userID string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM external_identities WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) GetCredential(ctx context.Context, identityID string) (*model.ExternalIdentityCredential, error) {
	var item model.ExternalIdentityCredential
	err := s.Pool.QueryRow(ctx, `
		SELECT identity_id, refresh_token_ciphertext, granted_scopes, authorization_status,
			last_refresh_at, last_refresh_error_at, updated_at
		FROM external_identity_credentials
		WHERE identity_id=$1
	`, identityID).Scan(&item.IdentityID, &item.RefreshTokenCiphertext, &item.GrantedScopes,
		&item.AuthorizationStatus, &item.LastRefreshAt, &item.LastRefreshErrorAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &item, err
}

func (s Store) UpdateCredential(ctx context.Context, item model.ExternalIdentityCredential) error {
	normalizeCredential(&item)
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO external_identity_credentials
			(identity_id, refresh_token_ciphertext, granted_scopes, authorization_status,
			 last_refresh_at, last_refresh_error_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (identity_id) DO UPDATE
		SET refresh_token_ciphertext=EXCLUDED.refresh_token_ciphertext,
			granted_scopes=EXCLUDED.granted_scopes,
			authorization_status=EXCLUDED.authorization_status,
			last_refresh_at=EXCLUDED.last_refresh_at,
			last_refresh_error_at=EXCLUDED.last_refresh_error_at,
			updated_at=EXCLUDED.updated_at
	`, item.IdentityID, item.RefreshTokenCiphertext, item.GrantedScopes,
		item.AuthorizationStatus, item.LastRefreshAt, item.LastRefreshErrorAt, item.UpdatedAt)
	return err
}

func (s Store) MarkCredentialRefreshRejected(ctx context.Context, identityID string, failedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE external_identity_credentials
		SET authorization_status='reauthorization_required', last_refresh_error_at=$2, updated_at=$2
		WHERE identity_id=$1
	`, identityID, failedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) MarkCredentialRefreshFailed(ctx context.Context, identityID string, failedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE external_identity_credentials
		SET last_refresh_error_at=$2, updated_at=$2
		WHERE identity_id=$1
	`, identityID, failedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
