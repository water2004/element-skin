package database

import (
	"context"
	"fmt"
	"strconv"
)

func (db *DB) GetSetting(ctx context.Context, key, fallback string) (string, error) {
	var v string
	err := db.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	if IsNoRows(err) {
		return fallback, nil
	}
	return v, err
}

func (db *DB) SetSetting(ctx context.Context, key string, value any) error {
	s := fmt.Sprint(value)
	if b, ok := value.(bool); ok {
		if b {
			s = "true"
		} else {
			s = "false"
		}
	}
	_, err := db.Pool.Exec(ctx, `INSERT INTO settings (key,value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, s)
	return err
}

func (db *DB) SettingInt(ctx context.Context, key string, fallback int) (int, error) {
	v, err := db.GetSetting(ctx, key, strconv.Itoa(fallback))
	if err != nil {
		return fallback, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback, nil
	}
	return n, nil
}

func (db *DB) GetSettingsGroup(ctx context.Context, keys map[string]string) (map[string]any, error) {
	out := make(map[string]any, len(keys))
	for k, fallback := range keys {
		v, err := db.GetSetting(ctx, k, fallback)
		if err != nil {
			return nil, err
		}
		if v == "true" {
			out[k] = true
		} else if v == "false" {
			out[k] = false
		} else {
			out[k] = v
		}
	}
	return out, nil
}

func (db *DB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.Pool.Query(ctx, `SELECT key,value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
