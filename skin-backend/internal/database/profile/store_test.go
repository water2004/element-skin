package profile_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	item := profile.ModelKey(map[string]any{"texture_model": "slim"})
	if item["model"] != "slim" {
		t.Fatalf("ModelKey should expose texture_model as model: %#v", item)
	}
	summary := profile.Summary(p)
	if summary["id"] != p.ID || summary["model"] != "slim" || summary["skin_hash"] == nil {
		t.Fatalf("summary mismatch: %#v", summary)
	}
	if err := store.Create(ctx, model.Profile{ID: "domain_profile_dup", UserID: user.ID, Name: p.Name, TextureModel: "default"}); !profile.IsNameConflict(err) {
		t.Fatalf("duplicate profile name should be detected as conflict, got %v", err)
	}
	if profile.IsNameConflict(&pgconn.PgError{Code: "23505", ConstraintName: "profiles_pkey"}) {
		t.Fatal("profile primary-key conflict must not be reported as a name conflict")
	}
	if profile.IsNameConflict(&pgconn.PgError{Code: "23514", ConstraintName: "profiles_name_key"}) {
		t.Fatal("non-unique profile error must not be reported as a name conflict")
	}
	if !profile.IsIDConflict(&pgconn.PgError{Code: "23505", ConstraintName: "profiles_pkey"}) {
		t.Fatal("profile primary-key conflict should be detected as an ID conflict")
	}
	if profile.IsIDConflict(&pgconn.PgError{Code: "23505", ConstraintName: "profiles_name_key"}) {
		t.Fatal("profile name conflict must not be reported as an ID conflict")
	}
	if profile.IsIDConflict(&pgconn.PgError{Code: "23514", ConstraintName: "profiles_pkey"}) {
		t.Fatal("non-unique profile error must not be reported as an ID conflict")
	}
	got, err := store.GetByName(ctx, "DomainProfileA")
	if err != nil || got == nil || got.ID != p.ID {
		t.Fatalf("GetByName mismatch: profile=%#v err=%v", got, err)
	}
	userProfiles, err := store.GetByUser(ctx, user.ID, 5)
	if err != nil || len(userProfiles) != 1 || userProfiles[0].ID != p.ID {
		t.Fatalf("GetByUser mismatch: profiles=%#v err=%v", userProfiles, err)
	}
	if count, err := store.CountByUser(ctx, user.ID); err != nil || count != 1 {
		t.Fatalf("CountByUser mismatch: count=%d err=%v", count, err)
	}
	if ok, err := store.VerifyOwnership(ctx, user.ID, p.ID); err != nil || !ok {
		t.Fatalf("ownership mismatch: ok=%v err=%v", ok, err)
	}
	if ok, err := store.VerifyOwnership(ctx, user.ID, "missing_profile"); err != nil || ok {
		t.Fatalf("missing ownership should be false: ok=%v err=%v", ok, err)
	}
	if ok, err := store.UpdateName(ctx, p.ID, "DomainProfileRenamed"); err != nil || !ok {
		t.Fatalf("rename mismatch: ok=%v err=%v", ok, err)
	}
	if ok, err := store.UpdateName(ctx, "missing_profile", "Nope"); err != nil || ok {
		t.Fatalf("missing rename should be false: ok=%v err=%v", ok, err)
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
	for name, err := range map[string]error{
		"skin":           store.UpdateSkin(ctx, "missing_profile", nil),
		"skin and model": store.UpdateSkinAndModel(ctx, "missing_profile", nil, "default"),
		"cape":           store.UpdateCape(ctx, "missing_profile", nil),
		"model":          store.UpdateModel(ctx, "missing_profile", "default"),
	} {
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("missing profile %s update error = %v; want pgx.ErrNoRows", name, err)
		}
	}
	search, err := store.SearchByNames(ctx, []string{"DomainProfileRenamed"}, 5)
	if err != nil || len(search) != 1 || search[0].TextureModel != "default" || search[0].SkinHash != nil {
		t.Fatalf("search mismatch: profiles=%#v err=%v", search, err)
	}
	deleted, err := store.DeleteCascade(ctx, p.ID)
	if err != nil || !deleted {
		t.Fatalf("delete cascade mismatch: deleted=%v err=%v", deleted, err)
	}
	if deleted, err := store.DeleteCascade(ctx, p.ID); err != nil || deleted {
		t.Fatalf("delete cascade missing profile should be false: deleted=%v err=%v", deleted, err)
	}
	if got, err := store.GetByID(ctx, p.ID); err != nil || got != nil {
		t.Fatalf("delete cascade should remove profile row: profile=%#v err=%v", got, err)
	}
}

func TestUpdateSkinAndModelIsAtomic(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := profile.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "profile-skin-model@test.com", "Password123", "ProfileSkinModel", false)
	oldHash := "old_profile_skin"
	item := model.Profile{
		ID:           "profile_skin_model",
		UserID:       user.ID,
		Name:         "ProfileSkinModel",
		TextureModel: "default",
		SkinHash:     &oldHash,
	}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	newHash := "new_profile_skin"
	if err := store.UpdateSkinAndModel(ctx, item.ID, &newHash, "slim"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByID(ctx, item.ID)
	if err != nil || got == nil || got.SkinHash == nil || *got.SkinHash != newHash || got.TextureModel != "slim" {
		t.Fatalf("successful update = %#v, %v; want hash=%q model=slim", got, err, newHash)
	}
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE profiles
		ADD CONSTRAINT profile_default_model_only CHECK (texture_model = 'slim')
	`); err != nil {
		t.Fatal(err)
	}
	rejectedHash := "rejected_profile_skin"
	err = store.UpdateSkinAndModel(ctx, item.ID, &rejectedHash, "default")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("failed update error = %#v; want PostgreSQL 23514", err)
	}
	got, err = store.GetByID(ctx, item.ID)
	if err != nil || got == nil || got.SkinHash == nil || *got.SkinHash != newHash || got.TextureModel != "slim" {
		t.Fatalf("failed update changed profile: profile=%#v err=%v", got, err)
	}
}
