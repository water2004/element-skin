package yggdrasil_test

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilProfileJSONExactTexturePayload(t *testing.T) {
	skin := "skin_hash"
	cape := "cape_hash"
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root/"
	cfg.APIURL = "https://api.example/skinapi/"
	ygg, err := yggdrasil.New(nil, cfg, settings.Settings{})
	if err != nil {
		t.Fatal(err)
	}

	signed, err := ygg.ProfileJSON(model.Profile{
		ID: "profile_id", Name: "SlimPlayer", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if signed["id"] != "profile_id" || signed["name"] != "SlimPlayer" {
		t.Fatalf("unexpected profile envelope: %#v", signed)
	}
	props := signed["properties"].([]map[string]any)
	textureProp := props[0]
	if textureProp["name"] != "textures" || textureProp["signature"] == "" {
		t.Fatalf("signed texture property missing name/signature: %#v", textureProp)
	}
	decoded, err := base64.StdEncoding.DecodeString(textureProp["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	textures := payload["textures"].(map[string]any)
	skinPayload := textures["SKIN"].(map[string]any)
	if skinPayload["url"] != "https://api.example/skinapi/static/textures/skin_hash.png" ||
		skinPayload["metadata"].(map[string]any)["model"] != "slim" ||
		textures["CAPE"].(map[string]any)["url"] != "https://api.example/skinapi/static/textures/cape_hash.png" {
		t.Fatalf("unexpected textures payload: %#v", textures)
	}
	verifySignature(t, cfg.PublicKeyPath, textureProp["value"].(string), textureProp["signature"].(string))
	if props[1]["name"] != "uploadableTextures" || props[1]["value"] != "skin,cape" {
		t.Fatalf("missing uploadableTextures property: %#v", props)
	}

	unsigned, err := ygg.ProfileJSON(model.Profile{ID: "p2", Name: "DefaultPlayer", TextureModel: "default", SkinHash: &skin}, false)
	if err != nil {
		t.Fatal(err)
	}
	unsignedProp := unsigned["properties"].([]map[string]any)[0]
	if _, ok := unsignedProp["signature"]; ok {
		t.Fatalf("unsigned profile should not include signature: %#v", unsignedProp)
	}
	decoded, err = base64.StdEncoding.DecodeString(unsignedProp["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	defaultSkin := payload["textures"].(map[string]any)["SKIN"].(map[string]any)
	if _, ok := defaultSkin["metadata"]; ok {
		t.Fatalf("default model should not include metadata: %#v", defaultSkin)
	}
}

func verifySignature(t *testing.T, publicKeyPath, value, signature string) {
	t.Helper()
	pemBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("public key fixture is not PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		t.Fatal("public key fixture is not RSA")
	}
	rawSignature, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha1.Sum([]byte(value))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA1, digest[:], rawSignature); err != nil {
		t.Fatalf("signature should verify against metadata public key: %v", err)
	}
}

func TestYggdrasilLookupNameReturnsExactStatus(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "lookup-service@test.com", "Password123", "LookupService", false)
	testutil.CreateProfile(t, db, user.ID, "lookup_profile_id", "LookupProfile")
	ygg := yggdrasil.Yggdrasil{DB: db}

	hit, status, err := ygg.LookupName(ctx, "LookupProfile")
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || hit["id"] != "lookup_profile_id" || hit["name"] != "LookupProfile" {
		t.Fatalf("unexpected lookup hit status=%d body=%#v", status, hit)
	}
	miss, status, err := ygg.LookupName(ctx, "MissingProfile")
	if err != nil {
		t.Fatal(err)
	}
	if status != 204 || miss != nil {
		t.Fatalf("unexpected lookup miss status=%d body=%#v", status, miss)
	}
}
