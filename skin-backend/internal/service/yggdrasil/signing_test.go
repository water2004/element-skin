package yggdrasil_test

import (
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestNewSignerRejectsMissingKeyFiles(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = filepath.Join(t.TempDir(), "missing-private.pem")

	if _, err := yggdrasil.NewSigner(cfg); err == nil || !strings.Contains(err.Error(), "私钥") {
		t.Fatalf("missing private key should fail clearly, got %v", err)
	}
}
