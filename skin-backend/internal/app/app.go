package app

import (
	"context"
	"net/http"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/redisstore"
	probesvc "element-skin/backend/internal/service/probe"
	settingssvc "element-skin/backend/internal/service/settings"
	sitepkg "element-skin/backend/internal/service/site"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"
)

type App struct {
	db       *database.DB
	redis    redisstore.Store
	handler  http.Handler
	cancelFn context.CancelFunc
}

type refreshTokenCleaner interface {
	DeleteExpiredRefresh(ctx context.Context, cutoff int64) error
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	if err := util.ValidateJWTSecret(cfg.JWTSecret); err != nil {
		return nil, err
	}
	db, err := database.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := db.Tokens.DeleteExpiredRefresh(ctx, database.NowMS()); err != nil {
		db.Close()
		return nil, err
	}
	redis, err := redisstore.Open(ctx, cfg)
	if err != nil {
		db.Close()
		return nil, err
	}
	settings := settingssvc.Settings{DB: db, Redis: redis}
	site := sitepkg.Site{DB: db, Cfg: cfg, Redis: redis, Settings: settings}
	ygg, err := yggpkg.New(db, cfg, redis, settings)
	if err != nil {
		_ = redis.Close()
		db.Close()
		return nil, err
	}
	cleanupCtx, cancel := context.WithCancel(context.Background())
	go RunRefreshCleanupLoop(cleanupCtx, db.Tokens, time.Hour)
	go probesvc.RunLoop(cleanupCtx, db, redis, settings)
	return &App{
		db:       db,
		redis:    redis,
		handler:  httpapi.NewRouterWithRedis(cfg, db, redis, site, ygg),
		cancelFn: cancel,
	}, nil
}

func NewWithDB(cfg config.Config, db *database.DB) (*App, error) {
	redis, err := redisstore.Open(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return NewWithDBAndRedis(cfg, db, redis)
}

func NewWithDBAndRedis(cfg config.Config, db *database.DB, redis redisstore.Store) (*App, error) {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	site := sitepkg.Site{DB: db, Cfg: cfg, Redis: redis, Settings: settings}
	ygg, err := yggpkg.New(db, cfg, redis, settings)
	if err != nil {
		_ = redis.Close()
		return nil, err
	}
	return &App{db: db, redis: redis, handler: httpapi.NewRouterWithRedis(cfg, db, redis, site, ygg)}, nil
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) Close() {
	if a.cancelFn != nil {
		a.cancelFn()
	}
	if a.db != nil {
		a.db.Close()
	}
	if a.redis != nil {
		_ = a.redis.Close()
	}
}

func RunRefreshCleanupLoop(ctx context.Context, cleaner refreshTokenCleaner, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = cleaner.DeleteExpiredRefresh(ctx, database.NowMS())
		}
	}
}
