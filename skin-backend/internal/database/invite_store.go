package database

import (
	"context"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/model"
)

var ErrInviteExhausted = invite.ErrExhausted

func (db *DB) CreateInvite(ctx context.Context, code string, totalUses int, note string) error {
	return (invite.Store{Pool: db.Pool}).Create(ctx, code, totalUses, note)
}

func (db *DB) GetInvite(ctx context.Context, code string) (*model.Invite, error) {
	return (invite.Store{Pool: db.Pool}).Get(ctx, code)
}

func (db *DB) DeleteInvite(ctx context.Context, code string) error {
	return (invite.Store{Pool: db.Pool}).Delete(ctx, code)
}

func (db *DB) ListInvites(ctx context.Context, limit int, lastCreated *int64, lastCode string) (map[string]any, error) {
	return (invite.Store{Pool: db.Pool}).List(ctx, limit, lastCreated, lastCode)
}
