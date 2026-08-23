package yggdrasil_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	settingssvc "element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestYggdrasilPublicKeysAndMetadataMergeOwnAndCachedKeysExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	redis := testutil.NewMemoryRedis()
	cfg := testutil.TestConfig()
	settings := settingssvc.Settings{DB: db, Redis: redis}
	ygg, err := yggsvc.New(db, cfg, redis, settings)
	if err != nil {
		t.Fatal(err)
	}
	first := testutil.NewPublicKeyFixture(t)
	second := testutil.NewPublicKeyFixture(t)
	third := testutil.NewPublicKeyFixture(t)
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{
		{Priority: 1, SessionURL: "https://one.example/session", AccountURL: "https://one.example/account", ServicesURL: "https://one.example/services"},
		{Priority: 2, SessionURL: "https://two.example/session", AccountURL: "https://two.example/account", ServicesURL: "https://two.example/services"},
	}); err != nil {
		t.Fatal(err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources := fallbacksvc.PublicKeySources(endpoints)
	if len(sources) != 2 {
		t.Fatalf("source count=%d, want 2: %#v", len(sources), sources)
	}
	if err := redis.SetFallbackPublicKeys(ctx, sources[0].ID, model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: first.DERBase64}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: second.DERBase64}},
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetFallbackPublicKeys(ctx, sources[1].ID, model.YggdrasilPublicKeys{
		ProfilePropertyKeys: []model.YggdrasilPublicKey{
			{PublicKey: first.DERBase64},
			{PublicKey: third.DERBase64},
		},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{
			{PublicKey: second.DERBase64},
			{PublicKey: third.DERBase64},
		},
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	own, err := fallbacksvc.NormalizePEMPublicKeys([]string{ygg.Signer.PublicKeyPEM()})
	if err != nil || len(own) != 1 {
		t.Fatalf("own key normalization=%#v err=%v", own, err)
	}
	want := model.YggdrasilPublicKeys{
		ProfilePropertyKeys: []model.YggdrasilPublicKey{
			own[0],
			{PublicKey: first.DERBase64},
			{PublicKey: third.DERBase64},
		},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{
			own[0],
			{PublicKey: second.DERBase64},
			{PublicKey: third.DERBase64},
		},
	}
	got, err := ygg.PublicKeys(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public keys mismatch:\n got=%#v\nwant=%#v", got, want)
	}

	metadata, err := ygg.Metadata(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	wantPEM := []string{ygg.Signer.PublicKeyPEM(), first.PEM, third.PEM}
	if metadata["signaturePublickey"] != ygg.Signer.PublicKeyPEM() || !reflect.DeepEqual(metadata["signaturePublickeys"], wantPEM) {
		t.Fatalf("metadata signature keys mismatch: %#v", metadata)
	}
}

func TestYggdrasilPublicMetadataRequiresPublicPermissionExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	cfg := testutil.TestConfig()
	ygg, err := yggsvc.New(db, cfg, redis, settingssvc.Settings{DB: db, Redis: redis})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ygg.PublicKeys(t.Context(), permission.Actor{}); !reflect.DeepEqual(got, model.YggdrasilPublicKeys{}) || !exactHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("PublicKeys without permission result=%#v err=%#v", got, err)
	}
	if got, err := ygg.Metadata(t.Context(), permission.Actor{}); got != nil || !exactHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("Metadata without permission result=%#v err=%#v", got, err)
	}
}

func exactHTTPError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Error() == detail
}
