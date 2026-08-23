package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	identitysvc "element-skin/backend/internal/service/identity"
	mailsvc "element-skin/backend/internal/service/mail"
	noticesvc "element-skin/backend/internal/service/notice"
	oauthsvc "element-skin/backend/internal/service/oauth"
	probesvc "element-skin/backend/internal/service/probe"
	settingssvc "element-skin/backend/internal/service/settings"
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

type noticeCleaner interface {
	DeleteExpired(ctx context.Context, actor permission.Actor, cutoff int64) error
}

type oauthGrantCleaner interface {
	CleanupGrants(ctx context.Context, actor permission.Actor, now int64) (oauthsvc.GrantCleanupResult, error)
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	if err := util.ValidateJWTSecret(cfg.JWTSecret); err != nil {
		return nil, err
	}
	db, err := database.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	identityService := identitysvc.Service{DB: db, Config: cfg}
	identityMigration, err := identityService.MigrateLegacyMicrosoftProvider(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	if identityMigration.ProviderCreated {
		log.Printf("migrated legacy Microsoft configuration to an OIDC identity provider")
	} else if identityMigration.LegacySettingsRemoved {
		log.Printf("removed unused legacy Microsoft configuration")
	}
	if err := db.Tokens.DeleteExpiredRefresh(ctx, database.NowMS()); err != nil {
		db.Close()
		return nil, err
	}
	noticeService := noticesvc.Service{DB: db}
	if err := noticeService.DeleteExpired(ctx, permission.SystemMaintenanceActor(), database.NowMS()); err != nil {
		db.Close()
		return nil, err
	}
	oauthCleaner := oauthsvc.Service{DB: db}
	if _, err := oauthCleaner.CleanupGrants(ctx, permission.SystemMaintenanceActor(), database.NowMS()); err != nil {
		db.Close()
		return nil, err
	}
	redis, err := redisstore.Open(ctx, cfg)
	if err != nil {
		db.Close()
		return nil, err
	}
	if db != nil {
		db.Permissions.Cache = &permissiondb.RedisPermCache{Store: redis}
	}
	settings := settingssvc.Settings{DB: db, Redis: redis}
	ygg, err := yggpkg.New(db, cfg, redis, settings)
	if err != nil {
		_ = redis.Close()
		db.Close()
		return nil, err
	}
	oidcSigner, err := oauthsvc.NewOIDCSigner(cfg)
	if err != nil {
		_ = redis.Close()
		db.Close()
		return nil, err
	}
	cleanupCtx, cancel := context.WithCancel(context.Background())
	probeService := probesvc.New(db, redis)
	StartScheduler(cleanupCtx,
		refreshCleanupTask(db.Tokens, time.Hour),
		noticeCleanupTask(noticeService, time.Hour),
		oauthGrantCleanupTask(oauthCleaner, time.Hour),
		probeStatusTask(probeService, settings),
	)
	return &App{
		db:       db,
		redis:    redis,
		handler:  httpapi.NewRouterWithOIDCSigner(cfg, db, redis, ygg, oidcSigner),
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

func NewWithDBAndRedis(cfg config.Config, db *database.DB, redis redisstore.Store, senders ...mailsvc.Sender) (*App, error) {
	if db != nil {
		db.Permissions.Cache = &permissiondb.RedisPermCache{Store: redis}
	}
	settings := settingssvc.Settings{DB: db, Redis: redis}
	ygg, err := yggpkg.New(db, cfg, redis, settings)
	if err != nil {
		_ = redis.Close()
		return nil, err
	}
	oidcSigner, err := oauthsvc.NewOIDCSigner(cfg)
	if err != nil {
		_ = redis.Close()
		return nil, err
	}
	return &App{db: db, redis: redis, handler: httpapi.NewRouterWithOIDCSigner(cfg, db, redis, ygg, oidcSigner, senders...)}, nil
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

func refreshCleanupTask(cleaner refreshTokenCleaner, interval time.Duration) ScheduledTask {
	return ScheduledTask{
		Name:     "refresh_token_cleanup",
		Interval: fixedInterval(interval),
		Run: func(ctx context.Context) error {
			return cleaner.DeleteExpiredRefresh(ctx, database.NowMS())
		},
	}
}

func noticeCleanupTask(cleaner noticeCleaner, interval time.Duration) ScheduledTask {
	return ScheduledTask{
		Name:     "notice_cleanup",
		Interval: fixedInterval(interval),
		Run: func(ctx context.Context) error {
			return cleaner.DeleteExpired(ctx, permission.SystemMaintenanceActor(), database.NowMS())
		},
	}
}

func oauthGrantCleanupTask(cleaner oauthGrantCleaner, interval time.Duration) ScheduledTask {
	return ScheduledTask{
		Name:     "oauth_grant_cleanup",
		Interval: fixedInterval(interval),
		Run: func(ctx context.Context) error {
			_, err := cleaner.CleanupGrants(ctx, permission.SystemMaintenanceActor(), database.NowMS())
			return err
		},
	}
}

func probeStatusTask(service *probesvc.Service, reader probesvc.IntervalReader) ScheduledTask {
	return ScheduledTask{
		Name:           "probe_status",
		RunImmediately: true,
		Interval: func(ctx context.Context) time.Duration {
			return probesvc.ReadInterval(ctx, reader)
		},
		Run: func(ctx context.Context) error {
			if service == nil {
				return nil
			}
			return service.Run(ctx)
		},
	}
}
