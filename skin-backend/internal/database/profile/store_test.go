package profile_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestStoreCRUDHelpersSearchAndCascade(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := profile.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-profile@test.com", "Password123", "DomainProfile", false)
	skin := "domain_skin"
	p := model.Profile{ID: "domain_profile_a", UserID: user.ID, Name: "DomainProfileA", TextureModel: "slim", SkinHash: &skin}
	if err := store.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if profile.NormalizeModel("wide") != "default" || profile.NormalizeModel("slim") != "slim" {
		t.Fatal("NormalizeModel should whitelist slim only")
	}
	summary := profile.Summary(p)
	if summary["id"] != p.ID || summary["model"] != "slim" || summary["skin_hash"] == nil {
		t.Fatalf("summary mismatch: %#v", summary)
	}
	got, err := store.GetByName(ctx, "DomainProfileA")
	if err != nil || got == nil || got.ID != p.ID {
		t.Fatalf("GetByName mismatch: profile=%#v err=%v", got, err)
	}
	if ok, err := store.VerifyOwnership(ctx, user.ID, p.ID); err != nil || !ok {
		t.Fatalf("ownership mismatch: ok=%v err=%v", ok, err)
	}
	if ok, err := store.UpdateName(ctx, p.ID, "DomainProfileRenamed"); err != nil || !ok {
		t.Fatalf("rename mismatch: ok=%v err=%v", ok, err)
	}
	if err := store.UpdateSkin(ctx, p.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCape(ctx, p.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateModel(ctx, p.ID, "default"); err != nil {
		t.Fatal(err)
	}
	search, err := store.SearchByNames(ctx, []string{"DomainProfileRenamed"}, 5)
	if err != nil || len(search) != 1 || search[0].TextureModel != "default" || search[0].SkinHash != nil {
		t.Fatalf("search mismatch: profiles=%#v err=%v", search, err)
	}
	deleted, err := store.DeleteCascade(ctx, p.ID)
	if err != nil || !deleted {
		t.Fatalf("delete cascade mismatch: deleted=%v err=%v", deleted, err)
	}
	if got, err := store.GetByID(ctx, p.ID); err != nil || got != nil {
		t.Fatalf("delete cascade should remove profile row: profile=%#v err=%v", got, err)
	}
}
