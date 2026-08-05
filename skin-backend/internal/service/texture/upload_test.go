package texture_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestUploadServiceUploadToLibraryPersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-service@test.com", "Password123", "TextureUploadService", false)
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}

	res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned", "texture.update_visibility.owned"),
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "",
		Note:        "Service Upload",
		IsPublic:    true,
		Model:       "slim",
	})
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := res["hash"].(string)
	if hash == "" || len(res) != 2 || res["type"] != "skin" {
		t.Fatalf("upload response mismatch: %#v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, hash+".png")); err != nil {
		t.Fatalf("uploaded file missing: %v", err)
	}
	info, err := db.Textures.GetInfo(ctx, user.ID, hash, "skin")
	if err != nil || info == nil || info["note"] != "Service Upload" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("uploaded library row mismatch: info=%#v err=%v", info, err)
	}
}

func TestUploadServiceRejectsPermissionsBeforeSideEffects(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-perm@test.com", "Password123", "TextureUploadPerm", false)
	profile := testutil.CreateProfile(t, db, user.ID, "texture_upload_perm", "TextureUploadPerm")
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}
	data := pngBytes(t, 64, 64, testColor())

	if res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID),
		Data:        data,
		TextureType: "skin",
	}); res != nil || !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("upload without create permission result=%#v err=%#v; want exact 403", res, err)
	}
	if res, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned"),
		Data:        data,
		TextureType: "skin",
	}, profile.ID); res != nil || !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("upload apply without apply permission result=%#v err=%#v; want exact 403", res, err)
	}
	if count, err := db.Textures.CountForUser(ctx, user.ID); err != nil || count != 0 {
		t.Fatalf("permission failures must not insert texture rows: count=%d err=%v", count, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("permission failures must not create texture files: entries=%#v err=%v", entries, err)
	}
}

func TestUploadServiceCleansNewFileWhenDatabaseInsertFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-db-fail@test.com", "Password123", "TextureUploadDBFail", false)
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE user_textures ADD CONSTRAINT reject_service_upload CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}

	res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned"),
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "skin",
	})
	if res != nil || err == nil {
		t.Fatalf("database failure result=%#v err=%#v; want nil and error", res, err)
	}
	if count, countErr := db.Textures.CountForUser(ctx, user.ID); countErr != nil || count != 0 {
		t.Fatalf("failed insert must leave no texture rows: count=%d err=%v", count, countErr)
	}
	if entries, readErr := os.ReadDir(dir); readErr != nil || len(entries) != 0 {
		t.Fatalf("failed insert must delete newly created file: entries=%#v err=%v", entries, readErr)
	}
}

func TestUploadServiceUploadAndApplyUpdatesProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-apply@test.com", "Password123", "TextureUploadApply", false)
	profile := testutil.CreateProfile(t, db, user.ID, "texture_upload_apply", "TextureUploadApply")
	svc := texturesvc.UploadService{DB: db, TexturesDir: t.TempDir()}

	res, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned", "texture.apply.owned"),
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "skin",
		Model:       "slim",
	}, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := res["hash"].(string)
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if hash == "" || len(res) != 2 || res["type"] != "skin" ||
		err != nil || updated == nil || updated.SkinHash == nil ||
		*updated.SkinHash != hash || updated.TextureModel != "slim" {
		t.Fatalf("upload apply mismatch: res=%#v profile=%#v err=%v", res, updated, err)
	}
}

func TestUploadServiceUploadAndApplyCapeAndForeignProfileFailureExactState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "texture-upload-cape-owner@test.com", "Password123", "TextureUploadCapeOwner", false)
	other := testutil.CreateUser(t, db, "texture-upload-cape-other@test.com", "Password123", "TextureUploadCapeOther", false)
	ownProfile := testutil.CreateProfile(t, db, owner.ID, "texture_upload_cape_owner", "TextureUploadCapeOwner")
	foreignProfile := testutil.CreateProfile(t, db, other.ID, "texture_upload_cape_foreign", "TextureUploadCapeForeign")
	svc := texturesvc.UploadService{DB: db, TexturesDir: t.TempDir()}

	capeResult, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(owner.ID, "texture.create.owned", "texture.apply.owned"),
		Data:        pngBytes(t, 64, 32, color.RGBA{R: 90, G: 50, B: 180, A: 255}),
		TextureType: "cape",
	}, ownProfile.ID)
	if err != nil {
		t.Fatal(err)
	}
	capeHash, _ := capeResult["hash"].(string)
	updated, err := db.Profiles.GetByID(ctx, ownProfile.ID)
	if capeHash == "" || len(capeResult) != 2 || capeResult["type"] != "cape" ||
		err != nil || updated == nil || updated.CapeHash == nil || *updated.CapeHash != capeHash ||
		updated.SkinHash != nil {
		t.Fatalf("cape upload apply mismatch: result=%#v profile=%#v err=%v", capeResult, updated, err)
	}

	foreignResult, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(owner.ID, "texture.create.owned", "texture.apply.owned"),
		Data:        pngBytes(t, 64, 64, color.RGBA{R: 20, G: 200, B: 120, A: 255}),
		TextureType: "skin",
	}, foreignProfile.ID)
	if foreignResult != nil || !httpErrorIs(err, http.StatusForbidden, "Profile not yours") {
		t.Fatalf("foreign profile upload apply = result=%#v err=%#v; want exact 403", foreignResult, err)
	}
	foreignAfter, err := db.Profiles.GetByID(ctx, foreignProfile.ID)
	if err != nil || foreignAfter == nil || foreignAfter.SkinHash != nil || foreignAfter.CapeHash != nil {
		t.Fatalf("foreign profile failure must not mutate profile: profile=%#v err=%v", foreignAfter, err)
	}
	if count, err := db.Textures.CountForUser(ctx, owner.ID); err != nil || count != 2 {
		t.Fatalf("upload apply failure should keep uploaded library row for retry: count=%d err=%v", count, err)
	}
}

func TestUploadServiceBoundProfileUploadDoesNotRequireCreatePermission(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-bound@test.com", "Password123", "TextureUploadBound", false)
	profile := testutil.CreateProfile(t, db, user.ID, "texture_upload_bound", "TextureUploadBound")
	svc := texturesvc.UploadService{DB: db, TexturesDir: t.TempDir()}
	actor := textureActor(user.ID, "texture.apply.bound_profile")
	actor.BoundProfileID = profile.ID

	res, err := svc.UploadAndApplyBoundProfile(ctx, texturesvc.UploadInput{
		Actor:       actor,
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "skin",
		Model:       "slim",
	}, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := res["hash"].(string)
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if hash == "" || len(res) != 2 || res["type"] != "skin" ||
		err != nil || updated == nil || updated.SkinHash == nil ||
		*updated.SkinHash != hash || updated.TextureModel != "slim" {
		t.Fatalf("bound upload apply mismatch: res=%#v profile=%#v err=%v", res, updated, err)
	}
	if info, err := db.Textures.GetInfo(ctx, user.ID, hash, "skin"); err != nil ||
		info == nil || info["is_public"] != 0 || info["model"] != "slim" {
		t.Fatalf("bound upload should persist private library row: info=%#v err=%v", info, err)
	}
}

func TestUploadServiceRejectsPublicUploadAndBoundProfileInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-bound-invalid@test.com", "Password123", "TextureUploadBoundInvalid", false)
	profile := testutil.CreateProfile(t, db, user.ID, "texture_upload_bound_invalid", "TextureUploadBoundInvalid")
	otherProfile := testutil.CreateProfile(t, db, user.ID, "texture_upload_bound_other", "TextureUploadBoundOther")
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}
	data := pngBytes(t, 64, 64, testColor())

	if res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned"),
		Data:        data,
		TextureType: "skin",
		IsPublic:    true,
	}); res != nil || !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("public upload without visibility permission result=%#v err=%#v; want exact 403", res, err)
	}
	if res, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned", "texture.apply.owned"),
		Data:        data,
		TextureType: "",
	}, profile.ID); res != nil || !httpErrorIs(err, http.StatusBadRequest, "uuid and texture_type are required") {
		t.Fatalf("upload apply missing type result=%#v err=%#v; want exact 400", res, err)
	}
	if res, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned", "texture.apply.owned"),
		Data:        data,
		TextureType: "elytra",
	}, profile.ID); res != nil || !httpErrorIs(err, http.StatusBadRequest, "Invalid texture_type") {
		t.Fatalf("upload apply invalid type result=%#v err=%#v; want exact 400", res, err)
	}

	boundActor := textureActor(user.ID, "texture.apply.bound_profile")
	boundActor.BoundProfileID = profile.ID
	if res, err := svc.UploadAndApplyBoundProfile(ctx, texturesvc.UploadInput{
		Actor:       boundActor,
		Data:        data,
		TextureType: "skin",
	}, otherProfile.ID); res != nil || !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("bound upload wrong profile result=%#v err=%#v; want exact 403", res, err)
	}
	if count, err := db.Textures.CountForUser(ctx, user.ID); err != nil || count != 0 {
		t.Fatalf("rejected upload inputs must not insert texture rows: count=%d err=%v", count, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("rejected upload inputs must not create files: entries=%#v err=%v", entries, err)
	}
}

func textureActor(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   permissiondb.SubjectIDForUser(userID),
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}

func testColor() color.RGBA {
	return color.RGBA{R: 80, G: 120, B: 200, A: 255}
}

func pngBytes(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func httpErrorIs(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}
