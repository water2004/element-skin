package imports_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/imports"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
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
	row, _ := db.Profiles.GetByID(ctx, "ms_uuid_1")
	if row == nil || row.SkinHash == nil || *row.SkinHash != "skin_h" || row.CapeHash == nil || *row.CapeHash != "cape_h" || row.TextureModel != "slim" {
		t.Fatalf("profile not persisted correctly: %#v", row)
	}
}

func TestImportProfileDeduplicatesFullLengthNameWithExactSuffix(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "import-full-name@test.com", "Password123", "ImportFullName", false)
	const fullName = "1234567890ABCDEF"
	testutil.CreateProfile(t, db, user.ID, "full_name_existing", fullName)
	testutil.CreateProfile(t, db, user.ID, "full_name_existing_suffix", "1234567890ABCD_1")

	result, err := (imports.ImportService{DB: db}).ImportProfile(
		ctx,
		user.ID,
		"full_name_imported",
		fullName,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	profile := result["profile"].(map[string]any)
	if profile["name"] != "1234567890ABCD_2" {
		t.Fatalf("imported full-length profile name = %#v; want exact _2 suffix", profile)
	}
	stored, err := db.Profiles.GetByID(ctx, "full_name_imported")
	if err != nil || stored == nil || stored.Name != "1234567890ABCD_2" || len(stored.Name) != 16 {
		t.Fatalf("stored imported profile = %#v, %v; want 16-character suffixed name", stored, err)
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
	row, _ := db.Profiles.GetByID(ctx, "tolerant_uuid")
	if row == nil || row.SkinHash != nil {
		t.Fatalf("unexpected tolerant profile: %#v", row)
	}
}

func TestImportProfileRejectsInvalidNamesWithoutPersisting(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "invalid-import-name@test.com", "Password123", "InvalidImportName", false)
	importer := imports.ImportService{DB: db}

	result, err := importer.ImportProfile(ctx, user.ID, "invalid_import_name_id", "Bad Name!", nil)
	if result != nil || err != (util.HTTPError{Status: 400, Detail: "invalid profile name"}) {
		t.Fatalf("invalid import name result=%#v err=%#v; want nil and exact 400", result, err)
	}
	if stored, err := db.Profiles.GetByID(ctx, "invalid_import_name_id"); err != nil || stored != nil {
		t.Fatalf("invalid import name persisted profile=%#v err=%v", stored, err)
	}

	batch := importer.ImportProfiles(ctx, user.ID, []map[string]string{
		{"profile_id": "invalid_import_batch", "profile_name": "Also-Bad"},
		{"profile_id": "valid_import_batch", "profile_name": "ValidImport"},
	}, func(context.Context, string) ([]imports.TextureAsset, error) {
		return nil, nil
	})
	if batch["success_count"] != 1 || batch["failure_count"] != 1 {
		t.Fatalf("invalid-name batch counts mismatch: %#v", batch)
	}
	items := batch["items"].([]map[string]any)
	failed := batch["failed"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != "valid_import_batch" || items[0]["name"] != "ValidImport" ||
		len(failed) != 1 || failed[0]["profile_id"] != "invalid_import_batch" || failed[0]["detail"] != "invalid profile name" {
		t.Fatalf("invalid-name batch result mismatch: %#v", batch)
	}
	if stored, err := db.Profiles.GetByID(ctx, "invalid_import_batch"); err != nil || stored != nil {
		t.Fatalf("invalid batch import persisted profile=%#v err=%v", stored, err)
	}
}

func TestConcurrentImportsRetryConflictingProfileName(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "concurrent-import-name@test.com", "Password123", "ConcurrentImportName", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_concurrent_import_name() RETURNS trigger AS $$
		BEGIN
			IF NEW.id LIKE 'concurrent_import_name_%' THEN
				PERFORM pg_sleep(0.2);
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_concurrent_import_name
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_concurrent_import_name();
	`); err != nil {
		t.Fatal(err)
	}

	importer := imports.ImportService{DB: db}
	type result struct {
		profile map[string]any
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, id := range []string{"concurrent_import_name_a", "concurrent_import_name_b"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, err := importer.ImportProfile(context.Background(), user.ID, id, "ConcurrentImp", nil)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{profile: res["profile"].(map[string]any)}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var names []string
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent import failed: %v", result.err)
		}
		names = append(names, result.profile["name"].(string))
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "ConcurrentImp" || names[1] != "ConcurrentImp_1" {
		t.Fatalf("concurrent imported names=%#v; want exact deduplicated names", names)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM profiles
		WHERE id IN ('concurrent_import_name_a', 'concurrent_import_name_b')
		  AND user_id=$1
	`, user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("concurrent imports persisted %d profiles; want 2", count)
	}
}

func TestConcurrentImportsReturnExactUUIDConflict(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "concurrent-import-id@test.com", "Password123", "ConcurrentImportID", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_concurrent_import_id() RETURNS trigger AS $$
		BEGIN
			IF NEW.id = 'concurrent_import_same_id' THEN
				PERFORM pg_sleep(0.2);
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_concurrent_import_id
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_concurrent_import_id();
	`); err != nil {
		t.Fatal(err)
	}

	importer := imports.ImportService{DB: db}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := importer.ImportProfile(
				context.Background(),
				user.ID,
				"concurrent_import_same_id",
				"ConcurrentID",
				nil,
			)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case err == (util.HTTPError{Status: 400, Detail: "UUID already exists"}):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent UUID import error: %#v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent UUID imports: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM profiles
		WHERE id='concurrent_import_same_id'
		  AND user_id=$1
		  AND name='ConcurrentID'
	`, user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent UUID import persisted %d exact profiles; want 1", count)
	}
}

func TestImportProfilesBatchKeepsBusinessErrorsAndConvergesInternal(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "batchimport@test.com", "Password123", "BatchImporter", false)
	if err := db.Profiles.Create(ctx, model.Profile{ID: "batch_biz_pid", UserID: user.ID, Name: "Existing", TextureModel: "default"}); err != nil {
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
