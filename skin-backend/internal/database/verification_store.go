package database

import (
	"context"

	"element-skin/backend/internal/database/verification"
)

func (db *DB) CreateVerificationCode(ctx context.Context, email, code, typ string, ttlSeconds int) error {
	return (verification.Store{Pool: db.Pool}).CreateCode(ctx, email, code, typ, ttlSeconds)
}

func (db *DB) GetVerificationCode(ctx context.Context, email, typ string) (string, int64, bool, error) {
	return (verification.Store{Pool: db.Pool}).GetCode(ctx, email, typ)
}

func (db *DB) DeleteVerificationCode(ctx context.Context, email, typ string) error {
	return (verification.Store{Pool: db.Pool}).DeleteCode(ctx, email, typ)
}
