package integration_test

import (
	"context"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"net/http"
	"testing"
	"time"
)

func TestAdminProfilesTexturesInvitesHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "root@test.com", "Password123", "Root", true)
	user := testutil.CreateUser(t, db, "owned@test.com", "Password123", "Owned", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	userToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}
	userCookie := &http.Cookie{Name: "access_token", Value: userToken}

	profile := testutil.CreateProfile(t, db, user.ID, "admin_profile", "AdminList1")
	otherProfile := testutil.CreateProfile(t, db, user.ID, "admin_profile_other", "OtherName")
	forbiddenProfiles := doJSON(t, h, "GET", "/admin/profiles", nil, userCookie)
	if forbiddenProfiles.Code != 403 {
		t.Fatalf("non-admin profiles list should be 403, got %d", forbiddenProfiles.Code)
	}
	profiles := doJSON(t, h, "GET", "/admin/profiles?q=AdminList1", nil, adminCookie)
	if profiles.Code != 200 {
		t.Fatalf("admin profiles status=%d body=%s", profiles.Code, profiles.Body.String())
	}
	items := parseJSON(t, profiles)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "AdminList1" {
		t.Fatalf("unexpected admin profiles: %#v", items)
	}

	patchProfile := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID, map[string]any{"name": "AdminRenamed"}, adminCookie)
	if patchProfile.Code != 200 {
		t.Fatalf("admin patch profile status=%d body=%s", patchProfile.Code, patchProfile.Body.String())
	}
	p, _ := db.Profiles.GetByID(context.Background(), profile.ID)
	if p.Name != "AdminRenamed" {
		t.Fatal("admin profile rename did not persist")
	}
	duplicateProfile := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID, map[string]any{"name": otherProfile.Name}, adminCookie)
	if duplicateProfile.Code != 409 {
		t.Fatalf("admin duplicate profile name should be 409, got %d body=%s", duplicateProfile.Code, duplicateProfile.Body.String())
	}
	forbiddenPatchProfile := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID, map[string]any{"name": "Nope"}, userCookie)
	if forbiddenPatchProfile.Code != 403 {
		t.Fatalf("non-admin profile patch should be 403, got %d", forbiddenPatchProfile.Code)
	}
	missingProfileDelete := doJSON(t, h, "DELETE", "/admin/profiles/missing-profile", nil, adminCookie)
	if missingProfileDelete.Code != 404 {
		t.Fatalf("missing admin profile delete should be 404, got %d", missingProfileDelete.Code)
	}
	skinHash := "skinhash"
	if err := db.Profiles.UpdateSkin(context.Background(), profile.ID, &skinHash); err != nil {
		t.Fatal(err)
	}
	capeHash := "capehash"
	if err := db.Profiles.UpdateCape(context.Background(), profile.ID, &capeHash); err != nil {
		t.Fatal(err)
	}
	forbiddenClearSkin := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID+"/skin", map[string]any{"hash": nil}, userCookie)
	if forbiddenClearSkin.Code != 403 {
		t.Fatalf("non-admin clear skin should be 403, got %d", forbiddenClearSkin.Code)
	}
	clearSkin := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID+"/skin", map[string]any{"hash": nil}, adminCookie)
	if clearSkin.Code != 200 {
		t.Fatalf("admin clear skin status=%d body=%s", clearSkin.Code, clearSkin.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profile.ID)
	if p.SkinHash != nil || p.CapeHash == nil || *p.CapeHash != capeHash {
		t.Fatalf("clearing skin should not affect cape: %#v", p)
	}
	clearCape := doJSON(t, h, "PATCH", "/admin/profiles/"+profile.ID+"/cape", map[string]any{"hash": nil}, adminCookie)
	if clearCape.Code != 200 {
		t.Fatalf("admin clear cape status=%d body=%s", clearCape.Code, clearCape.Body.String())
	}
	p, _ = db.Profiles.GetByID(context.Background(), profile.ID)
	if p.CapeHash != nil {
		t.Fatal("admin cape clear did not persist")
	}
	missingClearCape := doJSON(t, h, "PATCH", "/admin/profiles/missing-profile/cape", map[string]any{"hash": nil}, adminCookie)
	if missingClearCape.Code != 404 {
		t.Fatalf("missing clear cape should be 404, got %d", missingClearCape.Code)
	}

	if err := db.Textures.AddToLibrary(context.Background(), user.ID, "adm_hash", "skin", "AdminTexture", true, "default"); err != nil {
		t.Fatal(err)
	}
	forbiddenTextures := doJSON(t, h, "GET", "/admin/textures", nil, userCookie)
	if forbiddenTextures.Code != 403 {
		t.Fatalf("non-admin textures list should be 403, got %d", forbiddenTextures.Code)
	}
	textures := doJSON(t, h, "GET", "/admin/textures?q=AdminTexture", nil, adminCookie)
	if textures.Code != 200 {
		t.Fatalf("admin textures status=%d body=%s", textures.Code, textures.Body.String())
	}
	textureItems := parseJSON(t, textures)["items"].([]any)
	if len(textureItems) != 1 || textureItems[0].(map[string]any)["hash"] != "adm_hash" {
		t.Fatalf("unexpected admin textures: %#v", textureItems)
	}
	patchTex := doJSON(t, h, "PATCH", "/admin/textures/adm_hash", map[string]any{"type": "skin", "is_public": 0, "note": "AdminRenamedTexture"}, adminCookie)
	if patchTex.Code != 200 {
		t.Fatalf("admin patch texture status=%d body=%s", patchTex.Code, patchTex.Body.String())
	}
	info, _ := db.Textures.GetInfo(context.Background(), user.ID, "adm_hash", "skin")
	if info["is_public"].(int) != 0 || info["note"] != "AdminRenamedTexture" {
		t.Fatalf("admin texture patch did not persist: %#v", info)
	}
	forbiddenPatchTexture := doJSON(t, h, "PATCH", "/admin/textures/adm_hash", map[string]any{"type": "skin", "is_public": 1}, userCookie)
	if forbiddenPatchTexture.Code != 403 {
		t.Fatalf("non-admin texture patch should be 403, got %d", forbiddenPatchTexture.Code)
	}
	forbiddenDeleteTexture := doJSON(t, h, "DELETE", "/admin/textures/adm_hash?user_id="+user.ID+"&type=skin", nil, userCookie)
	if forbiddenDeleteTexture.Code != 403 {
		t.Fatalf("non-admin texture delete should be 403, got %d", forbiddenDeleteTexture.Code)
	}
	delTex := doJSON(t, h, "DELETE", "/admin/textures/adm_hash?user_id="+user.ID+"&type=skin", nil, adminCookie)
	if delTex.Code != 200 {
		t.Fatalf("admin delete texture status=%d body=%s", delTex.Code, delTex.Body.String())
	}
	info, _ = db.Textures.GetInfo(context.Background(), user.ID, "adm_hash", "skin")
	if info != nil {
		t.Fatal("texture should be deleted from user library")
	}

	createInvite := doJSON(t, h, "POST", "/admin/invites", map[string]any{"code": "INV_HTTP", "total_uses": 5, "note": "API Code"}, adminCookie)
	if createInvite.Code != 200 {
		t.Fatalf("create invite status=%d body=%s", createInvite.Code, createInvite.Body.String())
	}
	badInvite := doRawJSON(t, h, "POST", "/admin/invites", `{"code":"BAD_INVITE"`, adminCookie)
	if badInvite.Code != 400 {
		t.Fatalf("malformed invite JSON should be 400, got %d body=%s", badInvite.Code, badInvite.Body.String())
	}
	if inv, err := db.Invites.Get(context.Background(), "BAD_INVITE"); err != nil || inv != nil {
		t.Fatalf("malformed invite JSON must not create invite: invite=%#v err=%v", inv, err)
	}
	invites := doJSON(t, h, "GET", "/admin/invites", nil, adminCookie)
	if invites.Code != 200 {
		t.Fatalf("list invites status=%d body=%s", invites.Code, invites.Body.String())
	}
	found := false
	for _, raw := range parseJSON(t, invites)["items"].([]any) {
		if raw.(map[string]any)["code"] == "INV_HTTP" {
			found = true
		}
	}
	if !found {
		t.Fatal("created invite not listed")
	}
	delInvite := doJSON(t, h, "DELETE", "/admin/invites/INV_HTTP", nil, adminCookie)
	if delInvite.Code != 200 {
		t.Fatalf("delete invite status=%d body=%s", delInvite.Code, delInvite.Body.String())
	}
	inv, _ := db.Invites.Get(context.Background(), "INV_HTTP")
	if inv != nil {
		t.Fatal("invite should be deleted")
	}
}

func TestAdminUserControlsHTTP(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "admin-controls@test.com", "Password123", "AdminControls", true, true)
	user := testutil.CreateUser(t, db, "normal-controls@test.com", "Password123", "NormalControls", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}

	search := doJSON(t, h, "GET", "/admin/users?q=NormalControls", nil, adminCookie)
	if search.Code != 200 {
		t.Fatalf("admin user search status=%d body=%s", search.Code, search.Body.String())
	}
	items := parseJSON(t, search)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != user.ID {
		t.Fatalf("unexpected user search result: %#v", items)
	}
	detail := doJSON(t, h, "GET", "/admin/users/"+user.ID, nil, adminCookie)
	if detail.Code != 200 {
		t.Fatalf("admin user detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	detailBody := parseJSON(t, detail)
	if detailBody["id"] != user.ID || detailBody["email"] != user.Email || detailBody["display_name"] != user.DisplayName {
		t.Fatalf("unexpected admin user detail: %#v", detailBody)
	}
	if _, ok := detailBody["password"]; ok {
		t.Fatalf("admin user detail should not expose password: %#v", detailBody)
	}
	missingDetail := doJSON(t, h, "GET", "/admin/users/missing-user", nil, adminCookie)
	if missingDetail.Code != 404 {
		t.Fatalf("missing admin user detail should be 404, got %d body=%s", missingDetail.Code, missingDetail.Body.String())
	}

	invalidCursor := doJSON(t, h, "GET", "/admin/users?cursor=garbage!!", nil, adminCookie)
	if invalidCursor.Code != 400 {
		t.Fatalf("invalid admin user cursor should be 400, got %d", invalidCursor.Code)
	}

	selfToggle := doJSON(t, h, "POST", "/admin/users/"+admin.ID+"/toggle-admin", nil, adminCookie)
	if selfToggle.Code != 403 {
		t.Fatalf("self toggle should be 403, got %d body=%s", selfToggle.Code, selfToggle.Body.String())
	}
	toggled := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/toggle-admin", nil, adminCookie)
	if toggled.Code != 200 || parseJSON(t, toggled)["is_admin"] != true {
		t.Fatalf("toggle admin status=%d body=%s", toggled.Code, toggled.Body.String())
	}
	row, _ := db.Users.GetByID(context.Background(), user.ID)
	if row == nil || !row.IsAdmin {
		t.Fatalf("user should now be admin: %#v", row)
	}
	toggledBack := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/toggle-admin", nil, adminCookie)
	if toggledBack.Code != 200 || parseJSON(t, toggledBack)["is_admin"] != false {
		t.Fatalf("toggle back status=%d body=%s", toggledBack.Code, toggledBack.Body.String())
	}

	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	ban := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/ban", map[string]any{"banned_until": bannedUntil}, adminCookie)
	if ban.Code != 200 {
		t.Fatalf("ban status=%d body=%s", ban.Code, ban.Body.String())
	}
	if banned, _ := db.Users.IsBanned(context.Background(), user.ID); !banned {
		t.Fatal("user should be banned")
	}
	unban := doJSON(t, h, "POST", "/admin/users/"+user.ID+"/unban", nil, adminCookie)
	if unban.Code != 200 {
		t.Fatalf("unban status=%d body=%s", unban.Code, unban.Body.String())
	}
	if banned, _ := db.Users.IsBanned(context.Background(), user.ID); banned {
		t.Fatal("user should be unbanned")
	}

	now := database.NowMS()
	refreshHashes := []string{util.HashRefreshToken("admin-reset-1"), util.HashRefreshToken("admin-reset-2")}
	for _, hsh := range refreshHashes {
		if err := db.Tokens.AddRefresh(context.Background(), hsh, user.ID, now+3600*1000, now); err != nil {
			t.Fatal(err)
		}
	}
	reset := doJSON(t, h, "POST", "/admin/users/reset-password", map[string]any{"user_id": user.ID, "new_password": "NewStr0ngPass!"}, adminCookie)
	if reset.Code != 200 {
		t.Fatalf("reset password status=%d body=%s", reset.Code, reset.Body.String())
	}
	updated, _ := db.Users.GetByID(context.Background(), user.ID)
	if !util.VerifyPassword("NewStr0ngPass!", updated.Password) {
		t.Fatal("password was not updated")
	}
	for _, hsh := range refreshHashes {
		if row, _ := db.Tokens.GetRefresh(context.Background(), hsh); row != nil {
			t.Fatal("admin reset should revoke refresh tokens")
		}
	}
	missingReset := doJSON(t, h, "POST", "/admin/users/reset-password", map[string]any{"user_id": "missing", "new_password": "x"}, adminCookie)
	if missingReset.Code != 404 {
		t.Fatalf("missing reset should be 404, got %d", missingReset.Code)
	}

	profile := testutil.CreateProfile(t, db, user.ID, "delete_user_profile", "DeleteUserProfile")
	if err := db.Textures.AddToLibrary(context.Background(), user.ID, "delete_user_texture", "skin", "DeleteUserTex", true, "default"); err != nil {
		t.Fatal(err)
	}
	del := doJSON(t, h, "DELETE", "/admin/users/"+user.ID, nil, adminCookie)
	if del.Code != 200 {
		t.Fatalf("delete user status=%d body=%s", del.Code, del.Body.String())
	}
	if row, _ := db.Users.GetByID(context.Background(), user.ID); row != nil {
		t.Fatal("user should be deleted")
	}
	if p, _ := db.Profiles.GetByID(context.Background(), profile.ID); p != nil {
		t.Fatal("user profiles should be deleted")
	}
	if ok, _ := db.Textures.VerifyOwnership(context.Background(), user.ID, "delete_user_texture", "skin"); ok {
		t.Fatal("user textures should be deleted")
	}
	missingDelete := doJSON(t, h, "DELETE", "/admin/users/"+user.ID, nil, adminCookie)
	if missingDelete.Code != 404 {
		t.Fatalf("missing delete should be 404, got %d", missingDelete.Code)
	}
}

func TestAdminTextureValidationEdges(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "edgeadmin@test.com", "Password123", "EdgeAdmin", true)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}

	invalidPublic := doJSON(t, h, "PATCH", "/admin/textures/somehash", map[string]any{"is_public": 2}, adminCookie)
	if invalidPublic.Code != 400 {
		t.Fatalf("invalid is_public should be 400, got %d body=%s", invalidPublic.Code, invalidPublic.Body.String())
	}
	missingPublic := doJSON(t, h, "PATCH", "/admin/textures/nonexistent-hash", map[string]any{"is_public": 0}, adminCookie)
	if missingPublic.Code != 404 {
		t.Fatalf("missing texture public update should be 404, got %d body=%s", missingPublic.Code, missingPublic.Body.String())
	}
	missingNote := doJSON(t, h, "PATCH", "/admin/textures/nonexistent-hash", map[string]any{"note": "missing"}, adminCookie)
	if missingNote.Code != 404 {
		t.Fatalf("missing texture note update should be 404, got %d body=%s", missingNote.Code, missingNote.Body.String())
	}
	invalidModel := doJSON(t, h, "PATCH", "/admin/textures/somehash", map[string]any{"model": "invalid"}, adminCookie)
	if invalidModel.Code != 400 {
		t.Fatalf("invalid model should be 400, got %d body=%s", invalidModel.Code, invalidModel.Body.String())
	}
	missingModel := doJSON(t, h, "PATCH", "/admin/textures/nonexistent-hash", map[string]any{"model": "slim"}, adminCookie)
	if missingModel.Code != 404 {
		t.Fatalf("missing texture model update should be 404, got %d body=%s", missingModel.Code, missingModel.Body.String())
	}
	missingUser := doJSON(t, h, "DELETE", "/admin/textures/somehash?type=skin", nil, adminCookie)
	if missingUser.Code != 400 {
		t.Fatalf("per-user delete without user_id should be 400, got %d body=%s", missingUser.Code, missingUser.Body.String())
	}

	user1 := testutil.CreateUser(t, db, "tex1@test.com", "Password123", "TexOne", false)
	user2 := testutil.CreateUser(t, db, "tex2@test.com", "Password123", "TexTwo", false)
	if err := db.Textures.AddToLibrary(context.Background(), user1.ID, "force_hash", "skin", "Force", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(context.Background(), user2.ID, "force_hash", "skin", "Copy", true, "default"); err != nil {
		t.Fatal(err)
	}
	force := doJSON(t, h, "DELETE", "/admin/textures/force_hash?type=skin&force=true", nil, adminCookie)
	if force.Code != 200 {
		t.Fatalf("force delete status=%d body=%s", force.Code, force.Body.String())
	}
	if ok, _ := db.Textures.VerifyOwnership(context.Background(), user1.ID, "force_hash", "skin"); ok {
		t.Fatal("force delete should remove user1 reference")
	}
	if ok, _ := db.Textures.VerifyOwnership(context.Background(), user2.ID, "force_hash", "skin"); ok {
		t.Fatal("force delete should remove user2 reference")
	}
}
