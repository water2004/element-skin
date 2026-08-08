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

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	workerMaxConnections := cfg.WebhookWorkerMaxConnections
	if workerMaxConnections <= 0 {
		workerMaxConnections = 2
	}
	if cfg.MaxConnections > workerMaxConnections {
		cfg.MaxConnections = workerMaxConnections
	}
	db, err := database.OpenExisting(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	redis, err := redisstore.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer redis.Close()
	db.Permissions.Cache = &permissiondb.RedisPermCache{Store: redis}
	log.Printf("Element Skin webhook worker started")
	webhooksvc.NewWorker(db, cfg).Run(ctx)
}
