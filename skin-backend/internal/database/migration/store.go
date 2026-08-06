package migration

import (
	"context"
	"errors"
	"fmt"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLegacyMicrosoftSettingsChanged  = errors.New("legacy Microsoft settings changed during migration")
	ErrLegacyMicrosoftProviderConflict = errors.New("legacy Microsoft client conflicts with a non-Microsoft identity provider")
)

var legacyMicrosoftSettingKeys = []string{
	"microsoft_client_id",
	"microsoft_client_secret",
	"microsoft_redirect_uri",
}

type Store struct {
	Pool *pgxpool.Pool
}

type LegacyMicrosoftSettings struct {
	ClientID            string
	ClientSecret        string
	RedirectURI         string
	ClientIDPresent     bool
	ClientSecretPresent bool
	RedirectURIPresent  bool
}

func (s LegacyMicrosoftSettings) Present() bool {
	return s.ClientIDPresent || s.ClientSecretPresent || s.RedirectURIPresent
}

func (s LegacyMicrosoftSettings) count() int64 {
	var count int64
	for _, present := range []bool{s.ClientIDPresent, s.ClientSecretPresent, s.RedirectURIPresent} {
		if present {
			count++
		}
	}
	return count
}

type LegacyMicrosoftMigrationResult struct {
	ProviderCreated       bool
	LegacySettingsRemoved bool
}

func (s Store) ReadLegacyMicrosoftSettings(ctx context.Context) (LegacyMicrosoftSettings, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT key,value
		FROM settings
		WHERE key=ANY($1::text[])
	`, legacyMicrosoftSettingKeys)
	if err != nil {
		return LegacyMicrosoftSettings{}, err
	}
	defer rows.Close()
	return scanLegacyMicrosoftSettings(rows)
}

func (s Store) FinalizeLegacyMicrosoftMigration(
	ctx context.Context,
	expected LegacyMicrosoftSettings,
	provider *model.IdentityProvider,
) (LegacyMicrosoftMigrationResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return LegacyMicrosoftMigrationResult{}, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT key,value
		FROM settings
		WHERE key=ANY($1::text[])
		FOR UPDATE
	`, legacyMicrosoftSettingKeys)
	if err != nil {
		return LegacyMicrosoftMigrationResult{}, err
	}
	current, err := scanLegacyMicrosoftSettings(rows)
	rows.Close()
	if err != nil {
		return LegacyMicrosoftMigrationResult{}, err
	}
	if !current.Present() {
		if err := tx.Commit(ctx); err != nil {
			return LegacyMicrosoftMigrationResult{}, err
		}
		return LegacyMicrosoftMigrationResult{}, nil
	}
	if current != expected {
		return LegacyMicrosoftMigrationResult{}, ErrLegacyMicrosoftSettingsChanged
	}

	result := LegacyMicrosoftMigrationResult{}
	if provider != nil {
		created, err := createLegacyMicrosoftProvider(ctx, tx, *provider)
		if err != nil {
			return LegacyMicrosoftMigrationResult{}, err
		}
		result.ProviderCreated = created
	}

	tag, err := tx.Exec(ctx, `DELETE FROM settings WHERE key=ANY($1::text[])`, legacyMicrosoftSettingKeys)
	if err != nil {
		return LegacyMicrosoftMigrationResult{}, err
	}
	if tag.RowsAffected() != current.count() {
		return LegacyMicrosoftMigrationResult{}, fmt.Errorf(
			"delete legacy Microsoft settings: removed %d rows, expected %d",
			tag.RowsAffected(), current.count(),
		)
	}
	result.LegacySettingsRemoved = true
	if err := tx.Commit(ctx); err != nil {
		return LegacyMicrosoftMigrationResult{}, err
	}
	return result, nil
}

func createLegacyMicrosoftProvider(ctx context.Context, tx pgx.Tx, provider model.IdentityProvider) (bool, error) {
	var adapter string
	err := tx.QueryRow(ctx, `
		SELECT adapter
		FROM identity_providers
		WHERE issuer_url=$1 AND client_id=$2
		FOR UPDATE
	`, provider.IssuerURL, provider.ClientID).Scan(&adapter)
	switch {
	case err == nil:
		if adapter != "microsoft" {
			return false, ErrLegacyMicrosoftProviderConflict
		}
		return false, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return false, err
	}

	if provider.Scopes == nil {
		provider.Scopes = []string{}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO identity_providers (
			id, name, issuer_url, authorization_endpoint, token_endpoint, userinfo_endpoint,
			jwks_uri, client_id, client_secret_ciphertext, scopes, adapter, icon_url,
			enabled, login_enabled, link_enabled, registration_enabled, display_order, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, provider.ID, provider.Name, provider.IssuerURL, provider.AuthorizationEndpoint,
		provider.TokenEndpoint, provider.UserInfoEndpoint, provider.JWKSURI, provider.ClientID,
		provider.ClientSecretCiphertext, provider.Scopes, provider.Adapter, provider.IconURL,
		provider.Enabled, provider.LoginEnabled, provider.LinkEnabled, provider.RegistrationEnabled,
		provider.DisplayOrder, provider.CreatedAt, provider.UpdatedAt)
	if err != nil {
		return false, err
	}
	return true, nil
}

type rowsScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanLegacyMicrosoftSettings(rows rowsScanner) (LegacyMicrosoftSettings, error) {
	settings := LegacyMicrosoftSettings{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return LegacyMicrosoftSettings{}, err
		}
		switch key {
		case "microsoft_client_id":
			settings.ClientID = value
			settings.ClientIDPresent = true
		case "microsoft_client_secret":
			settings.ClientSecret = value
			settings.ClientSecretPresent = true
		case "microsoft_redirect_uri":
			settings.RedirectURI = value
			settings.RedirectURIPresent = true
		}
	}
	return settings, rows.Err()
}
