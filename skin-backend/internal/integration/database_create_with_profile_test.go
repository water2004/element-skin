package integration_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestDatabaseCreateWithProfileRollsBackOnProfileConflict(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()

	atomicUser := model.User{ID: "atomic_user", Email: "atomic@test.com", Password: "hash", DisplayName: "AtomicUser"}
	atomicProfile := model.Profile{ID: "atomic_profile", UserID: atomicUser.ID, Name: "AtomicProfile", TextureModel: "default"}
	if err := db.Users.CreateWithProfile(ctx, atomicUser, atomicProfile, "", ""); err != nil {
		t.Fatal(err)
	}
	if u, _ := db.Users.GetByID(ctx, atomicUser.ID); u == nil {
		t.Fatal("atomic user should be created")
	}
	if p, _ := db.Profiles.GetByID(ctx, atomicProfile.ID); p == nil {
		t.Fatal("atomic profile should be created")
	}

	conflictUser := model.User{ID: "orphan_user", Email: "orphan@test.com", Password: "hash", DisplayName: "OrphanUser"}
	conflictProfile := model.Profile{ID: "orphan_profile", UserID: conflictUser.ID, Name: "AtomicProfile", TextureModel: "default"}
	if err := db.Users.CreateWithProfile(ctx, conflictUser, conflictProfile, "", ""); err == nil {
		t.Fatal("profile name conflict should fail")
	}
	if u, _ := db.Users.GetByID(ctx, conflictUser.ID); u != nil {
		t.Fatalf("profile conflict should roll back user insert: %#v", u)
	}
	if u, _ := db.Users.GetByEmail(ctx, conflictUser.Email); u != nil {
		t.Fatalf("profile conflict should not leave user by email: %#v", u)
	}
}

func TestDatabaseCreateWithProfileConsumesInviteExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()

	if err := db.Invites.Create(ctx, "GOOD_INVITE", testutil.Pointer(2), "good"); err != nil {
		t.Fatal(err)
	}
	invitedUser := model.User{ID: "invited_user", Email: "invited@test.com", Password: "hash", DisplayName: "InvitedUser"}
	invitedProfile := model.Profile{ID: "invited_profile", UserID: invitedUser.ID, Name: "InvitedProfile", TextureModel: "default"}
	if err := db.Users.CreateWithProfile(ctx, invitedUser, invitedProfile, "GOOD_INVITE", invitedUser.Email); err != nil {
		t.Fatal(err)
	}
	goodInvite, err := db.Invites.Get(ctx, "GOOD_INVITE")
	if err != nil {
		t.Fatal(err)
	}
	if goodInvite == nil || goodInvite.UsedCount != 1 || goodInvite.UsedBy == nil || *goodInvite.UsedBy != invitedUser.Email {
		t.Fatalf("invite should be consumed with used_by: %#v", goodInvite)
	}

	if err := db.Invites.Create(ctx, "FULL_INVITE", testutil.Pointer(1), "full"); err != nil {
		t.Fatal(err)
	}
	firstUser := model.User{ID: "first_invite_user", Email: "first@test.com", Password: "hash", DisplayName: "FirstInviteUser"}
	firstProfile := model.Profile{ID: "first_invite_profile", UserID: firstUser.ID, Name: "FirstInviteProfile", TextureModel: "default"}
	if err := db.Users.CreateWithProfile(ctx, firstUser, firstProfile, "FULL_INVITE", firstUser.Email); err != nil {
		t.Fatal(err)
	}
	fullUser := model.User{ID: "full_invite_user", Email: "full@test.com", Password: "hash", DisplayName: "FullInviteUser"}
	fullProfile := model.Profile{ID: "full_invite_profile", UserID: fullUser.ID, Name: "FullInviteProfile", TextureModel: "default"}
	if err := db.Users.CreateWithProfile(ctx, fullUser, fullProfile, "FULL_INVITE", fullUser.Email); err != invite.ErrExhausted {
		t.Fatalf("expected ErrInviteExhausted, got %v", err)
	}
	if u, _ := db.Users.GetByID(ctx, fullUser.ID); u != nil {
		t.Fatalf("exhausted invite should roll back user: %#v", u)
	}
	if p, _ := db.Profiles.GetByID(ctx, fullProfile.ID); p != nil {
		t.Fatalf("exhausted invite should roll back profile: %#v", p)
	}
}

func TestDatabaseCreateWithProfileAllowsSingleInviteRaceWinner(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()

	if err := db.Invites.Create(ctx, "RACE_INVITE", testutil.Pointer(1), "race"); err != nil {
		t.Fatal(err)
	}
	wins := make(chan bool, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := model.User{ID: "race_user_" + strconv.Itoa(i), Email: "race" + strconv.Itoa(i) + "@test.com", Password: "hash", DisplayName: "RaceUser" + strconv.Itoa(i)}
			p := model.Profile{ID: "race_profile_" + strconv.Itoa(i), UserID: u.ID, Name: "RaceProfile" + strconv.Itoa(i), TextureModel: "default"}
			err := db.Users.CreateWithProfile(context.Background(), u, p, "RACE_INVITE", u.Email)
			if err == nil {
				wins <- true
				return
			}
			if err != invite.ErrExhausted {
				t.Errorf("unexpected invite race error: %v", err)
			}
			wins <- false
		}()
	}
	wg.Wait()
	close(wins)
	successes := 0
	for ok := range wins {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one invite race winner, got %d", successes)
	}
	raceInvite, err := db.Invites.Get(ctx, "RACE_INVITE")
	if err != nil {
		t.Fatal(err)
	}
	if raceInvite == nil || raceInvite.UsedCount != 1 {
		t.Fatalf("race invite should be consumed once: %#v", raceInvite)
	}
}
