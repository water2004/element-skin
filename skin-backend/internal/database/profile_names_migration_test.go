package database_test

import (
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func legacyCaseSensitiveNames(t *testing.T, db *database.DB) {
	t.Helper()
	if _, err := db.Pool.Exec(t.Context(), `DROP INDEX profiles_name_key; ALTER TABLE profiles ADD CONSTRAINT profiles_name_key UNIQUE(name)`); err != nil {
		t.Fatal(err)
	}
}

func TestProfileNameMigrationUsesImportCandidatesAndPreservesIdentity(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	user := testutil.CreateUser(t, db, "migration-case@test.com", "Password123", "CaseMigration", false)
	other := testutil.CreateUser(t, db, "migration-other@test.com", "Password123", "OtherMigration", false)
	legacyCaseSensitiveNames(t, db)
	fixtures := []struct{ id, name, want string }{
		{"a0", "CasePlayer", "CasePlayer"},
		{"a1", "caseplayer", "caseplayer_2"},
		{"a2", "CASEPLAYER", "CASEPLAYER_3"},
		{"b0", "Abcdefghijklmnop", "Abcdefghijklmnop"},
		{"b1", "ABCDEFGHIJKLMNOP", util.ProfileNameCandidate("ABCDEFGHIJKLMNOP", 2)},
		{"c0", "CasePlayer_1", "CasePlayer_1"},
		{"c1", "abcdefghijklmn_1", "abcdefghijklmn_1"},
	}
	expected := make(map[string]model.Profile)
	for i, item := range fixtures {
		ownerID := user.ID
		if i%2 != 0 {
			ownerID = other.ID
		}
		p := model.Profile{
			ID: item.id, UserID: ownerID, Name: item.name, TextureModel: "slim",
			SkinHash: testutil.Pointer("migration-skin"), CapeHash: testutil.Pointer("migration-cape"),
		}
		if err := db.Profiles.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		p.Name = item.want
		expected[p.ID] = p
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	type audit struct {
		ID, Old, New string
		At           int64
	}
	readAudit := func() []audit {
		rows, err := db.Pool.Query(ctx, `SELECT profile_id,old_name,new_name,migrated_at FROM profile_name_case_migrations ORDER BY profile_id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []audit
		for rows.Next() {
			var a audit
			if err := rows.Scan(&a.ID, &a.Old, &a.New, &a.At); err != nil {
				t.Fatal(err)
			}
			out = append(out, a)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	audits := readAudit()
	if len(audits) != 3 {
		t.Fatalf("audit=%#v", audits)
	}
	for i, item := range []struct{ id, old, name string }{{"a1", "caseplayer", "caseplayer_2"}, {"a2", "CASEPLAYER", "CASEPLAYER_3"}, {"b1", "ABCDEFGHIJKLMNOP", util.ProfileNameCandidate("ABCDEFGHIJKLMNOP", 2)}} {
		got := audits[i]
		if got.ID != item.id || got.Old != item.old || got.New != item.name || got.At <= 0 {
			t.Fatalf("audit=%#v want=%#v", got, item)
		}
	}
	for range 2 {
		if err := db.Init(ctx); err != nil {
			t.Fatalf("restart: %v", err)
		}
		for id, want := range expected {
			got, err := db.Profiles.GetByID(ctx, id)
			if err != nil || got == nil || !reflect.DeepEqual(*got, want) {
				t.Fatalf("identity got=%#v want=%#v err=%v", got, want, err)
			}
		}
		if got := readAudit(); !reflect.DeepEqual(got, audits) {
			t.Fatalf("restart rewrote audit=%#v want=%#v", got, audits)
		}
	}
	var definition string
	if err := db.Pool.QueryRow(ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname='public' AND indexname='profiles_name_key'`).Scan(&definition); err != nil || definition != "CREATE UNIQUE INDEX profiles_name_key ON public.profiles USING btree (lower(name))" {
		t.Fatalf("index=%q err=%v", definition, err)
	}
}

func TestProfileNameMigrationExhaustionRollsBackRenamesAuditAndConstraint(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	user := testutil.CreateUser(t, db, "exhaust-case@test.com", "Password123", "ExhaustCase", false)
	legacyCaseSensitiveNames(t, db)
	var original []model.Profile
	for _, item := range []struct{ id, name string }{{"a0", "First"}, {"a1", "FIRST"}, {"z0", "Blocked"}, {"z1", "BLOCKED"}} {
		original = append(original, testutil.CreateProfile(t, db, user.ID, item.id, item.name))
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO profiles (id,user_id,name) SELECT 'occupied-' || n, $1, 'blocked_' || n FROM generate_series(1,99) n`, user.ID); err != nil {
		t.Fatal(err)
	}
	err := db.Init(ctx)
	if err == nil || err.Error() != "profile name migration exhausted import name candidates for profile z1; see doc/minecraft-services.md" {
		t.Fatalf("error=%v", err)
	}
	for _, want := range original {
		got, err := db.Profiles.GetByID(ctx, want.ID)
		if err != nil || got == nil || !reflect.DeepEqual(*got, want) {
			t.Fatalf("rollback got=%#v want=%#v err=%v", got, want, err)
		}
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM profile_name_case_migrations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
	var oldConstraint bool
	if err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='profiles'::regclass AND conname='profiles_name_key')`).Scan(&oldConstraint); err != nil || !oldConstraint {
		t.Fatalf("old constraint=%v err=%v", oldConstraint, err)
	}
	if ok, err := db.Profiles.UpdateName(ctx, "occupied-99", "ReleasedName"); err != nil || !ok {
		t.Fatalf("release=%v err=%v", ok, err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{"a1": "FIRST_1", "z1": "BLOCKED_99"} {
		got, err := db.Profiles.GetByID(ctx, id)
		if err != nil || got == nil || got.Name != name || got.UserID != user.ID {
			t.Fatalf("retry %s got=%#v err=%v", id, got, err)
		}
	}
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM profile_name_case_migrations`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("retry audit count=%d err=%v", count, err)
	}
}
