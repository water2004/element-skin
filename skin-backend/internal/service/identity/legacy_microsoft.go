package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"element-skin/backend/internal/database"
	dbmigration "element-skin/backend/internal/database/migration"
	"element-skin/backend/internal/util"
)

const MicrosoftConsumerIssuer = "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"

var legacyMicrosoftScopes = []string{
	"openid",
	"profile",
	"email",
	"XboxLive.signin",
	"offline_access",
}

func (s Service) MigrateLegacyMicrosoftProvider(
	ctx context.Context,
) (dbmigration.LegacyMicrosoftMigrationResult, error) {
	legacy, err := s.DB.Migrations.ReadLegacyMicrosoftSettings(ctx)
	if err != nil {
		return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
			"read legacy Microsoft configuration: %w", err,
		)
	}
	if !legacy.Present() {
		return dbmigration.LegacyMicrosoftMigrationResult{}, nil
	}

	clientID := strings.TrimSpace(legacy.ClientID)
	clientSecret := strings.TrimSpace(legacy.ClientSecret)
	if clientID == "" && clientSecret == "" {
		result, err := s.DB.Migrations.FinalizeLegacyMicrosoftMigration(ctx, legacy, nil)
		if err != nil {
			return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
				"remove unused legacy Microsoft configuration: %w", err,
			)
		}
		return result, nil
	}
	if clientID == "" || clientSecret == "" {
		return dbmigration.LegacyMicrosoftMigrationResult{}, errors.New(
			"migrate legacy Microsoft configuration: client_id and client_secret must both be configured",
		)
	}
	existing, err := s.DB.Identities.GetProviderByIssuerClient(ctx, MicrosoftConsumerIssuer, clientID)
	if err != nil {
		return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
			"migrate legacy Microsoft configuration: %w", err,
		)
	}
	if existing != nil {
		result, err := s.DB.Migrations.FinalizeLegacyMicrosoftMigration(ctx, legacy, existing)
		if err != nil {
			return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
				"migrate legacy Microsoft configuration: %w", err,
			)
		}
		return result, nil
	}

	item, err := s.providerFromInput(ctx, ProviderInput{
		Name:                "Microsoft",
		IssuerURL:           MicrosoftConsumerIssuer,
		ClientID:            clientID,
		ClientSecret:        &clientSecret,
		Scopes:              legacyMicrosoftScopes,
		Adapter:             AdapterMicrosoft,
		Enabled:             true,
		LoginEnabled:        false,
		LinkEnabled:         true,
		RegistrationEnabled: false,
		DisplayOrder:        0,
	}, nil)
	if err != nil {
		return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
			"migrate legacy Microsoft configuration: %w", err,
		)
	}
	item.ID, err = util.GenerateUUIDNoDash()
	if err != nil {
		return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
			"migrate legacy Microsoft configuration: %w", err,
		)
	}
	item.CreatedAt = database.NowMS()
	item.UpdatedAt = item.CreatedAt

	result, err := s.DB.Migrations.FinalizeLegacyMicrosoftMigration(ctx, legacy, &item)
	if err != nil {
		return dbmigration.LegacyMicrosoftMigrationResult{}, fmt.Errorf(
			"migrate legacy Microsoft configuration: %w", err,
		)
	}
	return result, nil
}
