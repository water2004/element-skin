package setting

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) Get(ctx context.Context, key, fallback string) (string, error) {
	var v string
	err := s.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, nil
	}
	return v, err
}

func (s Store) Set(ctx context.Context, key string, value any) error {
	text := fmt.Sprint(value)
	if b, ok := value.(bool); ok {
		if b {
			text = "true"
		} else {
			text = "false"
		}
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO settings (key,value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, text)
	return err
}

func (s Store) Int(ctx context.Context, key string, fallback int) (int, error) {
	v, err := s.Get(ctx, key, strconv.Itoa(fallback))
	if err != nil {
		return fallback, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback, nil
	}
	return n, nil
}

func (s Store) Group(ctx context.Context, keys map[string]string) (map[string]any, error) {
	out := make(map[string]any, len(keys))
	for k, fallback := range keys {
		v, err := s.Get(ctx, k, fallback)
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

func (s Store) All(ctx context.Context) (map[string]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT key,value FROM settings`)
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
