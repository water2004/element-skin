package integration_test

import (
	"context"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"strconv"
	"sync"
	"testing"
	"time"
)

func TestDatabaseInitScripts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	for _, table := range []string{"users", "settings", "fallback_endpoints"} {
		var got string
		if err := db.Pool.QueryRow(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name=$1", table).Scan(&got); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
	v, err := db.Settings.Get(ctx, "enable_skin_library", "")
	if err != nil {
		t.Fatal(err)
	}
	if v != "true" {
		t.Fatalf("enable_skin_library=%q", v)
	}
}

func TestYggdrasilTokenStructStillUsable(t *testing.T) {
	_ = model.Token{AccessToken: "a", ClientToken: "c", UserID: "u", CreatedAt: time.Now().UnixMilli()}
}

func TestConcurrentRefreshSingleWinner(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "race@test.com", "Password123", "RaceUser", false)

	login := doJSON(t, h, "POST", "/site-login", map[string]any{"email": user.Email, "password": "Password123"})
	refresh := cookieNamed(login, "refresh_token")
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}

	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := doJSON(t, h, "POST", "/me/refresh-token", nil, refresh)
			codes <- rr.Code
		}()
	}
	wg.Wait()
	close(codes)

	seen := map[int]int{}
	for code := range codes {
		seen[code]++
	}
	if seen[200] != 1 || seen[401] != 1 {
		t.Fatalf("expected one 200 and one 401, got %#v", seen)
	}
	row, err := db.Tokens.GetRefresh(context.Background(), util.HashRefreshToken(refresh.Value))
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatal("old refresh token hash should be deleted")
	}
}

func TestDatabaseAtomicUserProfileInviteAndRefreshPrimitives(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "primitive@test.com", "Password123", "PrimitiveUser", false)
	now := database.NowMS()
	future := now + 7*24*3600*1000

	if err := db.Tokens.AddRefresh(ctx, "hash_consume", user.ID, future, now); err != nil {
		t.Fatal(err)
	}
	row, err := db.Tokens.ConsumeRefresh(ctx, "hash_consume")
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["user_id"] != user.ID || row["expires_at"] != future {
		t.Fatalf("unexpected consumed refresh row: %#v", row)
	}
	row, err = db.Tokens.ConsumeRefresh(ctx, "hash_consume")
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatalf("refresh token should be one-shot, got %#v", row)
	}

	if err := db.Tokens.AddRefresh(ctx, "hash_race", user.ID, future, now); err != nil {
		t.Fatal(err)
	}
	results := make(chan map[string]any, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := db.Tokens.ConsumeRefresh(context.Background(), "hash_race")
			if err != nil {
				t.Errorf("consume refresh: %v", err)
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for got := range results {
		if got != nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("expected one refresh consume winner, got %d", winners)
	}

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

	if err := db.Invites.Create(ctx, "GOOD_INVITE", 2, "good"); err != nil {
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

	if err := db.Invites.Create(ctx, "FULL_INVITE", 1, "full"); err != nil {
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

	if err := db.Invites.Create(ctx, "RACE_INVITE", 1, "race"); err != nil {
		t.Fatal(err)
	}
	wins := make(chan bool, 8)
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

func TestDatabaseCursorPaginationCoverage(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()

	userIDs := map[string]bool{}
	for i := 0; i < 8; i++ {
		id := "page_user_" + strconv.Itoa(i)
		userIDs[id] = true
		u := model.User{ID: id, Email: "page" + strconv.Itoa(i) + "@test.com", Password: "hash", DisplayName: "Page User " + strconv.Itoa(i), PreferredLanguage: "en_US"}
		if err := db.Users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	seenUsers := map[string]bool{}
	lastID := ""
	for i := 0; i < 20; i++ {
		page, err := db.Users.List(ctx, 3, lastID, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			id := raw["id"].(string)
			if seenUsers[id] {
				t.Fatalf("duplicate user page item %q", id)
			}
			seenUsers[id] = true
			if raw["email"] == "" || raw["display_name"] == "" {
				t.Fatalf("user pagination should map fields independently: %#v", raw)
			}
		}
		if page["has_next"] != true {
			break
		}
		lastID = page["next_key"].(map[string]any)["last_id"].(string)
	}
	for id := range userIDs {
		if !seenUsers[id] {
			t.Fatalf("user pagination missed %s, saw %#v", id, seenUsers)
		}
	}

	profileUser := testutil.CreateUser(t, db, "profiles-page@test.com", "Password123", "ProfilesPageUser", false)
	profileIDs := map[string]bool{}
	for i := 0; i < 5; i++ {
		id := "page_profile_" + strconv.Itoa(i)
		profileIDs[id] = true
		if err := db.Profiles.Create(ctx, model.Profile{ID: id, UserID: profileUser.ID, Name: "PageProfile" + strconv.Itoa(i), TextureModel: "default"}); err != nil {
			t.Fatal(err)
		}
	}
	seenProfiles := map[string]bool{}
	lastID = ""
	for i := 0; i < 20; i++ {
		page, err := db.Profiles.ListByUser(ctx, profileUser.ID, 2, lastID)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			id := raw["id"].(string)
			if seenProfiles[id] {
				t.Fatalf("duplicate profile page item %q", id)
			}
			seenProfiles[id] = true
		}
		if page["has_next"] != true {
			break
		}
		lastID = page["next_key"].(map[string]any)["last_id"].(string)
	}
	for id := range profileIDs {
		if !seenProfiles[id] {
			t.Fatalf("profile pagination missed %s, saw %#v", id, seenProfiles)
		}
	}

	baseTime := database.NowMS()
	inviteCodes := map[string]bool{}
	for i := 0; i < 6; i++ {
		code := "PAGE_INVITE_" + strconv.Itoa(i)
		inviteCodes[code] = true
		if err := db.Invites.Create(ctx, code, 1, "page"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `UPDATE invites SET created_at=$1 WHERE code=$2`, baseTime-int64(i*1000), code); err != nil {
			t.Fatal(err)
		}
	}
	seenInvites := map[string]bool{}
	var lastCreated *int64
	lastCode := ""
	for i := 0; i < 20; i++ {
		page, err := db.Invites.List(ctx, 2, lastCreated, lastCode)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			code := raw["code"].(string)
			if seenInvites[code] {
				t.Fatalf("duplicate invite page item %q", code)
			}
			seenInvites[code] = true
		}
		if page["has_next"] != true {
			break
		}
		next := page["next_key"].(map[string]any)
		v := next["last_created_at"].(int64)
		lastCreated = &v
		lastCode = next["last_code"].(string)
	}
	for code := range inviteCodes {
		if !seenInvites[code] {
			t.Fatalf("invite pagination missed %s, saw %#v", code, seenInvites)
		}
	}

	textureUser := testutil.CreateUser(t, db, "textures-page@test.com", "Password123", "TexturesPageUser", false)
	textureHashes := map[string]bool{}
	for i := 0; i < 5; i++ {
		hash := "page_skin_" + strconv.Itoa(i)
		textureHashes[hash] = true
		if err := db.Textures.AddToLibrary(ctx, textureUser.ID, hash, "skin", "Page Skin "+strconv.Itoa(i), false, "default"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := db.Textures.AddToLibrary(ctx, textureUser.ID, "page_cape_"+strconv.Itoa(i), "cape", "Page Cape "+strconv.Itoa(i), false, "default"); err != nil {
			t.Fatal(err)
		}
	}
	seenTextures := map[string]bool{}
	var lastTextureCreated *int64
	lastHash := ""
	for i := 0; i < 20; i++ {
		page, err := db.Textures.ListForUser(ctx, textureUser.ID, "skin", 2, lastTextureCreated, lastHash)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page["items"].([]map[string]any) {
			if raw["type"] != "skin" {
				t.Fatalf("type-filtered texture page returned non-skin: %#v", raw)
			}
			hash := raw["hash"].(string)
			if seenTextures[hash] {
				t.Fatalf("duplicate texture page item %q", hash)
			}
			seenTextures[hash] = true
		}
		if page["has_next"] != true {
			break
		}
		next := page["next_key"].(map[string]any)
		v := next["last_created_at"].(int64)
		lastTextureCreated = &v
		lastHash = next["last_hash"].(string)
	}
	for hash := range textureHashes {
		if !seenTextures[hash] {
			t.Fatalf("texture pagination missed %s, saw %#v", hash, seenTextures)
		}
	}
}

func TestDatabaseUserProfileTokenAndTextureCRUD(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "crud@test.com", "Password123", "CrudUser", false)

	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("CountUsers=%d err=%v", count, err)
	}
	if taken, err := db.Users.IsDisplayNameTaken(ctx, "CrudUser", ""); err != nil || !taken {
		t.Fatalf("display name should be taken: %v", err)
	}
	if err := db.Users.Update(ctx, user.ID, map[string]any{"email": "new@crud.com", "display_name": "NewCrud", "preferred_language": "en_US"}); err != nil {
		t.Fatal(err)
	}
	updated, _ := db.Users.GetByID(ctx, user.ID)
	if updated.Email != "new@crud.com" || updated.DisplayName != "NewCrud" || updated.PreferredLanguage != "en_US" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}
	if err := db.Users.Ban(ctx, user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.Users.IsBanned(ctx, user.ID); err != nil || !banned {
		t.Fatalf("expected banned user: %v", err)
	}
	if err := db.Users.Unban(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if banned, _ := db.Users.IsBanned(ctx, user.ID); banned {
		t.Fatal("expected unbanned user")
	}

	profile := testutil.CreateProfile(t, db, user.ID, "crud_profile", "CrudPlayer")
	skin := "skin_hash"
	cape := "cape_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(ctx, profile.ID, &cape); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}
	gotProfile, _ := db.Profiles.GetByID(ctx, profile.ID)
	if *gotProfile.SkinHash != skin || *gotProfile.CapeHash != cape || gotProfile.TextureModel != "slim" {
		t.Fatalf("unexpected profile: %#v", gotProfile)
	}

	if ok, err := db.Profiles.DeleteCascade(ctx, profile.ID); err != nil || !ok {
		t.Fatalf("DeleteProfileCascade ok=%v err=%v", ok, err)
	}

	if err := db.Textures.AddToLibrary(ctx, user.ID, "texhash", "skin", "MySkin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if info, _ := db.Textures.GetInfo(ctx, user.ID, "texhash", "skin"); info["note"] != "MySkin" || info["is_public"].(int) != 1 {
		t.Fatalf("unexpected texture info: %#v", info)
	}
	if err := db.Textures.UpdateNote(ctx, user.ID, "texhash", "skin", "NewNote"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.UpdatePublic(ctx, user.ID, "texhash", "skin", false); err != nil {
		t.Fatal(err)
	}
	other := testutil.CreateUser(t, db, "other@test.com", "Password123", "Other", false)
	ok, err := db.Textures.AddToWardrobe(ctx, other.ID, "texhash", "skin")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("other user should not add private texture")
	}
	if err := db.Textures.UpdatePublic(ctx, user.ID, "texhash", "skin", true); err != nil {
		t.Fatal(err)
	}
	ok, err = db.Textures.AddToWardrobe(ctx, other.ID, "texhash", "skin")
	if err != nil || !ok {
		t.Fatalf("public wardrobe add ok=%v err=%v", ok, err)
	}
	if info, _ := db.Textures.GetInfo(ctx, other.ID, "texhash", "skin"); info == nil || info["is_public"].(int) != 2 {
		t.Fatalf("wardrobe copy should use is_public=2, got %#v", info)
	}

	modelHash := "modelhash"
	if err := db.Textures.AddToLibrary(ctx, user.ID, modelHash, "skin", "ModelSkin", true, "default"); err != nil {
		t.Fatal(err)
	}
	modelProfile := testutil.CreateProfile(t, db, user.ID, "model_profile", "ModelTester")
	if err := db.Profiles.UpdateSkin(ctx, modelProfile.ID, &modelHash); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.UpdateModel(ctx, user.ID, modelHash, "skin", "slim"); err != nil {
		t.Fatal(err)
	}
	updatedModelProfile, _ := db.Profiles.GetByID(ctx, modelProfile.ID)
	if updatedModelProfile.TextureModel != "slim" {
		t.Fatalf("owner model update should cascade to profile, got %#v", updatedModelProfile)
	}
	otherModelUser := testutil.CreateUser(t, db, "other-model@test.com", "Password123", "OtherModel", false)
	if ok, err := db.Textures.AddToWardrobe(ctx, otherModelUser.ID, modelHash, "skin"); err != nil || !ok {
		t.Fatalf("other model wardrobe add ok=%v err=%v", ok, err)
	}
	if err := db.Textures.UpdateModel(ctx, otherModelUser.ID, modelHash, "skin", "default"); err != nil {
		t.Fatal(err)
	}
	updatedModelProfile, _ = db.Profiles.GetByID(ctx, modelProfile.ID)
	if updatedModelProfile.TextureModel != "slim" {
		t.Fatalf("non-uploader model update should not cascade owner profile, got %#v", updatedModelProfile)
	}

	privateHash := "private-readd-hash"
	if err := db.Textures.AddToLibrary(ctx, user.ID, privateHash, "skin", "PrivateSkin", false, "default"); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Textures.AddToWardrobe(ctx, other.ID, privateHash, "skin"); err != nil || ok {
		t.Fatalf("private library texture should not be addable through public wardrobe flow ok=%v err=%v", ok, err)
	}
}
