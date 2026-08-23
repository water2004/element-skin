package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/config"
)

type application interface {
	Handler() http.Handler
	Close()
}

type serverRuntime struct {
	newApplication func(context.Context, config.Config) (application, error)
	listen         func(*http.Server) error
	shutdown       func(*http.Server, context.Context) error
}

var defaultServerRuntime = serverRuntime{
	newApplication: func(ctx context.Context, cfg config.Config) (application, error) {
		return app.New(ctx, cfg)
	},
	listen: func(server *http.Server) error {
		return server.ListenAndServe()
	},
	shutdown: func(server *http.Server, ctx context.Context) error {
		return server.Shutdown(ctx)
	},
}

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := runServer(ctx, cfg, defaultServerRuntime); err != nil {
		log.Fatal(err)
	}
}

func runServer(ctx context.Context, cfg config.Config, runtime serverRuntime) error {
	application, err := runtime.newApplication(ctx, cfg)
	if err != nil {
		return err
	}
	defer application.Close()
	server := newHTTPServer(cfg, application.Handler())
	serveErrors := make(chan error, 1)
	go func() {
		log.Printf("Element Skin Go backend listening on %s", server.Addr)
		serveErrors <- runtime.listen(server)
	}()
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return runtime.shutdown(server, shutdownContext)
	}
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.ServerHost + ":" + cfg.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
