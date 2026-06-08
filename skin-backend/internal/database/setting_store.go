package database

import (
	"context"

	"element-skin/backend/internal/database/setting"
)

func (db *DB) GetSetting(ctx context.Context, key, fallback string) (string, error) {
	return (setting.Store{Pool: db.Pool}).Get(ctx, key, fallback)
}

func (db *DB) SetSetting(ctx context.Context, key string, value any) error {
	return (setting.Store{Pool: db.Pool}).Set(ctx, key, value)
}

func (db *DB) SettingInt(ctx context.Context, key string, fallback int) (int, error) {
	return (setting.Store{Pool: db.Pool}).Int(ctx, key, fallback)
}

func (db *DB) GetSettingsGroup(ctx context.Context, keys map[string]string) (map[string]any, error) {
	return (setting.Store{Pool: db.Pool}).Group(ctx, keys)
}

func (db *DB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	return (setting.Store{Pool: db.Pool}).All(ctx)
}
