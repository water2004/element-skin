package user_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

func TestStoreCreateUpdateDeleteAndInviteExhaustion(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	if err := db.Invites.Create(ctx, "domain_user_invite", testutil.Pointer(1), "domain user"); err != nil {
		t.Fatal(err)
	}
	hash, err := util.HashPassword("Password123")
	if err != nil {
		t.Fatal(err)
	}
	u := model.User{ID: "domain_user", Email: "domain-user@test.com", Password: hash, DisplayName: "DomainUser"}
	p := model.Profile{ID: "domain_user_profile", UserID: u.ID, Name: "DomainUserProfile", TextureModel: "default"}
	if err := store.CreateWithProfile(ctx, u, p, "domain_user_invite", u.Email); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWithProfile(ctx, model.User{ID: "domain_user_2", Email: "domain-user-2@test.com", Password: hash}, model.Profile{ID: "domain_user_profile_2", UserID: "domain_user_2", Name: "DomainUserProfile2", TextureModel: "default"}, "domain_user_invite", "second"); !errors.Is(err, invite.ErrExhausted) {
		t.Fatalf("expected exhausted invite, got %v", err)
	}
	if err := store.Update(ctx, u.ID, map[string]any{"email": "updated-domain-user@test.com", "display_name": "UpdatedDomainUser"}); err != nil {
		t.Fatal(err)
	}
	avatar := "domain-avatar-hash"
	if err := store.Update(ctx, u.ID, map[string]any{"preferred_language": "zh_CN", "avatar_hash": &avatar, "ignored_field": "ignored"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByEmail(ctx, "updated-domain-user@test.com")
	if err != nil || got == nil || got.ID != u.ID || got.DisplayName != "UpdatedDomainUser" || got.PreferredLanguage != "zh_CN" || got.AvatarHash == nil || *got.AvatarHash != avatar {
		t.Fatalf("updated user mismatch: user=%#v err=%v", got, err)
	}
	if count, err := store.Count(ctx); err != nil || count != 1 {
		t.Fatalf("count mismatch: count=%d err=%v", count, err)
	}
	if taken, err := store.IsDisplayNameTaken(ctx, "UpdatedDomainUser", ""); err != nil || !taken {
		t.Fatalf("display name should be taken: taken=%v err=%v", taken, err)
	}
	if taken, err := store.IsDisplayNameTaken(ctx, "UpdatedDomainUser", u.ID); err != nil || taken {
		t.Fatalf("display name exclude should ignore current user: taken=%v err=%v", taken, err)
	}
	directHash, err := util.HashPassword("DirectPassword123")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePassword(ctx, u.ID, directHash); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetByID(ctx, u.ID); err != nil || got == nil || !util.VerifyPassword("DirectPassword123", got.Password) {
		t.Fatalf("UpdatePassword should persist hash: user=%#v err=%v", got, err)
	}
	newHash, err := util.HashPassword("NewPassword123")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdatePasswordAndRevokeRefresh(ctx, u.ID, newHash)
	if err != nil || !updated {
		t.Fatalf("password update mismatch: updated=%v err=%v", updated, err)
	}
	if updated, err := store.UpdatePasswordAndRevokeRefresh(ctx, "missing-user", newHash); err != nil || updated {
		t.Fatalf("password update missing user should return false: updated=%v err=%v", updated, err)
	}
	if err := store.Update(ctx, "missing-user", map[string]any{"preferred_language": "en_US"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user update error = %v; want pgx.ErrNoRows", err)
	}
	if err := store.Update(ctx, "missing-user", map[string]any{"display_name": got.DisplayName}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user with taken display name error = %v; want pgx.ErrNoRows", err)
	}
	deleted, err := store.Delete(ctx, u.ID)
	if err != nil || !deleted {
		t.Fatalf("delete mismatch: deleted=%v err=%v", deleted, err)
	}
	if got, err := store.GetByID(ctx, u.ID); err != nil || got != nil {
		t.Fatalf("deleted user should be gone: user=%#v err=%v", got, err)
	}
	if deleted, err := store.Delete(ctx, u.ID); err != nil || deleted {
		t.Fatalf("delete missing user should return false: deleted=%v err=%v", deleted, err)
	}
}
