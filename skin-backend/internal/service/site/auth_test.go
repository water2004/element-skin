package site_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAuthRegisterCreatesFirstAdminAndOfflineProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	userID, err := svc.Register(ctx, " auth-service@test.com ", "Password123", "AuthService", "", "")
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Users.GetByID(ctx, userID)
	if err != nil || user == nil || !user.IsAdmin || !user.IsSuperAdmin || user.Email != "auth-service@test.com" || user.DisplayName != "AuthService" {
		t.Fatalf("registered user mismatch: user=%#v err=%v", user, err)
	}
	profiles, err := db.Profiles.GetByUser(ctx, userID, 10)
	if err != nil || len(profiles) != 1 || profiles[0].ID != util.OfflineUUIDNoDash("auth_service") || profiles[0].Name != "auth_service" {
		t.Fatalf("registration profile mismatch: profiles=%#v err=%v", profiles, err)
	}
}

func TestAuthRegisterRejectsPolicyFailuresWithoutCreatingUser(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())

	if err := db.Settings.Set(ctx, "allow_register", "false"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "closed-register@test.com", "Password123", "ClosedRegister", "", ""); !httpError(err, 403, "registration is disabled") {
		t.Fatalf("closed registration should reject exactly, got %#v", err)
	}
	if user, err := db.Users.GetByEmail(ctx, "closed-register@test.com"); err != nil || user != nil {
		t.Fatalf("closed registration must not create user: user=%#v err=%v", user, err)
	}

	if err := db.Settings.Set(ctx, "allow_register", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "enable_strong_password_check", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "weak-register@test.com", "weak", "WeakRegister", "", ""); err == nil {
		t.Fatal("strong password policy should reject weak registration password")
	}
	if user, err := db.Users.GetByEmail(ctx, "weak-register@test.com"); err != nil || user != nil {
		t.Fatalf("weak password registration must not create user: user=%#v err=%v", user, err)
	}
}

func TestAuthRegisterConsumesVerificationAndInviteExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "INVITE_ONCE", 1, "Invite Once"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Redis.SetVerificationCode(ctx, "verified-register@test.com", "register", "ABC12345", 0); err != nil {
		t.Fatal(err)
	}

	userID, err := svc.Register(ctx, "verified-register@test.com", "Password123", "VerifiedRegister", "INVITE_ONCE", "abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if user, err := db.Users.GetByID(ctx, userID); err != nil || user == nil || user.Email != "verified-register@test.com" || user.DisplayName != "VerifiedRegister" {
		t.Fatalf("verified invite registration should create user: user=%#v err=%v", user, err)
	}
	if _, err := svc.Redis.GetVerificationCode(ctx, "verified-register@test.com", "register"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful register should consume verification code, got %v", err)
	}
	invite, err := db.Invites.Get(ctx, "INVITE_ONCE")
	if err != nil || invite == nil || invite.UsedCount != 1 || invite.UsedBy == nil || *invite.UsedBy != "verified-register@test.com" {
		t.Fatalf("successful register should consume invite exactly: invite=%#v err=%v", invite, err)
	}

	if err := svc.Redis.SetVerificationCode(ctx, "second-register@test.com", "register", "SECOND12", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "second-register@test.com", "Password123", "SecondRegister", "INVITE_ONCE", "SECOND12"); !httpError(err, 400, "invite code has no remaining uses") {
		t.Fatalf("exhausted invite should reject exactly, got %#v", err)
	}
	if user, err := db.Users.GetByEmail(ctx, "second-register@test.com"); err != nil || user != nil {
		t.Fatalf("exhausted invite must not create user: user=%#v err=%v", user, err)
	}
	if stored, err := svc.Redis.GetVerificationCode(ctx, "second-register@test.com", "register"); err != nil || stored != "SECOND12" {
		t.Fatalf("failed registration must preserve the verification code for retry: code=%q err=%v", stored, err)
	}
}

func TestConcurrentRegistrationsConsumeSingleUseInviteExactlyOnce(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	testutil.CreateUser(t, db, "invite-race-seed@test.com", "Password123", "InviteRaceSeed", false)
	if err := db.Settings.Set(ctx, "require_invite", "true"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "INVITE_RACE_ONCE", 1, "Concurrent single use"); err != nil {
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
		case result.id == "" && httpError(result.err, 400, "invite code has no remaining uses"):
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

func TestAuthRejectsInvalidCredentialsAndRegistrationIdentityConflicts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	existing := testutil.CreateUser(t, db, "auth-existing@test.com", "Password123", "AuthExisting", false)

	for _, tc := range []struct {
		name     string
		email    string
		password string
	}{
		{name: "missing account", email: "missing-auth@test.com", password: "Password123"},
		{name: "wrong password", email: existing.Email, password: "WrongPassword"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Login(ctx, tc.email, tc.password)
			if !httpError(err, 401, "Invalid credentials") || res != nil {
				t.Fatalf("Login(%s) should reject exactly: res=%#v err=%#v", tc.name, res, err)
			}
		})
	}

	for _, tc := range []struct {
		name     string
		email    string
		username string
		want     string
	}{
		{name: "missing username", email: "missing-name@test.com", username: "   ", want: "Username is required"},
		{name: "invalid email", email: "not-an-email", username: "ValidName", want: "Invalid email format"},
		{name: "duplicate username", email: "new-email@test.com", username: existing.DisplayName, want: "Username already exists"},
		{name: "duplicate email", email: existing.Email, username: "DifferentName", want: "Email already registered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := svc.Register(ctx, tc.email, "Password123", tc.username, "", "")
			if !httpError(err, 400, tc.want) || id != "" {
				t.Fatalf("Register(%s) should reject exactly: id=%q err=%#v", tc.name, id, err)
			}
		})
	}

	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("rejected auth attempts must not create users: count=%d err=%v", count, err)
	}
}

func TestConcurrentRegistrationsKeepDisplayNameUnique(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
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
		case result.id == "" && httpError(result.err, 400, "Username already exists"):
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
	svc := newSiteService(db, testutil.TestConfig())
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
		case result.id == "" && httpError(result.err, 400, "Email already registered"):
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
	svc := newSiteService(db, testutil.TestConfig())
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
	var users, profiles, superAdmins int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users WHERE id = ANY($1)),
			(SELECT COUNT(*) FROM profiles WHERE user_id = ANY($1)),
			(SELECT COUNT(*) FROM users WHERE id = ANY($1) AND is_super_admin=TRUE)
	`, userIDs).Scan(&users, &profiles, &superAdmins); err != nil {
		t.Fatal(err)
	}
	if users != 2 || profiles != 2 || superAdmins != 1 {
		t.Fatalf("registration state: users=%d profiles=%d super_admins=%d; want 2, 2, 1", users, profiles, superAdmins)
	}
}

func TestRegisterStopsAfterGeneratedProfileNameCandidatesAreExhausted(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "profile-name-exhaust-owner@test.com", "Password123", "ProfileNameExhaustOwner", false)
	for attempt := 0; attempt < 100; attempt++ {
		testutil.CreateProfile(
			t,
			db,
			owner.ID,
			"profile_name_exhaust_"+strconv.Itoa(attempt),
			util.ProfileNameCandidate("collision", attempt),
		)
	}

	userID, err := svc.Register(ctx, "collision@new.test", "Password123", "ProfileNameExhaustNew", "", "")
	if userID != "" || !httpError(err, 500, "无法生成唯一角色名") {
		t.Fatalf("exhausted generated names: user_id=%q err=%#v, want empty id and exact 500", userID, err)
	}
	if user, err := db.Users.GetByEmail(ctx, "collision@new.test"); err != nil || user != nil {
		t.Fatalf("exhausted registration must not create user: user=%#v err=%v", user, err)
	}
	if count, err := db.Profiles.CountByUser(ctx, owner.ID); err != nil || count != 100 {
		t.Fatalf("exhausted registration changed existing profiles: count=%d err=%v", count, err)
	}
}
