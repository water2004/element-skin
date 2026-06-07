package database

import (
	"testing"

	"element-skin/backend/internal/model"
)

func TestNormalizeTextureModel(t *testing.T) {
	if NormalizeProfileModel("slim") != "slim" {
		t.Fatal("slim should pass through")
	}
	for _, input := range []string{"default", "classic", "", "SLIM"} {
		if got := NormalizeProfileModel(input); got != "default" {
			t.Fatalf("NormalizeProfileModel(%q)=%q", input, got)
		}
	}
}

func TestProfileSummaryMapsFields(t *testing.T) {
	skin := "skinhash"
	cape := "capehash"
	got := ProfileSummary(model.Profile{ID: "pid", UserID: "uid", Name: "Steve", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape})
	if got["id"] != "pid" || got["name"] != "Steve" || got["model"] != "slim" || got["skin_hash"] != &skin || got["cape_hash"] != &cape {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if _, ok := got["user_id"]; ok {
		t.Fatal("profile summary should not expose user_id")
	}
}
