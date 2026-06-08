package app

import (
	"context"
	"net/http"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi"
	sitepkg "element-skin/backend/internal/service/site"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"
)

type App struct {
	db       *database.DB
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
	site := sitepkg.Site{DB: db, Cfg: cfg}
	ygg, err := yggpkg.New(db, cfg)
	if err != nil {
		db.Close()
		return nil, err
	}
	cleanupCtx, cancel := context.WithCancel(context.Background())
	go RunRefreshCleanupLoop(cleanupCtx, db.Tokens, time.Hour)
	return &App{
		db:       db,
		handler:  httpapi.NewRouter(cfg, db, site, ygg),
		cancelFn: cancel,
	}, nil
}

func NewWithDB(cfg config.Config, db *database.DB) (*App, error) {
	site := sitepkg.Site{DB: db, Cfg: cfg}
	ygg, err := yggpkg.New(db, cfg)
	if err != nil {
		return nil, err
	}
	return &App{db: db, handler: httpapi.NewRouter(cfg, db, site, ygg)}, nil
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
