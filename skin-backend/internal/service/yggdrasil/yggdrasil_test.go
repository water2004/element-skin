package yggdrasil_test

import (
	"testing"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestNewLoadsSignerAndStoresDependencies(t *testing.T) {
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	ygg, err := yggdrasil.New(nil, cfg, redis, settings.Settings{Redis: redis})
	if err != nil {
		t.Fatal(err)
	}
	if ygg.Cfg.PublicKeyPath != cfg.PublicKeyPath || ygg.Signer == nil || ygg.Redis == nil {
		t.Fatalf("New should retain config and load signer: %#v", ygg)
	}
}
