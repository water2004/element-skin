package invite_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/testutil"
)

func TestStoreCreateGetListDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := invite.Store{Pool: db.Pool}
	if err := store.Create(ctx, "sub_invite", 3, "Sub Invite"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "sub_invite")
	if err != nil || got == nil || got.Code != "sub_invite" || got.TotalUses == nil || *got.TotalUses != 3 || got.Note != "Sub Invite" {
		t.Fatalf("invite get mismatch: invite=%#v err=%v", got, err)
	}
	list, err := store.List(ctx, 1, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["code"] != "sub_invite" || list["has_next"] != false {
		t.Fatalf("invite list mismatch: %#v", list)
	}
	if err := store.Delete(ctx, "sub_invite"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(ctx, "sub_invite"); err != nil || got != nil {
		t.Fatalf("invite should be deleted: invite=%#v err=%v", got, err)
	}
}
