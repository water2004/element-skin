package account_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
)

func TestConcurrentEmailChangesConsumeOneCodeExactlyOnce(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountServiceWithVerification(db, cache)
	first := testutil.CreateUser(t, db, "email-race-first@test.com", "Password123", "EmailRaceFirst", false)
	second := testutil.CreateUser(t, db, "email-race-second@test.com", "Password123", "EmailRaceSecond", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_user_email_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_user_email_update
		BEFORE UPDATE OF email ON users
		FOR EACH ROW EXECUTE FUNCTION delay_user_email_write();
	`); err != nil {
		t.Fatal(err)
	}

	const targetEmail = "email-race-target@test.com"
	const code = "EMAIL123"
	if err := cache.SetVerificationCode(ctx, targetEmail, "email_change", code, time.Hour); err != nil {
		t.Fatal(err)
	}
	results := runConcurrentSelfUpdates([]permission.Actor{
		accountActor(t, db, first.ID),
		accountActor(t, db, second.ID),
	}, func(actor permission.Actor) error {
		return svc.ChangeEmailSelf(context.Background(), actor, targetEmail, code)
	})
	assertOneSelfUpdateConflict(t, results, "verification_code.verify.invalid", "Email already in use")
	var targetCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE email=$1),
			COUNT(*) FILTER (WHERE email IN ($2,$3))
		FROM users
		WHERE id = ANY($4)
	`, targetEmail, first.Email, second.Email, []string{first.ID, second.ID}).Scan(&targetCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent email state: target=%d original=%d; want 1 and 1", targetCount, originalCount)
	}
}

func TestConcurrentDisplayNameUpdatesKeepNameUnique(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	first := testutil.CreateUser(t, db, "name-race-first@test.com", "Password123", "NameRaceFirst", false)
	second := testutil.CreateUser(t, db, "name-race-second@test.com", "Password123", "NameRaceSecond", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_user_display_name_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_user_display_name_update
		BEFORE UPDATE OF display_name ON users
		FOR EACH ROW EXECUTE FUNCTION delay_user_display_name_write();
	`); err != nil {
		t.Fatal(err)
	}

	const targetName = "SharedDisplayName"
	results := runConcurrentSelfUpdates([]permission.Actor{
		accountActor(t, db, first.ID),
		accountActor(t, db, second.ID),
	}, func(actor permission.Actor) error {
		return svc.UpdateSelf(context.Background(), actor, map[string]any{"display_name": targetName})
	})
	assertOneSelfUpdateConflict(t, results, "username.reserve.conflict")
	var targetCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE display_name=$1),
			COUNT(*) FILTER (WHERE display_name IN ($2,$3))
		FROM users
		WHERE id = ANY($4)
	`, targetName, first.DisplayName, second.DisplayName, []string{first.ID, second.ID}).Scan(&targetCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent display-name state: target=%d original=%d; want 1 and 1", targetCount, originalCount)
	}
}
