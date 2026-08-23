package homepage_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func newMultipartSource(fieldName, filename string, content []byte, fields map[string]string) homepagesvc.UploadInput {
	if fieldName != "file" {
		filename = ""
		content = nil
	}
	return homepagesvc.UploadInput{Filename: filename, Data: content, Fields: cloneFields(fields)}
}

func newFieldsOnlyMultipartSource(fields map[string]string) homepagesvc.UploadInput {
	return homepagesvc.UploadInput{Fields: cloneFields(fields)}
}

func cloneFields(fields map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range fields {
		out[key] = value
	}
	return out
}

func homepageActor(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "homepage-test",
		UserID:      "homepage-test-user",
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func homepageHTTPError(err error, status int, detail string) bool {
	httpErr, ok := err.(util.HTTPError)
	return ok && httpErr.Status == status && httpErr.Error() == detail
}

func closedPool(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func tinyWebPBytes(t *testing.T) []byte {
	t.Helper()
	encoded, err := os.ReadFile("testdata/tiny.webp.base64")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func validPanoramaZip(t *testing.T) []byte {
	files := map[string][]byte{}
	for i := 0; i < 6; i++ {
		files["panorama_"+string(rune('0'+i))+".png"] = tinyPNGBytes(t)
	}
	return zipWithFiles(t, files)
}

func zipWithFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func intPtr(v int) *int {
	return &v
}

func databaseModelHomepageMediaForTest(id string) []model.HomepageMedia {
	return []model.HomepageMedia{{
		ID: id, Type: "image", Title: id, StoragePath: id + ".png",
		OverlayOpacityLight: 0.45, OverlayOpacityDark: 0.45, YawSpeedDPS: 4,
		SortOrder: 0, Enabled: true, DurationMS: 6000, CreatedAt: 1, UpdatedAt: 1,
	}}
}
