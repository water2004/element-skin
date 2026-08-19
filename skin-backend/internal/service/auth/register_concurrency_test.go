package auth_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestConcurrentRegistrationsConsumeSingleUseInviteExactlyOnce(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	testutil.CreateUser(t, db, "invite-race-seed@test.com", "Password123", "InviteRaceSeed", false)
	if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "INVITE_RACE_ONCE", testutil.Pointer(1), "Concurrent single use"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_invite_consumption() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_invite_consumption
		BEFORE UPDATE ON invites
		FOR EACH ROW
		WHEN (OLD.code = 'INVITE_RACE_ONCE')
		EXECUTE FUNCTION delay_invite_consumption();
	`); err != nil {
		t.Fatal(err)
	}

	type attempt struct {
		email    string
		username string
	}
	type result struct {
		attempt attempt
		id      string
		err     error
	}
	attempts := []attempt{
		{email: "invite-race-first@test.com", username: "InviteRaceFirst"},
		{email: "invite-race-second@test.com", username: "InviteRaceSecond"},
	}
	start := make(chan struct{})
	results := make(chan result, len(attempts))
	var wg sync.WaitGroup
	for _, candidate := range attempts {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id, err := svc.Register(
				context.Background(),
				candidate.email,
				"Password123",
				candidate.username,
				"INVITE_RACE_ONCE",
				"",
			)
			results <- result{attempt: candidate, id: id, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var winner attempt
	successes := 0
	exhausted := 0
	for result := range results {
		switch {
		case result.err == nil && result.id != "":
			successes++
			winner = result.attempt
		case result.id == "" && httpError(result.err, 400, "invite.consume.exhausted"):
			exhausted++
		default:
			t.Fatalf("unexpected concurrent invite result: attempt=%#v id=%q err=%#v", result.attempt, result.id, result.err)
		}
	}
	if successes != 1 || exhausted != 1 {
		t.Fatalf("concurrent invite results: successes=%d exhausted=%d; want 1 and 1", successes, exhausted)
	}
	inv, err := db.Invites.Get(ctx, "INVITE_RACE_ONCE")
	if err != nil || inv == nil || inv.UsedCount != 1 || inv.UsedBy == nil || *inv.UsedBy != winner.email {
		t.Fatalf("single-use invite state=%#v err=%v; want used_count=1 used_by=%q", inv, err, winner.email)
	}
	var users, profiles int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users WHERE email = ANY($1)),
			(SELECT COUNT(*) FROM profiles WHERE user_id IN (
				SELECT id FROM users WHERE email = ANY($1)
			))
	`, []string{attempts[0].email, attempts[1].email}).Scan(&users, &profiles); err != nil {
		t.Fatal(err)
	}
	if users != 1 || profiles != 1 {
		t.Fatalf("single-use invite persisted users=%d profiles=%d; want 1 and 1", users, profiles)
	}
}

func TestConcurrentRegistrationsKeepDisplayNameUnique(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	testutil.CreateUser(t, db, "registration-name-seed@test.com", "Password123", "RegistrationSeed", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_user_registration_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_user_registration_insert
		BEFORE INSERT ON users
		FOR EACH ROW EXECUTE FUNCTION delay_user_registration_write();
	`); err != nil {
		t.Fatal(err)
	}

	type result struct {
		id  string
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, email := range []string{"registration-name-first@test.com", "registration-name-second@test.com"} {
		email := email
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id, err := svc.Register(context.Background(), email, "Password123", "ConcurrentRegistration", "", "")
			results <- result{id: id, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result.err == nil && result.id != "":
			successes++
		case result.id == "" && httpError(result.err, 400, "username.reserve.conflict"):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent registration result: id=%q err=%#v", result.id, result.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent registrations: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
	var usersWithName, registeredProfiles int
	if err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE display_name='ConcurrentRegistration'`,
	).Scan(&usersWithName); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM profiles WHERE user_id IN (SELECT id FROM users WHERE display_name='ConcurrentRegistration')`,
	).Scan(&registeredProfiles); err != nil {
		t.Fatal(err)
	}
	if usersWithName != 1 || registeredProfiles != 1 {
		t.Fatalf("concurrent registration state: users=%d profiles=%d; want 1 and 1", usersWithName, registeredProfiles)
	}
}

func TestConcurrentRegistrationsReturnExactEmailConflict(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	testutil.CreateUser(t, db, "registration-email-seed@test.com", "Password123", "RegistrationEmailSeed", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_registration_email_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_registration_email_insert
		BEFORE INSERT ON users
		FOR EACH ROW EXECUTE FUNCTION delay_registration_email_write();
	`); err != nil {
		t.Fatal(err)
	}

	type result struct {
		id  string
		err error
	}
	const targetEmail = "registration-email-race@test.com"
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, username := range []string{"RegistrationEmailFirst", "RegistrationEmailSecond"} {
		username := username
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id, err := svc.Register(context.Background(), targetEmail, "Password123", username, "", "")
			results <- result{id: id, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result.err == nil && result.id != "":
			successes++
		case result.id == "" && httpError(result.err, 400, "email.register.already_exists"):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent email registration: id=%q err=%#v", result.id, result.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent email registrations: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
	var userCount, profileCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email=$1`, targetEmail).Scan(&userCount); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM profiles
		WHERE user_id IN (SELECT id FROM users WHERE email=$1)
	`, targetEmail).Scan(&profileCount); err != nil {
		t.Fatal(err)
	}
	if userCount != 1 || profileCount != 1 {
		t.Fatalf("concurrent email registration state: users=%d profiles=%d; want 1 and 1", userCount, profileCount)
	}
}

func TestConcurrentRegistrationsRetryConflictingGeneratedProfileName(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newAuthService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_generated_profile_insert() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_generated_profile_insert
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_generated_profile_insert();
	`); err != nil {
		t.Fatal(err)
	}

	type result struct {
		id  string
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i, email := range []string{"same-local@first.test", "same-local@second.test"} {
		i := i
		email := email
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id, err := svc.Register(
				context.Background(),
				email,
				"Password123",
				"GeneratedProfileUser"+strconv.Itoa(i),
				"",
				"",
			)
			results <- result{id: id, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	userIDs := make([]string, 0, 2)
	for result := range results {
		if result.err != nil || result.id == "" {
			t.Fatalf("both registrations should succeed after generated-name retry: id=%q err=%#v", result.id, result.err)
		}
		userIDs = append(userIDs, result.id)
	}
	type storedProfile struct {
		id   string
		name string
	}
	var stored []storedProfile
	rows, err := db.Pool.Query(ctx, `SELECT id,name FROM profiles WHERE user_id = ANY($1) ORDER BY name`, userIDs)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var profile storedProfile
		if err := rows.Scan(&profile.id, &profile.name); err != nil {
			t.Fatal(err)
		}
		stored = append(stored, profile)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 ||
		stored[0].name != "same_local" ||
		stored[0].id != util.OfflineUUIDNoDash("same_local") ||
		stored[1].name != "same_local_1" ||
		stored[1].id != util.OfflineUUIDNoDash("same_local_1") {
		t.Fatalf("generated offline profiles=%#v; want exact base and suffixed offline identities", stored)
	}
	var users, profiles, protectedSubjects int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users WHERE id = ANY($1)),
			(SELECT COUNT(*) FROM profiles WHERE user_id = ANY($1)),
			(SELECT COUNT(*)
			 FROM permission_subjects ps
			 WHERE ps.user_id = ANY($1) AND ps.protected=TRUE)
	`, userIDs).Scan(&users, &profiles, &protectedSubjects); err != nil {
		t.Fatal(err)
	}
	if users != 2 || profiles != 2 || protectedSubjects != 1 {
		t.Fatalf("registration state: users=%d profiles=%d protected_subjects=%d; want 2, 2, 1", users, profiles, protectedSubjects)
	}
}
