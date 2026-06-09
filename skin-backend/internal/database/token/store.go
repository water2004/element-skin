package token

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) AddRefresh(ctx context.Context, hash, userID string, expiresAt, createdAt int64) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO site_refresh_tokens (token_hash,user_id,expires_at,created_at) VALUES ($1,$2,$3,$4)`, hash, userID, expiresAt, createdAt)
	return err
}

func (s Store) ConsumeRefresh(ctx context.Context, hash string) (map[string]any, error) {
	var tokenHash, userID string
	var expiresAt, createdAt int64
	err := s.Pool.QueryRow(ctx, `DELETE FROM site_refresh_tokens WHERE token_hash=$1 RETURNING token_hash,user_id,expires_at,created_at`, hash).
		Scan(&tokenHash, &userID, &expiresAt, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"token_hash": tokenHash, "user_id": userID, "expires_at": expiresAt, "created_at": createdAt}, nil
}

func (s Store) DeleteRefresh(ctx context.Context, hash string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE token_hash=$1`, hash)
	return err
}

func (s Store) DeleteRefreshByUser(ctx context.Context, userID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, userID)
	return err
}

func (s Store) DeleteExpiredRefresh(ctx context.Context, cutoff int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE expires_at < $1`, cutoff)
	return err
}

func (s Store) GetRefresh(ctx context.Context, hash string) (map[string]any, error) {
	var tokenHash, userID string
	var expiresAt, createdAt int64
	err := s.Pool.QueryRow(ctx, `SELECT token_hash,user_id,expires_at,created_at FROM site_refresh_tokens WHERE token_hash=$1`, hash).
		Scan(&tokenHash, &userID, &expiresAt, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"token_hash": tokenHash, "user_id": userID, "expires_at": expiresAt, "created_at": createdAt}, nil
}
