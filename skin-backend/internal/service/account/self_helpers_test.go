package account_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	mailsvc "element-skin/backend/internal/service/mail"
	settingssvc "element-skin/backend/internal/service/settings"
	verificationsvc "element-skin/backend/internal/service/verification"
)

func accountServiceWithVerification(db *database.DB, redis redisstore.Store) accountsvc.AccountService {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	verification := verificationsvc.Service{DB: db, Redis: redis, Settings: settings, Sender: accountTestMailSender{}}
	return accountsvc.AccountService{DB: db, Redis: redis, Verification: verification}
}

type accountTestMailSender struct{}

func (accountTestMailSender) SendVerificationCode(context.Context, string, string, string) error {
	return nil
}

var _ mailsvc.Sender = accountTestMailSender{}

func accountActor(t testing.TB, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(context.Background(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatalf("create account actor: %v", err)
	}
	return actor
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func runConcurrentSelfUpdates(actors []permission.Actor, update func(permission.Actor) error) []error {
	start := make(chan struct{})
	results := make(chan error, len(actors))
	var wg sync.WaitGroup
	for _, actor := range actors {
		actor := actor
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- update(actor)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	out := make([]error, 0, len(actors))
	for err := range results {
		out = append(out, err)
	}
	return out
}

func assertOneSelfUpdateConflict(t *testing.T, results []error, details ...string) {
	t.Helper()
	successes := 0
	conflicts := 0
	for _, err := range results {
		if err == nil {
			successes++
			continue
		}
		matched := false
		for _, detail := range details {
			if httpErrorIs(err, http.StatusBadRequest, detail) {
				matched = true
				break
			}
		}
		if matched {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent account result: %#v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent account updates: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
}

type deleteYggFailStore struct {
	redisstore.Store
	deleteCalls int
}

func (s *deleteYggFailStore) DeleteYggTokensByUser(context.Context, string) error {
	s.deleteCalls++
	return errors.New("ygg token revocation failed")
}
