package yggdrasil_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func mustYggAuth(t *testing.T, ctx context.Context, ygg yggdrasil.Yggdrasil, email, password string) map[string]any {
	t.Helper()
	auth, err := ygg.Authenticate(ctx, email, password, "client", false)
	if err != nil {
		t.Fatalf("authenticate fixture failed: %v", err)
	}
	return auth
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("PostgreSQL error=%#v, want code %s", err, code)
	}
}

func sameToken(got, want model.Token) bool {
	if got.AccessToken != want.AccessToken ||
		got.ClientToken != want.ClientToken ||
		got.UserID != want.UserID ||
		got.CreatedAt != want.CreatedAt {
		return false
	}
	if got.ProfileID == nil || want.ProfileID == nil {
		return got.ProfileID == nil && want.ProfileID == nil
	}
	return *got.ProfileID == *want.ProfileID
}

func yggError(err error, status int, yggCode, detail string) bool {
	var target util.YggError
	return errors.As(err, &target) &&
		target.Status == status &&
		target.Code == yggCode &&
		target.Message == detail
}
