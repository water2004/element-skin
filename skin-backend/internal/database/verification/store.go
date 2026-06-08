package verification

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) CreateCode(ctx context.Context, email, code, typ string, ttlSeconds int) error {
	now := time.Now().UnixMilli()
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO verification_codes (email,code,type,created_at,expires_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (email,type) DO UPDATE
		SET code=EXCLUDED.code, created_at=EXCLUDED.created_at, expires_at=EXCLUDED.expires_at
	`, email, code, typ, now, now+int64(ttlSeconds)*1000)
	return err
}

func (s Store) GetCode(ctx context.Context, email, typ string) (string, int64, bool, error) {
	var code string
	var expiresAt int64
	err := s.Pool.QueryRow(ctx, `SELECT code,expires_at FROM verification_codes WHERE email=$1 AND type=$2`, email, typ).Scan(&code, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	return code, expiresAt, true, nil
}

func (s Store) DeleteCode(ctx context.Context, email, typ string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM verification_codes WHERE email=$1 AND type=$2`, email, typ)
	return err
}
