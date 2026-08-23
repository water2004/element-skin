package homepage_test

import (
	"context"
	"net/http"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/testutil"
)

func TestHomepageServiceClosedDatabaseReturnsDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := homepagesvc.Service{DB: db, Redis: redisstore.NewMemoryStore(), CarouselDir: t.TempDir()}
	readActor := homepageActor("homepage_media.read.any")
	writeActor := homepageActor(
		"homepage_media.create.any",
		"homepage_media.update.any",
		"homepage_media.delete.any",
	)
	db.Close()

	if items, err := svc.List(ctx, readActor); items != nil || !closedPool(err) {
		t.Fatalf("List closed database = items=%#v err=%v; want nil closed pool", items, err)
	}
	if item, err := svc.UploadImage(ctx, writeActor, newMultipartSource("file", "closed.png", tinyPNGBytes(t), nil)); item != (model.HomepageMedia{}) || !closedPool(err) {
		t.Fatalf("UploadImage closed database = item=%#v err=%v; want empty closed pool", item, err)
	}
	if item, err := svc.UploadPanorama(ctx, writeActor, newMultipartSource("file", "closed.zip", validPanoramaZip(t), nil)); item != (model.HomepageMedia{}) || !closedPool(err) {
		t.Fatalf("UploadPanorama closed database = item=%#v err=%v; want empty closed pool", item, err)
	}
	title := "Closed"
	if item, err := svc.Patch(ctx, writeActor, "missing", homepagesvc.PatchInput{Title: &title}); item != (model.HomepageMedia{}) || !homepageHTTPError(err, http.StatusNotFound, "homepage_media.resolve.not_found") {
		t.Fatalf("Patch closed database = item=%#v err=%#v; want exact not found mapping", item, err)
	}
	if err := svc.Reorder(ctx, writeActor, []string{"closed"}); !homepageHTTPError(err, http.StatusNotFound, "homepage_media.resolve.not_found") {
		t.Fatalf("Reorder closed database err=%#v; want exact not found mapping", err)
	}
	if err := svc.Delete(ctx, writeActor, "closed"); !homepageHTTPError(err, http.StatusNotFound, "homepage_media.resolve.not_found") {
		t.Fatalf("Delete closed database err=%#v; want exact not found mapping", err)
	}
}
