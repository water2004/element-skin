package site_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestSiteAuthAccountAndSessionExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	site := site.Site{DB: db, Cfg: cfg}

	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	userID, err := site.Register(ctx, "  site-user@test.com  ", "Password123", "SiteUser", "", "")
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Users.GetByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || user.Email != "site-user@test.com" || user.DisplayName != "SiteUser" || !user.IsAdmin {
		t.Fatalf("registered first user mismatch: %#v", user)
	}
	profiles, err := db.Profiles.GetByUser(ctx, userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ID != util.OfflineUUIDNoDash("site_user") || profiles[0].Name != "site_user" || profiles[0].TextureModel != "default" {
		t.Fatalf("registration profile mismatch: %#v", profiles)
	}

	login, err := site.Login(ctx, "site-user@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	if login["user_id"] != userID || login["is_admin"] != true || login["access_token"] == "" || login["refresh_token"] == "" {
		t.Fatalf("login response mismatch: %#v", login)
	}
	rotated, err := site.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if rotated["is_admin"] != true || rotated["access_token"] == "" || rotated["refresh_token"] == "" || rotated["refresh_token"] == login["refresh_token"] {
		t.Fatalf("rotated session mismatch: %#v", rotated)
	}
	if _, err := site.RotateRefresh(ctx, login["refresh_token"].(string)); err == nil {
		t.Fatal("old refresh token should be single-use after rotation")
	}
	if err := site.RevokeRefresh(ctx, rotated["refresh_token"].(string)); err != nil {
		t.Fatal(err)
	}
	if _, err := site.RotateRefresh(ctx, rotated["refresh_token"].(string)); err == nil {
		t.Fatal("revoked refresh token should not rotate")
	}

	if err := site.UpdateMe(ctx, userID, map[string]any{"email": "updated-site@test.com", "display_name": "UpdatedSite", "preferred_language": "en_US", "avatar_hash": "avatar1"}); err != nil {
		t.Fatal(err)
	}
	me, err := site.Me(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if me["email"] != "updated-site@test.com" || me["display_name"] != "UpdatedSite" || me["lang"] != "en_US" ||
		me["avatar_hash"].(*string) == nil || *me["avatar_hash"].(*string) != "avatar1" || me["profile_count"] != 1 || me["texture_count"] != 0 {
		t.Fatalf("Me response mismatch after update: %#v", me)
	}
	if err := db.Tokens.AddRefresh(ctx, "change_password_refresh", userID, database.NowMS()+60_000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := site.ChangePassword(ctx, userID, "Password123", "NewPassword123"); err != nil {
		t.Fatal(err)
	}
	changed, err := db.Users.GetByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !util.VerifyPassword("NewPassword123", changed.Password) {
		t.Fatal("ChangePassword should persist new password hash")
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, "change_password_refresh"); err != nil || refresh != nil {
		t.Fatalf("ChangePassword should revoke refresh tokens: refresh=%#v err=%v", refresh, err)
	}
}

func TestSiteProfilesTexturesAndLibraryExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	site := site.Site{DB: db, Cfg: testutil.TestConfig()}
	user := testutil.CreateUser(t, db, "profile-site@test.com", "Password123", "ProfileSite", false)
	other := testutil.CreateUser(t, db, "other-site@test.com", "Password123", "OtherSite", false)

	created, err := site.CreateProfile(ctx, user.ID, "MainRole", "slim")
	if err != nil {
		t.Fatal(err)
	}
	profileID := created["id"].(string)
	if created["name"] != "MainRole" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	if err := site.UpdateProfile(ctx, user.ID, profileID, "RenamedRole"); err != nil {
		t.Fatal(err)
	}
	listProfiles, err := site.ListMyProfiles(ctx, user.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	profileItems := listProfiles["items"].([]map[string]any)
	if len(profileItems) != 1 || profileItems[0]["id"] != profileID || profileItems[0]["name"] != "RenamedRole" || listProfiles["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", listProfiles)
	}

	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_skin", "skin", "Site Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_cape", "cape", "Site Cape", false, "default"); err != nil {
		t.Fatal(err)
	}
	if err := site.ApplyTextureToProfile(ctx, user.ID, profileID, "site_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := site.ApplyTextureToProfile(ctx, user.ID, profileID, "site_cape", "cape"); err != nil {
		t.Fatal(err)
	}
	withTextures, err := db.Profiles.GetByID(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if withTextures.SkinHash == nil || *withTextures.SkinHash != "site_skin" || withTextures.CapeHash == nil || *withTextures.CapeHash != "site_cape" || withTextures.TextureModel != "slim" {
		t.Fatalf("ApplyTextureToProfile did not update profile exactly: %#v", withTextures)
	}

	detail, err := site.UpdateTexture(ctx, user.ID, "site_skin", "skin", map[string]any{"note": "Updated Skin", "model": "default", "is_public": false})
	if err != nil {
		t.Fatal(err)
	}
	if detail["ok"] != true || detail["note"] != "Updated Skin" || detail["model"] != "default" || detail["is_public"] != 0 {
		t.Fatalf("UpdateTexture detail mismatch: %#v", detail)
	}
	afterModel, err := db.Profiles.GetByID(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if afterModel.TextureModel != "default" {
		t.Fatalf("UpdateTexture model should propagate to profile using skin: %#v", afterModel)
	}

	if err := db.Textures.AdminUpdatePublic(ctx, "site_skin", "skin", true); err != nil {
		t.Fatal(err)
	}
	if err := site.AddTextureToWardrobe(ctx, other.ID, "site_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	otherTexture, err := site.TextureDetail(ctx, other.ID, "site_skin", "skin")
	if err != nil {
		t.Fatal(err)
	}
	if otherTexture["note"] != "Updated Skin" || otherTexture["is_public"] != 2 {
		t.Fatalf("wardrobe texture mismatch: %#v", otherTexture)
	}
	public, err := site.PublicLibrary(ctx, "", 10, "skin", "Updated")
	if err != nil {
		t.Fatal(err)
	}
	publicItems := public["items"].([]map[string]any)
	if len(publicItems) != 1 || publicItems[0]["hash"] != "site_skin" || publicItems[0]["name"] != "Updated Skin" {
		t.Fatalf("PublicLibrary mismatch: %#v", public)
	}
	myTextures, err := site.ListMyTextures(ctx, user.ID, "", 10, "skin")
	if err != nil {
		t.Fatal(err)
	}
	textureItems := myTextures["items"].([]map[string]any)
	if len(textureItems) != 1 || textureItems[0]["hash"] != "site_skin" || textureItems[0]["note"] != "Updated Skin" {
		t.Fatalf("ListMyTextures mismatch: %#v", myTextures)
	}

	if err := site.ClearProfileTexture(ctx, user.ID, profileID, "skin"); err != nil {
		t.Fatal(err)
	}
	cleared, err := db.Profiles.GetByID(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.SkinHash != nil || cleared.CapeHash == nil {
		t.Fatalf("ClearProfileTexture should clear only skin: %#v", cleared)
	}
	if err := site.DeleteTexture(ctx, user.ID, "site_cape", "cape"); err != nil {
		t.Fatal(err)
	}
	if cape, err := db.Textures.GetInfo(ctx, user.ID, "site_cape", "cape"); err != nil || cape != nil {
		t.Fatalf("DeleteTexture should remove user's cape: cape=%#v err=%v", cape, err)
	}
	if err := site.DeleteProfile(ctx, user.ID, profileID); err != nil {
		t.Fatal(err)
	}
	if deleted, err := db.Profiles.GetByID(ctx, profileID); err != nil || deleted != nil {
		t.Fatalf("DeleteProfile should remove profile: profile=%#v err=%v", deleted, err)
	}
}

func TestSiteVerificationAndResetPasswordExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	site := site.Site{DB: db, Cfg: testutil.TestConfig()}
	user := testutil.CreateUser(t, db, "reset-site@test.com", "Password123", "ResetSite", false)
	if err := db.Settings.Set(ctx, "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(ctx, "email_verify_ttl", "180"); err != nil {
		t.Fatal(err)
	}

	res, err := site.SendVerificationCode(ctx, "new-register@test.com", "register")
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true || res["ttl"] != 180 {
		t.Fatalf("SendVerificationCode register response mismatch: %#v", res)
	}
	code, expiresAt, ok, err := db.Verifications.GetCode(ctx, "new-register@test.com", "register")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(code) != 8 || strings.ToUpper(code) != code || expiresAt <= database.NowMS() {
		t.Fatalf("register verification code mismatch: code=%q expires=%d ok=%v", code, expiresAt, ok)
	}

	resetRes, err := site.SendVerificationCode(ctx, "reset-site@test.com", "reset")
	if err != nil {
		t.Fatal(err)
	}
	if resetRes["ttl"] != 180 {
		t.Fatalf("reset ttl mismatch: %#v", resetRes)
	}
	resetCode, _, ok, err := db.Verifications.GetCode(ctx, "reset-site@test.com", "reset")
	if err != nil || !ok {
		t.Fatalf("reset code should exist: code=%q ok=%v err=%v", resetCode, ok, err)
	}
	if verified, err := site.VerifyCode(ctx, "reset-site@test.com", resetCode, "reset"); err != nil || !verified {
		t.Fatalf("VerifyCode should accept exact stored code: verified=%v err=%v", verified, err)
	}
	if err := db.Tokens.AddRefresh(ctx, "reset_refresh", user.ID, database.NowMS()+60_000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := site.ResetPassword(ctx, "reset-site@test.com", "ResetPassword123", resetCode); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !util.VerifyPassword("ResetPassword123", updated.Password) {
		t.Fatal("ResetPassword should persist new password hash")
	}
	if _, _, ok, err := db.Verifications.GetCode(ctx, "reset-site@test.com", "reset"); err != nil || ok {
		t.Fatalf("ResetPassword should consume reset verification code: ok=%v err=%v", ok, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, "reset_refresh"); err != nil || refresh != nil {
		t.Fatalf("ResetPassword should revoke refresh tokens: refresh=%#v err=%v", refresh, err)
	}
}
