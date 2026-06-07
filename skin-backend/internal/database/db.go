package database

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, cfg config.Config) (*DB, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	pcfg.MaxConns = cfg.MaxConnections
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	db := &DB{Pool: pool}
	if err := db.Init(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) Init(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, InitSQL)
	return err
}

func (db *DB) ResetPublicSchema(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO public;`)
	if err != nil {
		return err
	}
	return db.Init(ctx)
}

func NowMS() int64 { return time.Now().UnixMilli() }

func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.Password, &u.IsAdmin, &u.PreferredLanguage, &u.DisplayName, &u.BannedUntil, &u.AvatarHash)
	return u, err
}

func scanProfile(row pgx.Row) (model.Profile, error) {
	var p model.Profile
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash)
	return p, err
}

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

var ErrNotFound = errors.New("not found")
