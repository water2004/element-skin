package app

import (
	"context"
	"net/http"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

type App struct {
	db       *database.DB
	handler  http.Handler
	cancelFn context.CancelFunc
}

type refreshTokenCleaner interface {
	DeleteExpiredRefreshTokens(ctx context.Context, cutoff int64) error
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	if err := util.ValidateJWTSecret(cfg.JWTSecret); err != nil {
		return nil, err
	}
	db, err := database.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := db.DeleteExpiredRefreshTokens(ctx, database.NowMS()); err != nil {
		db.Close()
		return nil, err
	}
	site := service.Site{DB: db, Cfg: cfg}
	ygg := service.Yggdrasil{DB: db, Cfg: cfg}
	cleanupCtx, cancel := context.WithCancel(context.Background())
	go RunRefreshCleanupLoop(cleanupCtx, db, time.Hour)
	return &App{
		db:       db,
		handler:  httpapi.NewRouter(cfg, db, site, ygg),
		cancelFn: cancel,
	}, nil
}

func NewWithDB(cfg config.Config, db *database.DB) *App {
	site := service.Site{DB: db, Cfg: cfg}
	ygg := service.Yggdrasil{DB: db, Cfg: cfg}
	return &App{db: db, handler: httpapi.NewRouter(cfg, db, site, ygg)}
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
			_ = cleaner.DeleteExpiredRefreshTokens(ctx, database.NowMS())
		}
	}
}
