package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTextureUserStoreCRUDAndPaginationExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-user-store@test.com", "Password123", "TextureUserStore", false)
	if err := db.AddTextureToLibrary(ctx, user.ID, "texture_user_hash", "skin", "Texture User Note", true, "slim"); err != nil {
		t.Fatal(err)
	}
	info, err := db.GetTextureInfo(ctx, user.ID, "texture_user_hash", "skin")
	if err != nil || info["hash"] != "texture_user_hash" || info["note"] != "Texture User Note" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("texture info mismatch: info=%#v err=%v", info, err)
	}
	if ok, err := db.VerifyTextureOwnership(ctx, user.ID, "texture_user_hash", "skin"); err != nil || !ok {
		t.Fatalf("ownership mismatch: ok=%v err=%v", ok, err)
	}
	list, err := db.ListUserTextures(ctx, user.ID, "skin", 1, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "texture_user_hash" || list["has_next"] != false {
		t.Fatalf("texture list mismatch: %#v", list)
	}
	if err := db.UpdateTextureNote(ctx, user.ID, "texture_user_hash", "skin", "Updated Note"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTextureModel(ctx, user.ID, "texture_user_hash", "skin", "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateTexturePublic(ctx, user.ID, "texture_user_hash", "skin", false); err != nil {
		t.Fatal(err)
	}
	info, err = db.GetTextureInfo(ctx, user.ID, "texture_user_hash", "skin")
	if err != nil || info["note"] != "Updated Note" || info["model"] != "default" || info["is_public"] != 0 {
		t.Fatalf("updated texture info mismatch: info=%#v err=%v", info, err)
	}
	deleted, err := db.DeleteTextureFromLibrary(ctx, user.ID, "texture_user_hash", "skin")
	if err != nil || !deleted {
		t.Fatalf("delete texture mismatch: deleted=%v err=%v", deleted, err)
	}
}
