package imports_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/imports"
	"element-skin/backend/internal/testutil"
)

func TestImportProfileSkinCapeAndNameDedup(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "import@test.com", "Password123", "Importer", false)
	testutil.CreateProfile(t, db, user.ID, "other_uuid", "TakenName")
	hashes := []string{"skin_h", "cape_h"}
	importer := imports.ImportService{
		DB:              db,
		DownloadTexture: func(context.Context, string) ([]byte, error) { return []byte("bytes"), nil },
		ProcessTexture: func(_ []byte, _ string) (string, error) {
			h := hashes[0]
			hashes = hashes[1:]
			return h, nil
		},
	}
	res, err := importer.ImportProfile(ctx, user.ID, "ms_uuid_1", "TakenName", []imports.TextureAsset{
		{URL: "http://skin", Kind: "skin", Variant: "slim"},
		{URL: "http://cape", Kind: "cape"},
	})
	if err != nil {
		t.Fatal(err)
	}
	profile := res["profile"].(map[string]any)
	if profile["name"] != "TakenName_1" || profile["model"] != "slim" {
		t.Fatalf("unexpected imported profile: %#v", profile)
	}
	row, _ := db.GetProfileByID(ctx, "ms_uuid_1")
	if row == nil || row.SkinHash == nil || *row.SkinHash != "skin_h" || row.CapeHash == nil || *row.CapeHash != "cape_h" || row.TextureModel != "slim" {
		t.Fatalf("profile not persisted correctly: %#v", row)
	}
}

func TestImportProfileUUIDConflictAndDownloadTolerance(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "conflict@test.com", "Password123", "Importer", false)
	testutil.CreateProfile(t, db, user.ID, "dup_uuid", "AlreadyHere")
	importer := imports.ImportService{DB: db}
	if _, err := importer.ImportProfile(ctx, user.ID, "dup_uuid", "Whatever", nil); err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("expected UUID conflict, got %v", err)
	}

	importer.DownloadTexture = func(context.Context, string) ([]byte, error) { return nil, errors.New("network down") }
	res, err := importer.ImportProfile(ctx, user.ID, "tolerant_uuid", "Tolerant", []imports.TextureAsset{{URL: "http://skin", Kind: "skin"}})
	if err != nil {
		t.Fatal(err)
	}
	profile := res["profile"].(map[string]any)
	if profile["skin_hash"] != (*string)(nil) {
		t.Fatalf("failed skin download should be tolerated: %#v", profile)
	}
	row, _ := db.GetProfileByID(ctx, "tolerant_uuid")
	if row == nil || row.SkinHash != nil {
		t.Fatalf("unexpected tolerant profile: %#v", row)
	}
}

func TestImportProfilesBatchKeepsBusinessErrorsAndConvergesInternal(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "batchimport@test.com", "Password123", "BatchImporter", false)
	if err := db.CreateProfile(ctx, model.Profile{ID: "batch_biz_pid", UserID: user.ID, Name: "Existing", TextureModel: "default"}); err != nil {
		t.Fatal(err)
	}
	importer := imports.ImportService{DB: db}
	result := importer.ImportProfiles(ctx, user.ID, []map[string]string{
		{"profile_id": "batch_ok", "profile_name": "BatchOk"},
		{"profile_id": "", "profile_name": "Broken"},
		{"profile_id": "batch_internal", "profile_name": "Internal"},
		{"profile_id": "batch_biz_pid", "profile_name": "Biz"},
	}, func(_ context.Context, id string) ([]imports.TextureAsset, error) {
		if id == "batch_internal" {
			return nil, errors.New("connect fail http://secret-host/token=zzz")
		}
		return nil, nil
	})
	if result["success_count"] != 1 || result["failure_count"] != 3 {
		t.Fatalf("unexpected batch result: %#v", result)
	}
	failed := result["failed"].([]map[string]any)
	byID := map[string]string{}
	for _, f := range failed {
		byID[f["profile_id"].(string)] = f["detail"].(string)
	}
	if byID["batch_internal"] != "导入失败" {
		t.Fatalf("internal error should be converged: %#v", byID)
	}
	if !strings.Contains(byID["batch_biz_pid"], "UUID") {
		t.Fatalf("business UUID error should pass through: %#v", byID)
	}
}
