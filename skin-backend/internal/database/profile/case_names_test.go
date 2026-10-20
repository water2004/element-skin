package profile_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestNamesIgnoreCasePreserveIdentityAndRejectConcurrentReservations(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	user := testutil.CreateUser(t, db, "case@test.com", "Password123", "CaseUser", false)
	original := testutil.CreateProfile(t, db, user.ID, "case-original", "CasePlayer")
	got, err := db.Profiles.GetByName(ctx, "caseplayer")
	if err != nil || got == nil || !reflect.DeepEqual(*got, original) {
		t.Fatalf("lookup=%#v err=%v want=%#v", got, err, original)
	}
	items, err := db.Profiles.SearchByNames(ctx, []string{"CASEPLAYER", "caseplayer", "missing"}, 100)
	if err != nil || !reflect.DeepEqual(items, []model.Profile{original}) {
		t.Fatalf("bulk=%#v err=%v", items, err)
	}
	if ok, err := db.Profiles.UpdateName(ctx, original.ID, "CASEPLAYER"); err != nil || !ok {
		t.Fatalf("display-case rename=%v err=%v", ok, err)
	}
	original.Name = "CASEPLAYER"
	got, err = db.Profiles.GetByID(ctx, original.ID)
	if err != nil || got == nil || !reflect.DeepEqual(*got, original) {
		t.Fatalf("rename changed identity: got=%#v err=%v", got, err)
	}
	other := testutil.CreateProfile(t, db, user.ID, "case-other", "OtherPlayer")
	if _, err := db.Profiles.UpdateOwnedName(ctx, other.ID, user.ID, "caseplayer", true); !profile.IsNameConflict(err) {
		t.Fatalf("owned rename conflict=%v", err)
	}
	got, err = db.Profiles.GetByID(ctx, other.ID)
	if err != nil || got == nil || !reflect.DeepEqual(*got, other) {
		t.Fatalf("failed rename mutated profile: %#v err=%v", got, err)
	}

	names := []string{"RacePlayer", "raceplayer", "RACEPLAYER"}
	errs := make([]error, len(names))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			<-start
			errs[i] = db.Profiles.Create(ctx, model.Profile{
				ID: fmt.Sprintf("race-%d", i), UserID: user.ID, Name: name, TextureModel: "default",
			})
		})
	}
	close(start)
	wg.Wait()
	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case profile.IsNameConflict(err):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || conflicts != 2 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE lower(name)='raceplayer'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("race rows=%d err=%v", count, err)
	}
}
