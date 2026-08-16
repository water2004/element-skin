package officialprofile_test

import (
	"context"
	"errors"
	"testing"

	officialstore "element-skin/backend/internal/database/officialprofile"
	"element-skin/backend/internal/model"
	identitysvc "element-skin/backend/internal/service/identity"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOfficialProfileStoreLifecycleAndSyncPersistExactStructuredState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "official-store@test.com", "Password123", "OfficialStore", false)
	provider := model.IdentityProvider{
		ID: "official-store-provider", Name: "Microsoft", IssuerURL: "https://login.example",
		AuthorizationEndpoint: "https://login.example/authorize", TokenEndpoint: "https://login.example/token",
		JWKSURI: "https://login.example/jwks", ClientID: "client", Scopes: []string{"openid"},
		Adapter: identitysvc.AdapterMicrosoft, Enabled: true, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	identity := model.ExternalIdentity{ID: "official-store-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", Label: "Main Microsoft", CreatedAt: 2, UpdatedAt: 2}
	if err := db.Identities.CreateIdentity(ctx, identity, model.ExternalIdentityCredential{IdentityID: identity.ID, UpdatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "0123456789abcdef0123456789abcdef", "LocalRole")
	binding := model.OfficialProfileBinding{
		ID: "official-store-binding", IdentityID: identity.ID, ProfileID: profile.ID,
		RemoteUUID: "0123456789abcdef0123456789abcdef", RemoteName: "RemoteOld",
		RemoteSkinURL: "https://textures.example/old.png", RemoteSkinModel: "default",
		CreatedAt: 10, UpdatedAt: 10,
	}
	if err := db.OfficialProfiles.Create(ctx, officialstore.CreateInput{
		Binding: binding, UserID: user.ID, ProfileNames: []string{"RemoteOld"}, ProfileModel: "default",
	}); err != nil {
		t.Fatal(err)
	}

	items, err := db.OfficialProfiles.ListByUser(ctx, user.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("list bindings=%#v err=%v", items, err)
	}
	if got := items[0]; got.Binding != binding || got.Profile != profile || got.IdentityLabel != "Main Microsoft" || got.ProviderID != provider.ID || got.ProviderName != "Microsoft" || got.ProviderAdapter != identitysvc.AdapterMicrosoft {
		t.Fatalf("binding view mismatch: %#v", got)
	}

	skinHash := "official_store_skin"
	capeHash := "official_store_cape"
	updated, err := db.OfficialProfiles.Sync(ctx, officialstore.SyncInput{
		ID: binding.ID, UserID: user.ID, RemoteName: "RemoteNew",
		RemoteSkinURL: "https://textures.example/skin.png", RemoteCapeURL: "https://textures.example/cape.png",
		RemoteSkinModel: "slim", SkinHash: &skinHash, CapeHash: &capeHash, SyncedAt: 99,
	})
	if err != nil || !updated {
		t.Fatalf("sync updated=%v err=%v", updated, err)
	}
	view, err := db.OfficialProfiles.GetByIDAndUser(ctx, binding.ID, user.ID)
	if err != nil || view == nil || view.Binding.RemoteName != "RemoteNew" || view.Binding.RemoteSkinURL != "https://textures.example/skin.png" || view.Binding.RemoteCapeURL != "https://textures.example/cape.png" || view.Binding.RemoteSkinModel != "slim" || view.Binding.UpdatedAt != 99 || view.Binding.LastSyncedAt == nil || *view.Binding.LastSyncedAt != 99 {
		t.Fatalf("synced binding mismatch: view=%#v err=%v", view, err)
	}
	if view.Profile.Name != "RemoteNew" || view.Profile.TextureModel != "slim" || view.Profile.SkinHash == nil || *view.Profile.SkinHash != skinHash || view.Profile.CapeHash == nil || *view.Profile.CapeHash != capeHash {
		t.Fatalf("synced profile mismatch: %#v", view.Profile)
	}
	for _, texture := range []struct{ hash, kind, model string }{{skinHash, "skin", "slim"}, {capeHash, "cape", "default"}} {
		info, err := db.Textures.GetInfo(ctx, user.ID, texture.hash, texture.kind)
		if err != nil || info == nil || info["model"] != texture.model || info["is_public"] != 0 {
			t.Fatalf("synced texture %#v info=%#v err=%v", texture, info, err)
		}
	}

	deleted, err := db.OfficialProfiles.DeleteByIDAndUser(ctx, binding.ID, user.ID)
	if err != nil || !deleted {
		t.Fatalf("delete binding deleted=%v err=%v", deleted, err)
	}
	if stored, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || stored == nil || stored.Name != "RemoteNew" {
		t.Fatalf("deleting binding must preserve profile: profile=%#v err=%v", stored, err)
	}
}

func TestOfficialProfileStoreSyncSkipsConflictingNameAndPersistsOtherFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "official-store-rollback@test.com", "Password123", "OfficialRollback", false)
	provider := model.IdentityProvider{ID: "rollback-provider", Name: "Microsoft", IssuerURL: "https://login.example", AuthorizationEndpoint: "https://login.example/a", TokenEndpoint: "https://login.example/t", JWKSURI: "https://login.example/j", ClientID: "client", Adapter: identitysvc.AdapterMicrosoft, Enabled: true, CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	identity := model.ExternalIdentity{ID: "rollback-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateIdentity(ctx, identity, model.ExternalIdentityCredential{IdentityID: identity.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "0123456789abcdef0123456789abcdef", "BeforeSync")
	_ = testutil.CreateProfile(t, db, user.ID, "rollback-conflict", "TakenName")
	binding := model.OfficialProfileBinding{ID: "rollback-binding", IdentityID: identity.ID, ProfileID: profile.ID, RemoteUUID: "0123456789abcdef0123456789abcdef", RemoteName: "BeforeSync", RemoteSkinModel: "default", CreatedAt: 1, UpdatedAt: 1}
	if err := db.OfficialProfiles.Create(ctx, officialstore.CreateInput{
		Binding: binding, UserID: user.ID, ProfileNames: []string{"BeforeSync"}, ProfileModel: "default",
	}); err != nil {
		t.Fatal(err)
	}
	hash := "conflicting_name_synced_skin"
	updated, err := db.OfficialProfiles.Sync(ctx, officialstore.SyncInput{ID: binding.ID, UserID: user.ID, RemoteName: "TakenName", RemoteSkinModel: "slim", SkinHash: &hash, SyncedAt: 50})
	if err != nil || !updated {
		t.Fatalf("conflicting-name sync updated=%v err=%v; want true and nil", updated, err)
	}
	view, getErr := db.OfficialProfiles.GetByIDAndUser(ctx, binding.ID, user.ID)
	if getErr != nil || view == nil || view.Binding.RemoteName != "TakenName" || view.Binding.LastSyncedAt == nil || *view.Binding.LastSyncedAt != 50 || view.Profile.Name != "BeforeSync" || view.Profile.TextureModel != "slim" || view.Profile.SkinHash == nil || *view.Profile.SkinHash != hash {
		t.Fatalf("conflicting-name sync state mismatch: view=%#v err=%v", view, getErr)
	}
	if info, getErr := db.Textures.GetInfo(ctx, user.ID, hash, "skin"); getErr != nil || info == nil || info["model"] != "slim" || info["is_public"] != 0 {
		t.Fatalf("conflicting-name sync texture mismatch: info=%#v err=%v", info, getErr)
	}
}

func TestOfficialProfileStoreCreateRollsBackProfileAndTexturesWhenTextureInsertFailsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "official-store-texture-failure@test.com", "Password123", "OfficialTextureFailure", false)
	provider := model.IdentityProvider{ID: "texture-failure-provider", Name: "Microsoft", IssuerURL: "https://login.example", AuthorizationEndpoint: "https://login.example/a", TokenEndpoint: "https://login.example/t", JWKSURI: "https://login.example/j", ClientID: "client", Adapter: identitysvc.AdapterMicrosoft, Enabled: true, CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	identity := model.ExternalIdentity{ID: "texture-failure-identity", UserID: user.ID, ProviderID: provider.ID, Subject: "subject", CreatedAt: 1, UpdatedAt: 1}
	if err := db.Identities.CreateIdentity(ctx, identity, model.ExternalIdentityCredential{IdentityID: identity.ID, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	profile := testutil.CreateProfile(t, db, user.ID, "texture_failure_profile", "TextureFailure")
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_official_user_texture() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'reject official texture';
		END
		$$;
		CREATE TRIGGER reject_official_user_texture
		BEFORE INSERT ON user_textures
		FOR EACH ROW EXECUTE FUNCTION reject_official_user_texture();
	`); err != nil {
		t.Fatal(err)
	}
	skinHash := "texture-failure-hash"
	binding := model.OfficialProfileBinding{
		ID: "texture-failure-binding", IdentityID: identity.ID, ProfileID: profile.ID,
		RemoteUUID: profile.ID, RemoteName: "Remote", CreatedAt: 2, UpdatedAt: 2,
	}
	err := db.OfficialProfiles.Create(ctx, officialstore.CreateInput{
		Binding: binding, UserID: user.ID, ProfileNames: []string{"Remote"}, ProfileModel: "slim", SkinHash: &skinHash,
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
		t.Fatalf("texture-failed create error=%#v", err)
	}
	stored, getErr := db.Profiles.GetByID(ctx, profile.ID)
	if getErr != nil || stored == nil || stored.Name != profile.Name || stored.TextureModel != profile.TextureModel || stored.SkinHash != nil || stored.CapeHash != nil {
		t.Fatalf("texture-failed create profile=%#v err=%v", stored, getErr)
	}
	var bindingCount, userTextureCount, libraryCount int
	if getErr := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM official_profile_bindings WHERE id=$1`, binding.ID).Scan(&bindingCount); getErr != nil {
		t.Fatal(getErr)
	}
	if getErr := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_textures WHERE hash=$1`, skinHash).Scan(&userTextureCount); getErr != nil {
		t.Fatal(getErr)
	}
	if getErr := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM skin_library WHERE skin_hash=$1`, skinHash).Scan(&libraryCount); getErr != nil {
		t.Fatal(getErr)
	}
	if bindingCount != 0 || userTextureCount != 0 || libraryCount != 0 {
		t.Fatalf("texture-failed create state bindings=%d user_textures=%d library=%d", bindingCount, userTextureCount, libraryCount)
	}
}
