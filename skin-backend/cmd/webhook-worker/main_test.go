package main

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

type trackingStore struct {
	redisstore.Store
	closeCalls int
}

func (s *trackingStore) Close() error {
	s.closeCalls++
	return s.Store.Close()
}

func TestRunWebhookWorkerCapsConnectionsAndClosesDependenciesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cache := redisstore.NewMemoryStore()
	trackedCache := &trackingStore{Store: cache}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var openedConfig, runConfig config.Config
	var runCalls int
	runtime := workerRuntime{
		openDatabase: func(_ context.Context, cfg config.Config) (*database.DB, error) {
			openedConfig = cfg
			return db, nil
		},
		openRedis: func(_ context.Context, cfg config.Config) (redisstore.Store, error) {
			if cfg.MaxConnections != 3 {
				t.Fatalf("Redis config max_connections=%d want=3", cfg.MaxConnections)
			}
			return trackedCache, nil
		},
		run: func(gotContext context.Context, gotDB *database.DB, cfg config.Config) {
			runCalls++
			runConfig = cfg
			if gotDB != db || !errors.Is(gotContext.Err(), context.Canceled) || gotDB.Permissions.Cache == nil {
				t.Fatalf("worker runtime context=%v db_match=%v cache=%#v", gotContext.Err(), gotDB == db, gotDB.Permissions.Cache)
			}
		},
	}
	cfg := testutil.TestConfig()
	cfg.MaxConnections = 100
	cfg.WebhookWorkerMaxConnections = 3
	if err := runWebhookWorker(ctx, cfg, runtime); err != nil {
		t.Fatal(err)
	}
	if openedConfig.MaxConnections != 3 || runConfig.MaxConnections != 3 || runCalls != 1 {
		t.Fatalf("worker configs opened=%d run=%d calls=%d", openedConfig.MaxConnections, runConfig.MaxConnections, runCalls)
	}
	if trackedCache.closeCalls != 1 {
		t.Fatalf("worker Redis close calls=%d want=1", trackedCache.closeCalls)
	}
}

func TestRunWebhookWorkerUsesSafeDefaultAndPropagatesOpenFailuresExactly(t *testing.T) {
	wantDatabaseErr := errors.New("webhook database unavailable")
	var openedConfig config.Config
	runtime := workerRuntime{
		openDatabase: func(_ context.Context, cfg config.Config) (*database.DB, error) {
			openedConfig = cfg
			return nil, wantDatabaseErr
		},
		openRedis: func(context.Context, config.Config) (redisstore.Store, error) {
			t.Fatal("Redis should not open after database failure")
			return nil, nil
		},
		run: func(context.Context, *database.DB, config.Config) {
			t.Fatal("worker should not run after database failure")
		},
	}
	cfg := testutil.TestConfig()
	cfg.MaxConnections = 20
	cfg.WebhookWorkerMaxConnections = 0
	if err := runWebhookWorker(t.Context(), cfg, runtime); !errors.Is(err, wantDatabaseErr) {
		t.Fatalf("database open error=%v", err)
	}
	if openedConfig.MaxConnections != 2 {
		t.Fatalf("default worker max_connections=%d want=2", openedConfig.MaxConnections)
	}
}

func TestRunWebhookWorkerClosesDatabaseWhenRedisOpenFailsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	wantRedisErr := errors.New("webhook Redis unavailable")
	runtime := workerRuntime{
		openDatabase: func(context.Context, config.Config) (*database.DB, error) { return db, nil },
		openRedis: func(context.Context, config.Config) (redisstore.Store, error) {
			return nil, wantRedisErr
		},
		run: func(context.Context, *database.DB, config.Config) {
			t.Fatal("worker should not run after Redis failure")
		},
	}
	if err := runWebhookWorker(t.Context(), testutil.TestConfig(), runtime); !errors.Is(err, wantRedisErr) {
		t.Fatalf("Redis open error=%v", err)
	}
	if err := db.Pool.Ping(t.Context()); err == nil {
		t.Fatal("database should be closed after Redis open failure")
	}
}
