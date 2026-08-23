package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/redisstore"
	webhooksvc "element-skin/backend/internal/service/webhook"
)

type workerRuntime struct {
	openDatabase func(context.Context, config.Config) (*database.DB, error)
	openRedis    func(context.Context, config.Config) (redisstore.Store, error)
	run          func(context.Context, *database.DB, config.Config)
}

var defaultWorkerRuntime = workerRuntime{
	openDatabase: database.OpenExisting,
	openRedis: func(ctx context.Context, cfg config.Config) (redisstore.Store, error) {
		return redisstore.Open(ctx, cfg)
	},
	run: func(ctx context.Context, db *database.DB, cfg config.Config) {
		webhooksvc.NewWorker(db, cfg).Run(ctx)
	},
}

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := runWebhookWorker(ctx, cfg, defaultWorkerRuntime); err != nil {
		log.Fatal(err)
	}
}

func runWebhookWorker(ctx context.Context, cfg config.Config, runtime workerRuntime) error {
	workerMaxConnections := cfg.WebhookWorkerMaxConnections
	if workerMaxConnections <= 0 {
		workerMaxConnections = 2
	}
	if cfg.MaxConnections > workerMaxConnections {
		cfg.MaxConnections = workerMaxConnections
	}
	db, err := runtime.openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	redis, err := runtime.openRedis(ctx, cfg)
	if err != nil {
		return err
	}
	defer redis.Close()
	db.Permissions.Cache = &permissiondb.RedisPermCache{Store: redis}
	log.Printf("Element Skin webhook worker started")
	runtime.run(ctx, db, cfg)
	return nil
}
