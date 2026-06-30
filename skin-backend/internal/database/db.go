package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/database/homepage"
	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/database/notice"
	"element-skin/backend/internal/database/oauth"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/database/setting"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/database/token"
	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/database/verification"
	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool          *pgxpool.Pool
	Users         user.Store
	Profiles      profile.Store
	Textures      texture.Store
	Tokens        token.Store
	Settings      setting.Store
	Invites       invite.Store
	Fallbacks     fallback.Store
	HomepageMedia homepage.Store
	Verifications verification.Store
	Notices       notice.Store
	OAuth         oauth.Store
	Permissions   permissiondb.Store
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
	db := New(pool)
	if err := db.Init(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := db.MigrateHomepageMediaFiles(ctx, cfg.CarouselDir); err != nil {
		pool.Close()
		return nil, err
	}
	return db, nil
}

func New(pool *pgxpool.Pool) *DB {
	return &DB{
		Pool:          pool,
		Users:         user.Store{Pool: pool},
		Profiles:      profile.Store{Pool: pool},
		Textures:      texture.Store{Pool: pool},
		Tokens:        token.Store{Pool: pool},
		Settings:      setting.Store{Pool: pool},
		Invites:       invite.Store{Pool: pool},
		Fallbacks:     fallback.Store{Pool: pool},
		HomepageMedia: homepage.Store{Pool: pool},
		Verifications: verification.Store{Pool: pool},
		Notices:       notice.Store{Pool: pool},
		OAuth:         oauth.Store{Pool: pool},
		Permissions:   permissiondb.Store{Pool: pool},
	}
}

func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) Init(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, InitSQL); err != nil {
		return err
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		return err
	}
	_, err := db.Pool.Exec(ctx, `
		DROP INDEX IF EXISTS idx_users_single_super_admin;
		ALTER TABLE users DROP COLUMN IF EXISTS is_admin;
		ALTER TABLE users DROP COLUMN IF EXISTS is_super_admin;
	`)
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

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func (db *DB) MigrateHomepageMediaFiles(ctx context.Context, dir string) error {
	var count int64
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM homepage_media`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch strings.ToLower(filepath.Ext(name)) {
		case ".png", ".jpg", ".jpeg", ".webp":
			names = append(names, name)
		}
	}
	sort.Strings(names)
	now := NowMS()
	for i, name := range names {
		id := strings.TrimSuffix(name, filepath.Ext(name))
		if id == "" {
			continue
		}
		if err := db.HomepageMedia.Create(ctx, modelHomepageImage(id, name, i, now)); err != nil {
			return err
		}
	}
	return nil
}

func modelHomepageImage(id, filename string, order int, now int64) model.HomepageMedia {
	return model.HomepageMedia{
		ID:                  id,
		Type:                "image",
		Title:               filename,
		StoragePath:         filename,
		OverlayOpacityLight: 0.45,
		OverlayOpacityDark:  0.45,
		YawSpeedDPS:         4,
		SortOrder:           order,
		Enabled:             true,
		DurationMS:          6000,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}
