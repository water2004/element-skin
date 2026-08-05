package oauth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s Store) GetPairwiseSubject(ctx context.Context, clientID, userID string) (string, error) {
	var subject string
	err := s.Pool.QueryRow(ctx, `
		SELECT subject FROM oidc_pairwise_subjects WHERE client_id=$1 AND user_id=$2
	`, clientID, userID).Scan(&subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return subject, err
}

func (s Store) CreatePairwiseSubject(ctx context.Context, clientID, userID, subject string, createdAt int64) (string, error) {
	var stored string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO oidc_pairwise_subjects (client_id,user_id,subject,created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (client_id,user_id) DO UPDATE SET client_id=EXCLUDED.client_id
		RETURNING subject
	`, clientID, userID, subject, createdAt).Scan(&stored)
	return stored, err
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
