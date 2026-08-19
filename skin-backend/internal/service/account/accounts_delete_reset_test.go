package account_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAccountServiceDeleteUserCascadesAndInvalidatesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-delete@test.com", "Password123", "AdminAccountDelete", true)
	target := testutil.CreateUser(t, db, "target-account-delete@test.com", "Password123", "TargetAccountDelete", false)
	other := testutil.CreateUser(t, db, "other-account-delete@test.com", "Password123", "OtherAccountDelete", false)
	unaffected := testutil.CreateUser(t, db, "unaffected-account-delete@test.com", "Password123", "UnaffectedAccountDelete", false)
	profile := testutil.CreateProfile(t, db, target.ID, "account_delete_profile", "AccountDeleteProfile")
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "account.delete.any")
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "account_delete_ygg", UserID: target.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	ownedClient := createAccountOAuthClient(t, db, target.ID, "account-delete-owned-client", "account.read.self")
	otherClient := createAccountOAuthClient(t, db, other.ID, "account-delete-other-client", "account.read.self")
	targetGrantToOther := model.OAuthGrant{
		ID:        "account-delete-target-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  otherClient.ID,
		Status:    "active",
		CreatedAt: 3100,
	}
	otherGrantToTargetApp := model.OAuthGrant{
		ID:        "account-delete-other-grant",
		UserID:    other.ID,
		SubjectID: permissiondb.SubjectIDForUser(other.ID),
		ClientID:  ownedClient.ID,
		Status:    "active",
		CreatedAt: 3200,
	}
	if err := db.OAuth.CreateGrant(ctx, targetGrantToOther, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateGrant(ctx, otherGrantToTargetApp, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	unaffectedClient := createAccountOAuthClient(t, db, unaffected.ID, "account-delete-unaffected-client", "account.read.self")
	unaffectedGrant := model.OAuthGrant{
		ID:        "account-delete-unaffected-grant",
		UserID:    unaffected.ID,
		SubjectID: permissiondb.SubjectIDForUser(unaffected.ID),
		ClientID:  unaffectedClient.ID,
		Status:    "active",
		CreatedAt: 3250,
	}
	if err := db.OAuth.CreateGrant(ctx, unaffectedGrant, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
		TokenHash: "account-delete-refresh",
		ClientID:  otherClient.ID,
		UserID:    target.ID,
		GrantID:   targetGrantToOther.ID,
		ExpiresAt: 9000,
		CreatedAt: 3300,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateAuthorizationCode(ctx, model.OAuthAuthorizationCode{
		CodeHash:            "account-delete-code",
		ClientID:            otherClient.ID,
		UserID:              target.ID,
		GrantID:             targetGrantToOther.ID,
		RedirectURI:         otherClient.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           9000,
		CreatedAt:           3400,
	}, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	userID := target.ID
	subjectID := permissiondb.SubjectIDForUser(target.ID)
	if err := db.OAuth.CreateDeviceCode(ctx, model.OAuthDeviceCode{
		DeviceCodeHash: "account-delete-device",
		UserCodeHash:   "account-delete-user-code",
		ClientID:       otherClient.ID,
		UserID:         &userID,
		SubjectID:      &subjectID,
		Status:         "approved",
		ExpiresAt:      9000,
		CreatedAt:      3500,
	}, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	oauthAccessTokens := []redisstore.OAuthAccessToken{
		{TokenHash: "account-delete-target-access", ClientID: otherClient.ID, UserID: target.ID, GrantID: targetGrantToOther.ID},
		{TokenHash: "account-delete-owned-client-access", ClientID: ownedClient.ID, UserID: other.ID, GrantID: otherGrantToTargetApp.ID},
		{TokenHash: "account-delete-unaffected-access", ClientID: unaffectedClient.ID, UserID: unaffected.ID, GrantID: unaffectedGrant.ID},
	}
	for _, token := range oauthAccessTokens {
		if err := cache.SetOAuthAccessToken(ctx, token, time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.DeleteUser(ctx, actor, target.ID); err != nil {
		t.Fatal(err)
	}
	if user, err := db.Users.GetByID(ctx, target.ID); err != nil || user != nil {
		t.Fatalf("delete should remove user row exactly: user=%#v err=%v", user, err)
	}
	if gotProfile, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || gotProfile != nil {
		t.Fatalf("delete should cascade profile row exactly: profile=%#v err=%v", gotProfile, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should invalidate auth cache exactly, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "account_delete_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should revoke ygg tokens exactly, got %v", err)
	}
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, ownedClient.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM permission_subjects WHERE id=$1`, permissiondb.SubjectIDForClient(ownedClient.ID), 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, otherClient.ID, 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, targetGrantToOther.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, otherGrantToTargetApp.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, unaffectedClient.ID, 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM permission_subjects WHERE id=$1`, permissiondb.SubjectIDForClient(unaffectedClient.ID), 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, unaffectedGrant.ID, 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_refresh_tokens WHERE token_hash=$1`, "account-delete-refresh", 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_authorization_codes WHERE code_hash=$1`, "account-delete-code", 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_device_codes WHERE device_code_hash=$1`, "account-delete-device", 0)
	if _, err := cache.GetOAuthAccessToken(ctx, "account-delete-target-access"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should remove target delegated access token exactly, got %v", err)
	}
	if _, err := cache.GetOAuthAccessToken(ctx, "account-delete-owned-client-access"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should remove access tokens issued to owned clients exactly, got %v", err)
	}
	if token, err := cache.GetOAuthAccessToken(ctx, "account-delete-unaffected-access"); err != nil || token.ClientID != unaffectedClient.ID || token.UserID != unaffected.ID {
		t.Fatalf("delete should preserve unrelated oauth access token: token=%#v err=%v", token, err)
	}

	if err := svc.DeleteUser(ctx, actor, adminUser.ID); !httpErrorIs(err, http.StatusForbidden, "user.delete.denied") {
		t.Fatalf("self delete error mismatch: %#v", err)
	}
	if err := svc.DeleteUser(ctx, actor, "missing-account-delete"); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("delete missing user mismatch: %#v", err)
	}
}

func TestAccountServiceResetPasswordRevokesTokensAndInvalidatesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-reset@test.com", "Password123", "AdminAccountReset", true)
	target := testutil.CreateUser(t, db, "target-account-reset@test.com", "Password123", "TargetAccountReset", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "account.update.any")
	refreshHash := "account_reset_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, target.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "account_reset_ygg", UserID: target.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: target.ID, NewPassword: "ChangedPassword123"}); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, target.ID)
	if err != nil || updated == nil || !util.VerifyPassword("ChangedPassword123", updated.Password) || util.VerifyPassword("Password123", updated.Password) {
		t.Fatalf("reset should persist exact new password hash: user=%#v err=%v", updated, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh != nil {
		t.Fatalf("reset should revoke refresh token exactly: refresh=%#v err=%v", refresh, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset should invalidate auth cache exactly, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "account_reset_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset should revoke ygg tokens exactly, got %v", err)
	}

	if err := svc.ResetPassword(ctx, permission.Actor{}, accountsvc.ResetPasswordInput{UserID: target.ID, NewPassword: "NextPassword123"}); !httpErrorIs(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("reset without permission mismatch: %#v", err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: target.ID}); !httpErrorIs(err, http.StatusBadRequest, "password_reset.validate.required") {
		t.Fatalf("reset missing password mismatch: %#v", err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: "missing-account-reset", NewPassword: "NextPassword123"}); !httpErrorIs(err, http.StatusNotFound, "user.resolve.not_found") {
		t.Fatalf("reset missing user mismatch: %#v", err)
	}
}

func TestAccountServiceDeleteUserRecountsSharedTexturesAndDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-delete-texture@test.com", "Password123", "AdminDeleteTexture", true)
	owner := testutil.CreateUser(t, db, "owner-account-delete-texture@test.com", "Password123", "OwnerDeleteTexture", false)
	target := testutil.CreateUser(t, db, "target-account-delete-texture@test.com", "Password123", "TargetDeleteTexture", false)
	other := testutil.CreateUser(t, db, "other-account-delete-texture@test.com", "Password123", "OtherDeleteTexture", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, target.ID, "delete_user_shared_skin", "skin"); err != nil || !added {
		t.Fatalf("seed target shared texture: added=%v err=%v", added, err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, other.ID, "delete_user_shared_skin", "skin"); err != nil || !added {
		t.Fatalf("seed other shared texture: added=%v err=%v", added, err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || !added {
		t.Fatalf("seed other uploaded texture: added=%v err=%v", added, err)
	}

	if err := svc.DeleteUser(ctx, actorWithPermissions(adminUser.ID, "account.delete.any"), target.ID); err != nil {
		t.Fatal(err)
	}
	public, err := db.Textures.ListPublic(ctx, texture.PublicListOptions{Limit: 10, TextureType: "skin", Query: "delete_user_shared_skin", Sort: texture.PublicLibrarySortMostUsed})
	if err != nil {
		t.Fatal(err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "delete_user_shared_skin" || items[0]["usage_count"] != int64(2) {
		t.Fatalf("shared texture usage after delete = %#v; want one row with usage_count=2", public)
	}
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}
