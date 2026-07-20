package integration_test

import (
	"context"
	"strconv"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

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
		if err := db.Invites.Create(ctx, code, testutil.Pointer(1), "page"); err != nil {
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
