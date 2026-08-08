package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	webhooksvc "element-skin/backend/internal/service/webhook"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.MaxConnections > 5 {
		cfg.MaxConnections = 5
	}
	db, err := database.OpenExisting(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Printf("Element Skin webhook worker started")
	webhooksvc.NewWorker(db, cfg).Run(ctx)
}
