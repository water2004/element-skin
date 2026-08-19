package auth_test

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	authsvc "element-skin/backend/internal/service/auth"
	mailsvc "element-skin/backend/internal/service/mail"
	settingssvc "element-skin/backend/internal/service/settings"
	verificationsvc "element-skin/backend/internal/service/verification"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func newAuthService(db *database.DB, cfg config.Config) authsvc.Service {
	redis := testutil.NewMemoryRedis()
	return newAuthServiceWithRedis(db, cfg, redis)
}

func newAuthServiceWithRedis(db *database.DB, cfg config.Config, redis redisstore.Store) authsvc.Service {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	verification := verificationsvc.Service{DB: db, Redis: redis, Settings: settings, Sender: testMailSender{}}
	return authsvc.Service{DB: db, Cfg: cfg, Redis: redis, Settings: settings, Verification: verification}
}

type testMailSender struct{}

func (testMailSender) SendVerificationCode(context.Context, string, string, string) error {
	return nil
}

var _ mailsvc.Sender = testMailSender{}

func httpError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Error() == detail
}

func closedPoolError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}

func assertPgCode(t testingT, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

type testingT interface {
	Helper()
	Fatalf(string, ...any)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type deleteYggFailStore struct {
	redisstore.Store
	deleteCalls int
}

func (s *deleteYggFailStore) DeleteYggTokensByUser(context.Context, string) error {
	s.deleteCalls++
	return errors.New("ygg token revocation failed")
}
